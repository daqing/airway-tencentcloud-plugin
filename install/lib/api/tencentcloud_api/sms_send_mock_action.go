package tencentcloud_api

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/daqing/airway/lib/render"
	"github.com/gin-gonic/gin"

	"github.com/daqing/airway-tencentcloud-plugin/install/lib/tencentcloud"
)

// mockCodeDigits is the length of the verification code the mock endpoint
// generates.
const mockCodeDigits = 6

// randomCode indirects code generation so tests can pin the value.
var randomCode = func() (string, error) {
	n, err := rand.Int(rand.Reader, new(big.Int).Exp(big.NewInt(10), big.NewInt(mockCodeDigits), nil))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", mockCodeDigits, n.Int64()), nil
}

// SmsSendMockAction mirrors SmsSendAction for local debugging: it binds the
// same request struct, applies the same validation, and renders the same
// SendSmsResult type, so the two endpoints share one response contract and
// clients can switch between them without any code changes. Nothing
// reaches Tencent Cloud; the generated verification code is the value of
// request_id and every status's serial_no. It hands out codes to anyone
// who calls it — never expose it in production.
func SmsSendMockAction(c *gin.Context) {
	var req SmsSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		render.ErrorMessage(c, "invalid request body: "+err.Error())
		return
	}
	if msg := req.validate(); msg != "" {
		render.ErrorMessage(c, msg)
		return
	}

	code, err := randomCode()
	if err != nil {
		render.Error(c, err)
		return
	}

	statuses := make([]tencentcloud.SendStatus, 0, len(req.PhoneNumbers))
	for _, phone := range req.PhoneNumbers {
		statuses = append(statuses, tencentcloud.SendStatus{
			SerialNo:       code,
			PhoneNumber:    phone,
			Fee:            1,
			SessionContext: req.SessionContext,
			Code:           "Ok",
			Message:        "mock send success",
		})
	}

	render.OK(c, &tencentcloud.SendSmsResult{
		RequestID:    code,
		SendStatuses: statuses,
	})
}
