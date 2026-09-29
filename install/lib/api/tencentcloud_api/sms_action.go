package tencentcloud_api

import (
	"fmt"

	"github.com/daqing/airway/lib/render"
	"github.com/gin-gonic/gin"

	"github.com/daqing/airway-tencentcloud-plugin/install/lib/tencentcloud"
)

// Tencent Cloud accepts at most 200 phone numbers per SendSms request.
const maxPhoneNumbers = 200

// sendSms indirects the tencentcloud package so tests can stub the SDK call.
var sendSms = tencentcloud.SendSms

// SmsSendRequest is the JSON body of both send endpoints; it doubles as the
// openapi.go request-body schema, so omitempty marks the fields validate()
// treats as optional.
type SmsSendRequest struct {
	PhoneNumbers     []string `json:"phone_numbers"`
	TemplateID       string   `json:"template_id"`
	SignName         string   `json:"sign_name,omitempty"`
	TemplateParamSet []string `json:"template_param_set,omitempty"`
	SessionContext   string   `json:"session_context,omitempty"`
}

// validate applies the send rules shared by the real and mock actions so
// the two endpoints reject the same requests; it returns the error message
// or "" when the request is valid.
func (req SmsSendRequest) validate() string {
	if len(req.PhoneNumbers) == 0 {
		return "phone_numbers must not be empty"
	}
	if len(req.PhoneNumbers) > maxPhoneNumbers {
		return fmt.Sprintf("phone_numbers supports at most %d numbers per request", maxPhoneNumbers)
	}
	if req.TemplateID == "" {
		return "template_id must not be empty"
	}
	return ""
}

// SmsSendAction sends SMS through Tencent Cloud. Each request must target
// one template; phone numbers and template params map onto the template's
// variables in order.
func SmsSendAction(c *gin.Context) {
	var req SmsSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		render.ErrorMessage(c, "invalid request body: "+err.Error())
		return
	}
	if msg := req.validate(); msg != "" {
		render.ErrorMessage(c, msg)
		return
	}

	result, err := sendSms(c.Request.Context(), tencentcloud.SendSmsInput{
		PhoneNumbers:     req.PhoneNumbers,
		TemplateID:       req.TemplateID,
		SignName:         req.SignName,
		TemplateParamSet: req.TemplateParamSet,
		SessionContext:   req.SessionContext,
	})
	if err != nil {
		render.Error(c, err)
		return
	}

	render.OK(c, result)
}
