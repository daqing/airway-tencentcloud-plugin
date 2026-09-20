// Package tencentcloudplugin implements the tencentcloud Airway plugin.
package tencentcloudplugin

import (
	"github.com/daqing/airway/lib/plugin"
	"github.com/gin-gonic/gin"

	"github.com/daqing/airway-tencentcloud-plugin/install/lib/api/tencentcloud_api"
)

// Plugin is the tencentcloud feature module.
type Plugin struct{}

func (Plugin) Name() string      { return "tencentcloud" }
func (Plugin) MountPath() string { return "/api/v1/tencentcloud" }

func (Plugin) Routes(r *gin.RouterGroup) {
	tencentcloud_api.Routes(r)
}

func init() {
	plugin.Register(Plugin{})
}
