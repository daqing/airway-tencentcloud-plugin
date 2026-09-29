package sms_codes_api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/daqing/airway/lib/repo"
	"github.com/daqing/airway/lib/sql"
	"github.com/gin-gonic/gin"

	"github.com/daqing/airway-tencentcloud-plugin/install/ignore/testdb"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/captcha"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/models"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/smsverify"
)

const (
	testIP    = "10.0.0.1"
	testPhone = "13800138000"
)

// envelope is the framework's response shape; every status travels in Code.
type envelope struct {
	Code    int            `json:"code"`
	Data    map[string]any `json:"data"`
	Message string         `json:"message"`
}

func setupRouter(t *testing.T) *gin.Engine {
	t.Helper()

	db, err := testdb.Setup()
	if err != nil {
		t.Fatalf("setup database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	Routes(r.Group("/api/v1"))

	return r
}

func postSend(t *testing.T, r *gin.Engine, ip, body string) envelope {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sms_codes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":12345"

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}

	return env
}

// seedCodes writes n codes from ip, which is how the tests age an IP past the
// captcha threshold without sending anything.
func seedCodes(t *testing.T, ip string, n int, age time.Duration) {
	t.Helper()

	now := time.Now()
	for i := 0; i < n; i++ {
		if _, err := repo.CreateFrom[models.SmsVerification](sql.H{
			"phone":      fmt.Sprintf("1380013%04d", i),
			"code":       "123456",
			"ip":         ip,
			"expires_at": now.Add(smsverify.CodeTTL),
			"created_at": now.Add(-age),
			"updated_at": now.Add(-age),
		}); err != nil {
			t.Fatalf("seed code: %v", err)
		}
	}
}

// issueCaptcha returns a captcha id bound to ip along with its answer.
func issueCaptcha(t *testing.T, ip string) (id, answer string) {
	t.Helper()

	id, _, err := captcha.Issue(context.Background(), ip)
	if err != nil {
		t.Fatalf("captcha.Issue: %v", err)
	}

	row, err := repo.FindOneBy[models.Captcha](sql.H{"id": id})
	if err != nil || row == nil {
		t.Fatalf("captcha row = %#v, err = %v", row, err)
	}

	return id, row.Answer
}

func TestSendActionRejectsBadInput(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		want  int
		match string
	}{
		{name: "malformed json", body: `not json`, want: codeBadRequest, match: "invalid request body"},
		{name: "missing phone", body: `{}`, want: codeBadRequest, match: "invalid phone number"},
		{name: "invalid phone", body: `{"phone":"12345"}`, want: codeBadRequest, match: "invalid phone number"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("SMS_DRIVER", smsverify.DriverMock)

			env := postSend(t, setupRouter(t), testIP, test.body)

			if env.Code != test.want {
				t.Fatalf("code = %d, want %d; body = %+v", env.Code, test.want, env)
			}
			if !strings.Contains(env.Message, test.match) {
				t.Fatalf("message = %q, want it to contain %q", env.Message, test.match)
			}
			if env.Data != nil {
				t.Fatalf("data = %#v, want null", env.Data)
			}
		})
	}
}

func TestSendActionReportsSuccess(t *testing.T) {
	t.Setenv("SMS_DRIVER", smsverify.DriverMock)

	env := postSend(t, setupRouter(t), testIP, `{"phone":"`+testPhone+`"}`)

	if env.Code != 0 || env.Message != "" {
		t.Fatalf("code = %d, message = %q", env.Code, env.Message)
	}
	if env.Data["sent"] != true {
		t.Fatalf("data = %#v, want sent: true", env.Data)
	}

	// Only the mock driver may hand the code back to the caller.
	code, _ := env.Data["dev_code"].(string)
	if len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		t.Fatalf("dev_code = %q, want 6 digits", code)
	}
}

func TestSendActionHidesTheCodeInTencentMode(t *testing.T) {
	// A missing template id fails the send before the SDK is ever called, so
	// this never reaches Tencent Cloud.
	t.Setenv("SMS_DRIVER", smsverify.DriverTencent)
	t.Setenv("TENCENTCLOUD_SMS_TEMPLATE_ID", "")

	env := postSend(t, setupRouter(t), testIP, `{"phone":"`+testPhone+`"}`)

	if env.Code == 0 {
		t.Fatalf("code = 0, want a failure; body = %+v", env)
	}
	if !strings.Contains(env.Message, "TENCENTCLOUD_SMS_TEMPLATE_ID") {
		t.Fatalf("message = %q", env.Message)
	}
	if env.Data != nil {
		t.Fatalf("data = %#v, want null", env.Data)
	}
}

func TestSendActionRateLimitsTheResend(t *testing.T) {
	t.Setenv("SMS_DRIVER", smsverify.DriverMock)

	r := setupRouter(t)
	body := `{"phone":"` + testPhone + `"}`

	if env := postSend(t, r, testIP, body); env.Code != 0 {
		t.Fatalf("first send: %+v", env)
	}

	env := postSend(t, r, testIP, body)
	if env.Code != codeRateLimited {
		t.Fatalf("code = %d, want %d; body = %+v", env.Code, codeRateLimited, env)
	}
}

func TestSendActionAsksForACaptcha(t *testing.T) {
	t.Setenv("SMS_DRIVER", smsverify.DriverMock)

	r := setupRouter(t)
	seedCodes(t, testIP, smsverify.CaptchaThresholdPerHour, time.Minute)

	env := postSend(t, r, testIP, `{"phone":"13900139000"}`)

	if env.Code != codeCaptchaRequired {
		t.Fatalf("code = %d, want %d; body = %+v", env.Code, codeCaptchaRequired, env)
	}
	if env.Data["captcha_required"] != true {
		t.Fatalf("data = %#v, want captcha_required: true", env.Data)
	}
	if env.Data["captcha_url"] != captchaURL {
		t.Fatalf("captcha_url = %#v, want %q", env.Data["captcha_url"], captchaURL)
	}
}

func TestSendActionAcceptsACaptcha(t *testing.T) {
	t.Setenv("SMS_DRIVER", smsverify.DriverMock)

	r := setupRouter(t)
	seedCodes(t, testIP, smsverify.CaptchaThresholdPerHour, time.Minute)

	id, _ := issueCaptcha(t, testIP)

	t.Run("wrong answer", func(t *testing.T) {
		env := postSend(t, r, testIP,
			fmt.Sprintf(`{"phone":"13900139000","captcha_id":%q,"captcha_answer":"99999"}`, id))

		if env.Code != codeBadRequest || !strings.Contains(env.Message, "invalid captcha") {
			t.Fatalf("body = %+v, want an invalid captcha error", env)
		}
	})

	t.Run("right answer", func(t *testing.T) {
		// The wrong answer above burned that captcha, so issue another.
		id, answer := issueCaptcha(t, testIP)

		env := postSend(t, r, testIP,
			fmt.Sprintf(`{"phone":"13900139000","captcha_id":%q,"captcha_answer":%q}`, id, answer))

		if env.Code != 0 || env.Data["sent"] != true {
			t.Fatalf("body = %+v, want a successful send", env)
		}
	})
}
