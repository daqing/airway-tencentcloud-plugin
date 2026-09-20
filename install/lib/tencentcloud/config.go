// Package tencentcloud wraps the Tencent Cloud Go SDK for Airway host
// applications. Credentials and SMS defaults are read from TENCENTCLOUD_*
// environment variables on first use; the SDK client is then cached for the
// lifetime of the process.
package tencentcloud

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	smsapi "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

const defaultRegion = "ap-guangzhou"

// Config is the plugin's environment-derived configuration.
type Config struct {
	SecretID    string
	SecretKey   string
	Region      string
	SmsSdkAppID string
	SignName    string
}

// LoadConfig reads the TENCENTCLOUD_* environment variables. Values that
// are empty after trimming read as unset. SecretID, SecretKey and
// SmsSdkAppID are required; Region defaults to ap-guangzhou, SignName is
// the default signature for domestic SMS and may be overridden per request.
//
// The host's main.go loads .env with godotenv.Load, which never overrides
// variables already present in the process environment, so reading
// os.Getenv directly keeps the shell-beats-.env precedence while staying
// controllable from tests via t.Setenv.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		SecretID:    strings.TrimSpace(os.Getenv("TENCENTCLOUD_SECRET_ID")),
		SecretKey:   strings.TrimSpace(os.Getenv("TENCENTCLOUD_SECRET_KEY")),
		Region:      strings.TrimSpace(os.Getenv("TENCENTCLOUD_REGION")),
		SmsSdkAppID: strings.TrimSpace(os.Getenv("TENCENTCLOUD_SMS_SDK_APP_ID")),
		SignName:    strings.TrimSpace(os.Getenv("TENCENTCLOUD_SMS_SIGN_NAME")),
	}
	if cfg.Region == "" {
		cfg.Region = defaultRegion
	}

	var missing []string
	if cfg.SecretID == "" {
		missing = append(missing, "TENCENTCLOUD_SECRET_ID")
	}
	if cfg.SecretKey == "" {
		missing = append(missing, "TENCENTCLOUD_SECRET_KEY")
	}
	if cfg.SmsSdkAppID == "" {
		missing = append(missing, "TENCENTCLOUD_SMS_SDK_APP_ID")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("tencentcloud plugin: environment variables not set: %s; add them to the host's .env (see .env.example)", strings.Join(missing, ", "))
	}

	return cfg, nil
}

var (
	setupOnce sync.Once
	cfg       *Config
	setupErr  error
	smsClient *smsapi.Client
)

// setup loads the config and builds the SDK client exactly once; later
// callers reuse the cached client.
func setup() {
	setupOnce.Do(func() {
		cfg, setupErr = LoadConfig()
		if setupErr != nil {
			return
		}

		credential := common.NewCredential(cfg.SecretID, cfg.SecretKey)
		smsClient, setupErr = smsapi.NewClient(credential, cfg.Region, profile.NewClientProfile())
	})
}

// resetForTest re-arms the lazy setup so tests can exercise different
// configurations. It is only for tests.
func resetForTest() {
	setupOnce = sync.Once{}
	cfg = nil
	setupErr = nil
	smsClient = nil
}
