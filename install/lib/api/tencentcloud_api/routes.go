package tencentcloud_api

import (
	"github.com/gin-gonic/gin"
)

// Routes registers the plugin's HTTP endpoints. The group is already mounted
// at the plugin's MountPath.
func Routes(r *gin.RouterGroup) {
	r.POST("/sms/send", SmsSendAction)
}
