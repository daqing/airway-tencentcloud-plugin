// Package tencentcloudplugin implements the Airway plugins for Tencent Cloud
// services and for phone verification codes.
package tencentcloudplugin

import (
	"github.com/daqing/airway/lib/plugin"
	"github.com/gin-gonic/gin"

	"github.com/daqing/airway-tencentcloud-plugin/install/lib/api/captcha_api"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/api/sms_codes_api"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/models"

	// The schema arrives as Go migrations: importing them registers the tables
	// with lib/migrate, so the host's db:migrate creates them from the module
	// zip without a file ever being copied into the host project.
	_ "github.com/daqing/airway-tencentcloud-plugin/install/lib/migrations"
)

// Plugin is the tencentcloud feature module. Hosts use its Go API rather than
// its (nonexistent) HTTP routes.
type Plugin struct{}

func (Plugin) Name() string      { return "tencentcloud" }
func (Plugin) MountPath() string { return "/api/v1/tencentcloud" }

// Routes mounts the plugin's HTTP routes, of which there are none: hosts
// call the Go API (tencentcloud.SendSms) in-process, and the phone
// verification flow that clients reach over HTTP belongs to the smsverify
// plugin. The type stays registered so plugin:install still resolves the
// module, and so the migrations compiled in beside it are part of the host
// binary.
func (Plugin) Routes(*gin.RouterGroup) {}

// SmsVerifyPlugin is the phone verification module: it issues image captchas
// and rate-limited login codes over SMS.
type SmsVerifyPlugin struct{}

func (SmsVerifyPlugin) Name() string      { return "smsverify" }
func (SmsVerifyPlugin) MountPath() string { return "/api/v1" }

// Routes mounts the two public endpoints unconditionally: clients call them
// directly, and they defend themselves with smsverify's rate limits rather
// than with an authentication gate. They are the module's only HTTP surface.
func (SmsVerifyPlugin) Routes(r *gin.RouterGroup) {
	sms_codes_api.Routes(r)
	captcha_api.Routes(r)
}

func (SmsVerifyPlugin) REPLModels() map[string]any { return models.REPLModels() }

func init() {
	plugin.Register(Plugin{})
	plugin.Register(SmsVerifyPlugin{})
}
