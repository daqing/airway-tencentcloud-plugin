package tencentcloud_api

import (
	"github.com/daqing/airway/lib/utils"
	"github.com/gin-gonic/gin"
)

// Routes registers the plugin's HTTP endpoints. The group is already mounted
// at the plugin's MountPath.
//
// The SMS endpoints are for local debugging only, so the plugin mounts them
// only when the host runs in local mode (AIRWAY_ENV=local). Neither endpoint
// authenticates its caller: /sms/send spends real SMS quota for anyone who
// can reach it, and /sms/send/mock hands out verification codes. Host
// applications call the Go API (tencentcloud.SendSms) in-process instead, so
// a production host serves no SMS route at all.
func Routes(r *gin.RouterGroup) {
	if !utils.AppConfig().IsLocal {
		return
	}

	DebugRoutes(r)
}

// DebugRoutes registers both SMS endpoints unconditionally, for callers that
// are local by construction: the standalone devserver, or a host that puts
// its own authentication in front of them. The endpoints are unauthenticated
// as registered here, so never mount them on a public router — see Routes
// for the default. OpenAPI declarations follow the routes, so a host that
// mounts these on a router other than the one `airway openapi:generate`
// enumerates will get "matches no registered route" warnings.
func DebugRoutes(r *gin.RouterGroup) {
	declareOpenAPI()

	r.POST("/sms/send", SmsSendAction)
	r.POST("/sms/send/mock", SmsSendMockAction)
}
