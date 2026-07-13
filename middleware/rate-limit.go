package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/logger"
)

var inMemoryRateLimiter common.InMemoryRateLimiter

var rateLimitScript = redis.NewScript(`
local key = KEYS[1]
local maxNum = tonumber(ARGV[1])
local duration = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local expire = tonumber(ARGV[4])

local len = redis.call('LLEN', key)

if len < maxNum then
    redis.call('LPUSH', key, now)
    redis.call('EXPIRE', key, expire)
    return 1
end

local old = redis.call('LINDEX', key, -1)
if old == false then
    redis.call('LPUSH', key, now)
    redis.call('LTRIM', key, 0, maxNum - 1)
    redis.call('EXPIRE', key, expire)
    return 1
end

local oldNum = tonumber(old)
if now - oldNum < duration then
    redis.call('EXPIRE', key, expire)
    return 0
end

redis.call('LPUSH', key, now)
redis.call('LTRIM', key, 0, maxNum - 1)
redis.call('EXPIRE', key, expire)
return 1
`)

func redisRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	ctx := context.Background()
	rdb := common.RDB
	key := "rateLimit:" + mark + c.ClientIP()

	result, err := rateLimitScript.Run(ctx, rdb, []string{key}, maxRequestNum, duration, time.Now().Unix(), int64(config.RateLimitKeyExpirationDuration.Seconds())).Int64()
	if err != nil {
		logger.SysError("redis rate limit script failed: " + err.Error())
		c.Status(http.StatusInternalServerError)
		c.Abort()
		return
	}

	if result != 1 {
		c.Status(http.StatusTooManyRequests)
		c.Abort()
		return
	}
}

func memoryRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	key := mark + c.ClientIP()
	if !inMemoryRateLimiter.Request(key, maxRequestNum, duration) {
		c.Status(http.StatusTooManyRequests)
		c.Abort()
		return
	}
}

func rateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	if maxRequestNum == 0 || config.DebugEnabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	if common.RedisEnabled {
		return func(c *gin.Context) {
			redisRateLimiter(c, maxRequestNum, duration, mark)
		}
	} else {
		// It's safe to call multi times.
		inMemoryRateLimiter.Init(config.RateLimitKeyExpirationDuration)
		return func(c *gin.Context) {
			memoryRateLimiter(c, maxRequestNum, duration, mark)
		}
	}
}

func GlobalWebRateLimit() func(c *gin.Context) {
	limiter := rateLimitFactory(config.GlobalWebRateLimitNum, config.GlobalWebRateLimitDuration, "GW")
	return func(c *gin.Context) {
		if isStaticAsset(c.Request.URL.Path) {
			c.Next()
			return
		}
		limiter(c)
	}
}

func GlobalAPIRateLimit() func(c *gin.Context) {
	return rateLimitFactory(config.GlobalApiRateLimitNum, config.GlobalApiRateLimitDuration, "GA")
}

func CriticalRateLimit() func(c *gin.Context) {
	return rateLimitFactory(config.CriticalRateLimitNum, config.CriticalRateLimitDuration, "CT")
}

func DownloadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(config.DownloadRateLimitNum, config.DownloadRateLimitDuration, "DW")
}

func UploadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(config.UploadRateLimitNum, config.UploadRateLimitDuration, "UP")
}
