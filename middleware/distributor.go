package middleware

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/fallback"
	"github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay/channeltype"
)

type ModelRequest struct {
	Model string `json:"model" form:"model"`
}

func Distribute() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userId := c.GetInt(ctxkey.Id)
		userGroup, _ := model.CacheGetUserGroup(userId)
		c.Set(ctxkey.Group, userGroup)
		var requestModel string
		var channel *model.Channel
		channelId, ok := c.Get(ctxkey.SpecificChannelId)
		if ok {
			id, err := strconv.Atoi(channelId.(string))
			if err != nil {
				abortWithMessage(c, http.StatusBadRequest, "无效的渠道 Id")
				return
			}
			channel, err = model.GetChannelById(id, true)
			if err != nil {
				abortWithMessage(c, http.StatusBadRequest, "无效的渠道 Id")
				return
			}
			if channel.Status != model.ChannelStatusEnabled {
				abortWithMessage(c, http.StatusForbidden, "该渠道已被禁用")
				return
			}
		} else {
			requestModel = c.GetString(ctxkey.RequestModel)

			// Check if request model is a virtual model
			if common.IsFallbackEnabled && fallback.IsVirtualModel(requestModel) {
				// Use fallback mode
				dep, selectedChannel, err := getFirstUsableFallbackDeployment(requestModel)
				if err != nil {
					abortWithMessage(c, http.StatusServiceUnavailable, fmt.Sprintf("No available deployment for virtual model: %s", requestModel))
					return
				}
				channel = selectedChannel

				if channel.Status != model.ChannelStatusEnabled {
					abortWithMessage(c, http.StatusForbidden, "该渠道已被禁用")
					return
				}

				// Set fallback context keys
				c.Set(ctxkey.FallbackEnabled, true)
				c.Set(ctxkey.FallbackVirtualModel, requestModel)
				c.Set(ctxkey.FallbackDeploymentID, dep.ID)
				c.Set(ctxkey.FallbackRealModel, dep.RealModel)
				c.Set(ctxkey.FallbackChannelID, dep.ChannelID)

				// Log for debugging
				logger.SysLog(fmt.Sprintf("[fallback] virtual model %s matched deployment %s channel %d real model %s",
					requestModel, dep.ID, dep.ChannelID, dep.RealModel))
			} else {
				// Use normal mode
				var err error
				channel, err = model.CacheGetRandomSatisfiedChannel(userGroup, requestModel, false)
				if err != nil {
					message := fmt.Sprintf("当前分组 %s 下对于模型 %s 无可用渠道", userGroup, requestModel)
					if channel != nil {
						logger.SysError(fmt.Sprintf("渠道不存在：%d", channel.Id))
						message = "数据库一致性已被破坏，请联系管理员"
					}
					abortWithMessage(c, http.StatusServiceUnavailable, message)
					return
				}
			}
		}
		logger.Debugf(ctx, "user id %d, user group: %s, request model: %s, using channel #%d", userId, userGroup, requestModel, channel.Id)
		SetupContextForSelectedChannel(c, channel, requestModel)
		c.Next()
	}
}

func getFirstUsableFallbackDeployment(requestModel string) (*fallback.DeploymentConfig, *model.Channel, error) {
	deployments, err := fallback.GetDeploymentsForVirtualModel(requestModel)
	if err != nil {
		return nil, nil, err
	}

	for _, dep := range deployments {
		channel, err := model.GetChannelById(dep.ChannelID, true)
		if err != nil {
			logger.SysLog(fmt.Sprintf("[fallback] skip bootstrap deployment %s: channel %d not found", dep.ID, dep.ChannelID))
			continue
		}
		if channel.Status != model.ChannelStatusEnabled {
			logger.SysLog(fmt.Sprintf("[fallback] skip bootstrap deployment %s: channel %d disabled", dep.ID, dep.ChannelID))
			continue
		}
		selectedDep := dep
		return &selectedDep, channel, nil
	}

	return nil, nil, fmt.Errorf("no enabled channel found for virtual model %s", requestModel)
}

func SetupContextForSelectedChannel(c *gin.Context, channel *model.Channel, modelName string) {
	c.Set(ctxkey.Channel, channel.Type)
	c.Set(ctxkey.ChannelId, channel.Id)
	c.Set(ctxkey.ChannelName, channel.Name)
	if channel.SystemPrompt != nil && *channel.SystemPrompt != "" {
		c.Set(ctxkey.SystemPrompt, *channel.SystemPrompt)
	}
	c.Set(ctxkey.ModelMapping, channel.GetModelMapping())
	c.Set(ctxkey.OriginalModel, modelName) // for retry
	c.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", channel.Key))
	// Propagate trace ID to upstream
	if requestID := c.GetString(helper.RequestIdKey); requestID != "" {
		c.Request.Header.Set(helper.RequestIdKey, requestID)
	}
	// Propagate idempotency key to upstream — forward for every attempt
	if idempotencyKey := c.Request.Header.Get("Idempotency-Key"); idempotencyKey != "" {
		c.Request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	if idempotencyKey := c.Request.Header.Get("X-Idempotency-Key"); idempotencyKey != "" {
		c.Request.Header.Set("X-Idempotency-Key", idempotencyKey)
	}
	c.Set(ctxkey.BaseURL, channel.GetBaseURL())
	cfg, _ := channel.LoadConfig()
	// this is for backward compatibility
	if channel.Other != nil {
		switch channel.Type {
		case channeltype.Azure:
			if cfg.APIVersion == "" {
				cfg.APIVersion = *channel.Other
			}
		case channeltype.Xunfei:
			if cfg.APIVersion == "" {
				cfg.APIVersion = *channel.Other
			}
		case channeltype.Gemini:
			if cfg.APIVersion == "" {
				cfg.APIVersion = *channel.Other
			}
		case channeltype.AIProxyLibrary:
			if cfg.LibraryID == "" {
				cfg.LibraryID = *channel.Other
			}
		case channeltype.Ali:
			if cfg.Plugin == "" {
				cfg.Plugin = *channel.Other
			}
		}
	}
	c.Set(ctxkey.Config, cfg)
}
