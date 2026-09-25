package tencentcloud

import (
	"strings"
	"testing"
)

var envKeys = []string{
	"TENCENTCLOUD_SECRET_ID",
	"TENCENTCLOUD_SECRET_KEY",
	"TENCENTCLOUD_REGION",
	"TENCENTCLOUD_SMS_SDK_APP_ID",
	"TENCENTCLOUD_SMS_SIGN_NAME",
}

// setTencentCloudEnv sets every TENCENTCLOUD_* variable; keys missing from
// values are set to the empty string, which reads as unset.
func setTencentCloudEnv(t *testing.T, values map[string]string) {
	t.Helper()
	for _, key := range envKeys {
		t.Setenv(key, values[key])
	}
}

func TestLoadConfigReadsEnvironment(t *testing.T) {
	setTencentCloudEnv(t, map[string]string{
		"TENCENTCLOUD_SECRET_ID":      "id-123",
		"TENCENTCLOUD_SECRET_KEY":     "key-456",
		"TENCENTCLOUD_REGION":         "ap-shanghai",
		"TENCENTCLOUD_SMS_SDK_APP_ID": "1400006666",
		"TENCENTCLOUD_SMS_SIGN_NAME":  "Airway",
	})

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.SecretID != "id-123" || cfg.SecretKey != "key-456" {
		t.Fatalf("unexpected credentials: %#v", cfg)
	}
	if cfg.Region != "ap-shanghai" || cfg.SmsSdkAppID != "1400006666" || cfg.SignName != "Airway" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadConfigDefaultsRegion(t *testing.T) {
	setTencentCloudEnv(t, map[string]string{
		"TENCENTCLOUD_SECRET_ID":      "id-123",
		"TENCENTCLOUD_SECRET_KEY":     "key-456",
		"TENCENTCLOUD_SMS_SDK_APP_ID": "1400006666",
	})

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Region != defaultRegion {
		t.Fatalf("Region = %q, want %q", cfg.Region, defaultRegion)
	}
}

func TestLoadConfigReportsMissingVariables(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		// missing lists the variables the error message must mention;
		// present lists the ones it must not.
		missing []string
		present []string
	}{
		{
			name:    "nothing set",
			values:  map[string]string{},
			missing: []string{"TENCENTCLOUD_SECRET_ID", "TENCENTCLOUD_SECRET_KEY", "TENCENTCLOUD_SMS_SDK_APP_ID"},
		},
		{
			name: "only secret id set",
			values: map[string]string{
				"TENCENTCLOUD_SECRET_ID": "id-123",
			},
			missing: []string{"TENCENTCLOUD_SECRET_KEY", "TENCENTCLOUD_SMS_SDK_APP_ID"},
			present: []string{"TENCENTCLOUD_SECRET_ID"},
		},
		{
			name: "whitespace values read as unset",
			values: map[string]string{
				"TENCENTCLOUD_SECRET_ID":      " id-123 ",
				"TENCENTCLOUD_SECRET_KEY":     "key-456",
				"TENCENTCLOUD_SMS_SDK_APP_ID": "   ",
			},
			missing: []string{"TENCENTCLOUD_SMS_SDK_APP_ID"},
			present: []string{"TENCENTCLOUD_SECRET_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setTencentCloudEnv(t, test.values)

			cfg, err := LoadConfig()
			if err == nil {
				t.Fatalf("LoadConfig succeeded: %#v", cfg)
			}
			for _, name := range test.missing {
				if !strings.Contains(err.Error(), name) {
					t.Fatalf("error %q does not mention %s", err, name)
				}
			}
			for _, name := range test.present {
				if strings.Contains(err.Error(), name) {
					t.Fatalf("error %q unexpectedly mentions %s", err, name)
				}
			}
		})
	}
}
