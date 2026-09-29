package sms_codes_api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/daqing/airway/lib/render"
	"github.com/gin-gonic/gin"

	"github.com/daqing/airway-tencentcloud-plugin/install/lib/smsverify"
)

// Response codes. Every response is HTTP 200 and carries the outcome in the
// body's code, so these mirror the HTTP status each one replaces.
const (
	codeBadRequest      = 40001
	codeCaptchaRequired = 40301
	codeRateLimited     = 42901
)

// captchaURL points clients at the companion endpoint. It is the plugin's
// mount path plus the captcha route, so a host serving the API under a
// URL_PREFIX should rewrite it.
const captchaURL = "/api/v1/captcha"

// SendParams is the JSON body of the send endpoint; it doubles as the
// openapi.go request-body schema.
type SendParams struct {
	Phone         string `json:"phone"`
	CaptchaID     string `json:"captcha_id,omitempty"`
	CaptchaAnswer string `json:"captcha_answer,omitempty"`
}

// SendAction validates the phone, enforces the rate limits (per-phone
// cooldown and daily cap, per-IP daily cap) and sends a verification code.
// Once the requesting IP has been sending many codes, a captcha is required.
// In mock delivery mode the code comes back as dev_code.
func SendAction(c *gin.Context) {
	var params SendParams
	if err := c.ShouldBindJSON(&params); err != nil {
		fail(c, codeBadRequest, "invalid request body", nil)
		return
	}

	result, err := smsverify.Send(
		c.Request.Context(),
		strings.TrimSpace(params.Phone),
		c.ClientIP(),
		strings.TrimSpace(params.CaptchaID),
		strings.TrimSpace(params.CaptchaAnswer),
	)
	if err != nil {
		respondError(c, err)
		return
	}

	data := gin.H{"sent": true}
	if result.Mock {
		data["dev_code"] = result.Code
	}

	render.OK(c, data)
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, smsverify.ErrInvalidPhone), errors.Is(err, smsverify.ErrCaptchaInvalid):
		fail(c, codeBadRequest, err.Error(), nil)
	case errors.Is(err, smsverify.ErrRateLimited):
		fail(c, codeRateLimited, err.Error(), nil)
	case errors.Is(err, smsverify.ErrCaptchaRequired):
		fail(c, codeCaptchaRequired, err.Error(), gin.H{
			"captcha_required": true,
			"captcha_url":      captchaURL,
		})
	default:
		render.Error(c, err)
	}
}

// fail writes the framework's response envelope with a body-level error code
// and a data payload, mirroring render.ErrorCodeMsg. It exists because that
// helper always sends data: null, so it cannot express the captcha-required
// response.
func fail(c *gin.Context, code int, message string, data gin.H) {
	c.JSON(http.StatusOK, gin.H{"code": code, "data": data, "message": message})
	c.Abort()
}
