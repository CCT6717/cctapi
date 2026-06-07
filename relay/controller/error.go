package controller

import (
	"encoding/json"
	"fmt"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/relay/model"
	"io"
	"net/http"
	"strconv"
	"time"
)

type GeneralErrorResponse struct {
	Error    model.Error `json:"error"`
	Message  string      `json:"message"`
	Msg      string      `json:"msg"`
	Err      string      `json:"err"`
	ErrorMsg string      `json:"error_msg"`
	Header   struct {
		Message string `json:"message"`
	} `json:"header"`
	Response struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"response"`
}

func (e GeneralErrorResponse) ToMessage() string {
	if e.Error.Message != "" {
		return e.Error.Message
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Msg != "" {
		return e.Msg
	}
	if e.Err != "" {
		return e.Err
	}
	if e.ErrorMsg != "" {
		return e.ErrorMsg
	}
	if e.Header.Message != "" {
		return e.Header.Message
	}
	if e.Response.Error.Message != "" {
		return e.Response.Error.Message
	}
	return ""
}

func RelayErrorHandler(resp *http.Response) (ErrorWithStatusCode *model.ErrorWithStatusCode) {
	if resp == nil {
		return &model.ErrorWithStatusCode{
			StatusCode: 500,
			Error: model.Error{
				Message: "resp is nil",
				Type:    "upstream_error",
				Code:    "bad_response",
			},
		}
	}
	ErrorWithStatusCode = &model.ErrorWithStatusCode{
		StatusCode: resp.StatusCode,
		Error: model.Error{
			Message: "",
			Type:    "upstream_error",
			Code:    "bad_response_status_code",
			Param:   strconv.Itoa(resp.StatusCode),
		},
	}

	// Parse Retry-After header from upstream response
	if retryAfterStr := resp.Header.Get("Retry-After"); retryAfterStr != "" {
		if seconds, err := strconv.Atoi(retryAfterStr); err == nil {
			// Integer seconds format: "120"
			if seconds > 0 {
				ErrorWithStatusCode.RetryAfterSeconds = &seconds
			}
		} else if retryAfterTime, err := time.Parse(time.RFC1123, retryAfterStr); err == nil {
			// HTTP-Date format: "Wed, 21 Oct 2015 07:28:00 GMT"
			seconds := int(time.Until(retryAfterTime).Seconds())
			if seconds < 0 {
				seconds = 0
			}
			ErrorWithStatusCode.RetryAfterSeconds = &seconds
		}
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if config.DebugEnabled {
		logger.SysLog(fmt.Sprintf("error happened, status code: %d, response: \n%s", resp.StatusCode, string(responseBody)))
	}
	err = resp.Body.Close()
	if err != nil {
		return
	}
	var errResponse GeneralErrorResponse
	err = json.Unmarshal(responseBody, &errResponse)
	if err != nil {
		return
	}
	if errResponse.Error.Message != "" {
		// OpenAI format error, so we override the default one
		ErrorWithStatusCode.Error = errResponse.Error
	} else {
		ErrorWithStatusCode.Error.Message = errResponse.ToMessage()
	}
	if ErrorWithStatusCode.Error.Message == "" {
		ErrorWithStatusCode.Error.Message = fmt.Sprintf("bad response status code %d", resp.StatusCode)
	}
	return
}
