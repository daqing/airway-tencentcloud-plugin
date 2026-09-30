package tencentcloudplugin

import (
	"slices"
	"testing"

	"github.com/daqing/airway/lib/migrate/schema"
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

// TestPluginMountsNoRoutes pins the plugin's HTTP attack surface at zero. The
// SMS endpoints it used to expose spent real quota and handed out verification
// codes without authenticating anyone; hosts call the Go API
// (tencentcloud.SendSms) in-process instead, and the endpoints clients reach
// belong to the smsverify plugin.
func TestPluginMountsNoRoutes(t *testing.T) {
	for _, env := range []string{"", "production", "local"} {
		t.Run("env="+env, func(t *testing.T) {
			t.Setenv("AIRWAY_ENV", env)

			gin.SetMode(gin.TestMode)
			r := gin.New()

			Plugin{}.Routes(r.Group(Plugin{}.MountPath()))

			if got := r.Routes(); len(got) != 0 {
				t.Fatalf("routes = %v, want none", got)
			}
		})
	}
}

func TestSmsVerifyPluginIsRegistered(t *testing.T) {
	if plugin.Find("smsverify") == nil {
		t.Fatal("init() did not register the smsverify plugin")
	}
}

func TestSmsVerifyPluginContract(t *testing.T) {
	p := SmsVerifyPlugin{}

	if got := p.Name(); got != "smsverify" {
		t.Fatalf("Name() = %q, want smsverify", got)
	}
	if got := p.MountPath(); got != "/api/v1" {
		t.Fatalf("MountPath() = %q, want /api/v1", got)
	}
}

// TestSmsVerifyPluginRoutesAreAlwaysMounted pins the difference from the
// tencentcloud plugin: these two endpoints exist in every environment, because
// clients call them directly and smsverify's rate limits — not an
// authentication gate — are what protects them.
func TestSmsVerifyPluginRoutesAreAlwaysMounted(t *testing.T) {
	for _, env := range []string{"", "production", "local"} {
		t.Run("env="+env, func(t *testing.T) {
			t.Setenv("AIRWAY_ENV", env)

			gin.SetMode(gin.TestMode)
			r := gin.New()

			SmsVerifyPlugin{}.Routes(r.Group(SmsVerifyPlugin{}.MountPath()))

			var got []string
			for _, route := range r.Routes() {
				got = append(got, route.Method+" "+route.Path)
			}
			slices.Sort(got)

			want := []string{"GET /api/v1/captcha", "POST /api/v1/sms_codes"}
			if !slices.Equal(got, want) {
				t.Fatalf("routes = %v, want %v", got, want)
			}
		})
	}
}

// TestMigrationsAreRegistered covers what the host's `db:migrate` needs: the
// blank import above compiles the Go migrations in, and each one must carry
// both directions so `db:rollback` has something to run. The versions match the
// SQL migrations this module used to ship, so a host that already applied those
// skips these rather than re-creating the tables.
func TestMigrationsAreRegistered(t *testing.T) {
	want := map[string]string{
		"20260929153501": "create_sms_verifications",
		"20260929153502": "create_captchas",
	}

	found := map[string]schema.Definition{}
	for _, def := range schema.Definitions() {
		found[def.Version] = def
	}

	for version, name := range want {
		def, ok := found[version]
		if !ok {
			t.Fatalf("migration %s is not registered", version)
		}
		if def.Name != name {
			t.Fatalf("migration %s name = %q, want %q", version, def.Name, name)
		}
		if len(def.UpOps) == 0 || len(def.DownOps) == 0 {
			t.Fatalf("migration %s has %d up ops and %d down ops, want both non-empty",
				version, len(def.UpOps), len(def.DownOps))
		}
	}
}
