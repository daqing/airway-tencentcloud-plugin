package tencentcloudplugin

import (
	"io/fs"
	"slices"
	"strings"
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

// TestPluginRoutesOnlyInLocalMode pins the plugin's HTTP attack surface: the
// SMS endpoints spend real quota and hand out verification codes without
// authenticating anyone, so they exist only when the host runs in local mode.
func TestPluginRoutesOnlyInLocalMode(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want []string
	}{
		{name: "unset", env: "", want: nil},
		{name: "production", env: "production", want: nil},
		{
			name: "local",
			env:  "local",
			want: []string{
				"POST /api/v1/tencentcloud/sms/send",
				"POST /api/v1/tencentcloud/sms/send/mock",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("AIRWAY_ENV", test.env)

			gin.SetMode(gin.TestMode)
			r := gin.New()

			Plugin{}.Routes(r.Group(Plugin{}.MountPath()))

			var got []string
			for _, route := range r.Routes() {
				got = append(got, route.Method+" "+route.Path)
			}
			slices.Sort(got)

			if !slices.Equal(got, test.want) {
				t.Fatalf("routes = %v, want %v", got, test.want)
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

// TestPluginsProvideMigrations covers the installer's lookup: it resolves the
// plugin by module, which yields the name "tencentcloud", and reads the
// migrations from whichever provider it finds. Both implement the interface so
// that lookup can never miss and fall back to the module zip.
func TestPluginsProvideMigrations(t *testing.T) {
	for _, p := range []plugin.Plugin{Plugin{}, SmsVerifyPlugin{}} {
		if _, ok := p.(plugin.MigrationProvider); !ok {
			t.Fatalf("%s does not implement plugin.MigrationProvider", p.Name())
		}
	}
}

// TestMigrationPairsAreComplete guards a hard failure in plugin:install, which
// refuses an up migration with no down beside it.
func TestMigrationPairsAreComplete(t *testing.T) {
	ups := map[string]bool{}
	downs := map[string]bool{}

	err := fs.WalkDir(SmsVerifyPlugin{}.MigrationFS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".up.sql") {
			ups[path] = true
		}
		if strings.HasSuffix(path, ".down.sql") {
			downs[path] = true
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk migrations: %v", err)
	}

	if len(ups) != 2 {
		t.Fatalf("up migrations = %v, want 2", ups)
	}
	for up := range ups {
		if down := strings.TrimSuffix(up, ".up.sql") + ".down.sql"; !downs[down] {
			t.Fatalf("%s has no matching down migration", up)
		}
	}
}
