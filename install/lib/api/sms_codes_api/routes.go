package sms_codes_api

import (
	"github.com/gin-gonic/gin"
)

// Routes registers the SMS verification code API on an API router group.
// Mounted under /api/v1, the endpoint is:
//
//	POST /api/v1/sms_codes  send a verification code to a phone
//
// The endpoint is public by design — clients call it directly — and protects
// itself with the rate limits in smsverify rather than with authentication.
func Routes(r *gin.RouterGroup) {
	r.POST("/sms_codes", SendAction)
}
