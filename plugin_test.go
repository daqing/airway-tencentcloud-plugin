package tencentcloudplugin

import (
	"testing"

	"github.com/daqing/airway/lib/plugin"
	"github.com/gin-gonic/gin"
)

func TestPluginIsRegistered(t *testing.T) {
	found := plugin.Find("tencentcloud")
	if found == nil {
		t.Fatal("init() did not register the tencentcloud plugin")
	}
}

func TestPluginContract(t *testing.T) {
	p := Plugin{}

	if got := p.Name(); got != "tencentcloud" {
		t.Fatalf("Name() = %q, want tencentcloud", got)
	}
	if got := p.MountPath(); got != "/api/v1/tencentcloud" {
		t.Fatalf("MountPath() = %q, want /api/v1/tencentcloud", got)
	}
}

func TestPluginRoutesRegistersSmsSend(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	Plugin{}.Routes(r.Group(Plugin{}.MountPath()))

	for _, route := range r.Routes() {
		if route.Method == "POST" && route.Path == "/api/v1/tencentcloud/sms/send" {
			return
		}
	}
	t.Fatalf("POST /api/v1/tencentcloud/sms/send not registered; routes = %#v", r.Routes())
}
