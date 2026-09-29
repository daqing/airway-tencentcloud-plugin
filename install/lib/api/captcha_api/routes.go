package captcha_api

import (
	"github.com/gin-gonic/gin"
)

// Routes registers the captcha API on an API router group. Mounted under
// /api/v1, the endpoint is:
//
//	GET /api/v1/captcha  issue an image captcha
func Routes(r *gin.RouterGroup) {
	r.GET("/captcha", ShowAction)
}
