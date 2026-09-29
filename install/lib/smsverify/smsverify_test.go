package smsverify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/daqing/airway/lib/repo"
	"github.com/daqing/airway/lib/sql"

	"github.com/daqing/airway-tencentcloud-plugin/install/ignore/testdb"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/captcha"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/models"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/tencentcloud"
)

const (
	testPhone = "13800138000"
	testIP    = "10.0.0.1"
	testCode  = "123456"
)

func setupDB(t *testing.T) {
	t.Helper()

	db, err := testdb.Setup()
	if err != nil {
		t.Fatalf("setup database: %v", err)
	}

	t.Cleanup(func() { db.Close() })
}

// stubSendSms replaces the SDK call, so no test ever reaches Tencent Cloud.
func stubSendSms(t *testing.T, fn func(context.Context, tencentcloud.SendSmsInput) (*tencentcloud.SendSmsResult, error)) {
	t.Helper()

	original := deliverSMS
	deliverSMS = fn
	t.Cleanup(func() { deliverSMS = original })
}

// acceptSend stubs a delivery call that succeeds, and fails the test if it is
// reached at all when reached is false.
func acceptSend(t *testing.T, captured *tencentcloud.SendSmsInput) {
	t.Helper()

	stubSendSms(t, func(_ context.Context, in tencentcloud.SendSmsInput) (*tencentcloud.SendSmsResult, error) {
		if captured != nil {
			*captured = in
		}

		return &tencentcloud.SendSmsResult{SendStatuses: []tencentcloud.SendStatus{
			{PhoneNumber: in.PhoneNumbers[0], Code: "Ok", Message: "send success"},
		}}, nil
	})
}

// rejectDelivery stubs a delivery call that must never happen.
func rejectDelivery(t *testing.T) {
	t.Helper()

	stubSendSms(t, func(context.Context, tencentcloud.SendSmsInput) (*tencentcloud.SendSmsResult, error) {
		t.Fatal("SMS delivery must not be attempted")
		return nil, nil
	})
}

// seedCode writes one delivered code straight into the table, which is how the
// tests age rows past the resend cooldown without sleeping.
func seedCode(t *testing.T, phone, ip, code string, attempts int, createdAt time.Time) {
	t.Helper()

	if _, err := repo.CreateFrom[models.SmsVerification](sql.H{
		"phone":      phone,
		"code":       code,
		"ip":         ip,
		"expires_at": createdAt.Add(CodeTTL),
		"attempts":   attempts,
		"created_at": createdAt,
		"updated_at": createdAt,
	}); err != nil {
		t.Fatalf("seed code: %v", err)
	}
}

func send(t *testing.T, phone, ip string) (Result, error) {
	t.Helper()

	return Send(context.Background(), phone, ip, "", "")
}

func TestSendValidatesPhone(t *testing.T) {
	tests := []struct {
		name  string
		phone string
	}{
		{name: "empty", phone: ""},
		{name: "too short", phone: "1380013800"},
		{name: "too long", phone: "138001380000"},
		{name: "landline", phone: "01012345678"},
		{name: "unsupported prefix", phone: "12800138000"},
		{name: "country code included", phone: "+8613800138000"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupDB(t)
			rejectDelivery(t)
			t.Setenv("SMS_DRIVER", DriverTencent)
			t.Setenv("TENCENTCLOUD_SMS_TEMPLATE_ID", "1234567")

			if _, err := send(t, test.phone, testIP); !errors.Is(err, ErrInvalidPhone) {
				t.Fatalf("Send(%q) err = %v, want ErrInvalidPhone", test.phone, err)
			}

			count, err := repo.CountWhere[models.SmsVerification](sql.H{"phone": test.phone})
			if err != nil {
				t.Fatalf("count: %v", err)
			}
			if count != 0 {
				t.Fatalf("stored %d rows for an invalid phone", count)
			}
		})
	}
}

func TestDriverFollowsEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		driver  string
		env     string
		want    string
		wantErr bool
	}{
		{name: "unset is mock locally", driver: "", env: "local", want: DriverMock},
		{name: "unset is tencent elsewhere", driver: "", env: "production", want: DriverTencent},
		{name: "unset is tencent with no env at all", driver: "", env: "", want: DriverTencent},
		{name: "explicit mock overrides production", driver: DriverMock, env: "production", want: DriverMock},
		{name: "explicit tencent overrides local", driver: DriverTencent, env: "local", want: DriverTencent},
		{name: "unknown value is an error", driver: "aliyun", env: "local", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("AIRWAY_ENV", test.env)
			t.Setenv("SMS_DRIVER", test.driver)

			got, err := driver()
			if test.wantErr {
				if err == nil {
					t.Fatalf("driver() = %q, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("driver() err = %v", err)
			}
			if got != test.want {
				t.Fatalf("driver() = %q, want %q", got, test.want)
			}
		})
	}
}

// TestSendResolvesDriverBeforeSideEffects pins that a misconfigured driver is
// reported as such rather than as whatever the database says: this test sets up
// no database, so a Send that got past the driver check would fail on a closed
// connection instead.
func TestSendResolvesDriverBeforeSideEffects(t *testing.T) {
	t.Setenv("SMS_DRIVER", "aliyun")

	_, err := send(t, testPhone, testIP)
	if err == nil || !strings.Contains(err.Error(), "unknown SMS_DRIVER") {
		t.Fatalf("Send err = %v, want it to name the unknown SMS_DRIVER", err)
	}
}

func TestSendCooldown(t *testing.T) {
	setupDB(t)
	acceptSend(t, nil)
	t.Setenv("SMS_DRIVER", DriverMock)

	if _, err := send(t, testPhone, testIP); err != nil {
		t.Fatalf("first Send: %v", err)
	}

	if _, err := send(t, testPhone, testIP); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("second Send err = %v, want ErrRateLimited", err)
	}
}

func TestSendCapsPerPhonePerDay(t *testing.T) {
	setupDB(t)
	rejectDelivery(t)
	t.Setenv("SMS_DRIVER", DriverMock)

	// Older than the cooldown, inside the 24h window.
	for i := 0; i < MaxPerPhonePerDay; i++ {
		seedCode(t, testPhone, testIP, testCode, 0, time.Now().Add(-2*time.Hour))
	}

	if _, err := send(t, testPhone, testIP); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Send err = %v, want ErrRateLimited", err)
	}
}

func TestSendCapsPerIPPerDay(t *testing.T) {
	setupDB(t)
	rejectDelivery(t)
	t.Setenv("SMS_DRIVER", DriverMock)

	// Distinct phones, so only the per-IP cap can be what stops the send.
	for i := 0; i < MaxPerIPPerDay; i++ {
		seedCode(t, fmt.Sprintf("1380013%04d", i), testIP, testCode, 0, time.Now().Add(-2*time.Hour))
	}

	if _, err := send(t, "13900139000", testIP); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Send err = %v, want ErrRateLimited", err)
	}
}

func TestSendRequiresCaptchaPastHourlyQuota(t *testing.T) {
	setupDB(t)
	acceptSend(t, nil)
	t.Setenv("SMS_DRIVER", DriverMock)

	ip := testIP
	for i := 0; i < CaptchaThresholdPerHour; i++ {
		seedCode(t, fmt.Sprintf("1380013%04d", i), ip, testCode, 0, time.Now().Add(-time.Minute))
	}

	phone := "13900139000"

	t.Run("without a captcha", func(t *testing.T) {
		if _, err := send(t, phone, ip); !errors.Is(err, ErrCaptchaRequired) {
			t.Fatalf("Send err = %v, want ErrCaptchaRequired", err)
		}
	})

	t.Run("with a wrong answer", func(t *testing.T) {
		id, answer := issueCaptcha(t, ip)

		// Longer than the real answer, so it can never match whatever was drawn.
		if _, err := Send(context.Background(), phone, ip, id, answer+"0"); !errors.Is(err, ErrCaptchaInvalid) {
			t.Fatalf("Send err = %v, want ErrCaptchaInvalid", err)
		}
	})

	t.Run("with the right answer", func(t *testing.T) {
		id, answer := issueCaptcha(t, ip)

		if _, err := Send(context.Background(), phone, ip, id, answer); err != nil {
			t.Fatalf("Send err = %v", err)
		}
	})
}

// issueCaptcha issues a captcha bound to ip and returns its id along with the
// answer the image encodes, which is what the client would type back.
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

func TestSendRetiresThePreviousCode(t *testing.T) {
	setupDB(t)
	acceptSend(t, nil)
	t.Setenv("SMS_DRIVER", DriverMock)

	seedCode(t, testPhone, testIP, "000000", 0, time.Now().Add(-2*time.Minute))

	result, err := send(t, testPhone, testIP)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	rows, err := repo.FindBy[models.SmsVerification](sql.H{"phone": testPhone, "consumed": false})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(rows) != 1 || rows[0].Code != result.Code {
		t.Fatalf("live codes = %#v, want exactly %q", rows, result.Code)
	}
}

func TestSendDelivers(t *testing.T) {
	t.Run("mock driver hands the code back", func(t *testing.T) {
		setupDB(t)
		rejectDelivery(t)
		t.Setenv("SMS_DRIVER", DriverMock)

		result, err := send(t, testPhone, testIP)
		if err != nil {
			t.Fatalf("Send: %v", err)
		}

		if !result.Mock {
			t.Fatal("Mock = false for the mock driver")
		}
		if len(result.Code) != codeLength || strings.Trim(result.Code, "0123456789") != "" {
			t.Fatalf("Code = %q, want %d digits", result.Code, codeLength)
		}
	})

	t.Run("tencent driver sends a template SMS", func(t *testing.T) {
		setupDB(t)
		t.Setenv("SMS_DRIVER", DriverTencent)
		t.Setenv("TENCENTCLOUD_SMS_TEMPLATE_ID", "1234567")

		var captured tencentcloud.SendSmsInput
		acceptSend(t, &captured)

		result, err := send(t, testPhone, testIP)
		if err != nil {
			t.Fatalf("Send: %v", err)
		}

		if result.Mock {
			t.Fatal("Mock = true for the tencent driver")
		}
		if len(captured.PhoneNumbers) != 1 || captured.PhoneNumbers[0] != "+86"+testPhone {
			t.Fatalf("PhoneNumbers = %v, want [+86%s]", captured.PhoneNumbers, testPhone)
		}
		if captured.TemplateID != "1234567" {
			t.Fatalf("TemplateID = %q", captured.TemplateID)
		}
		if len(captured.TemplateParamSet) != 1 || captured.TemplateParamSet[0] != result.Code {
			t.Fatalf("TemplateParamSet = %v, want [%s]", captured.TemplateParamSet, result.Code)
		}
	})

	t.Run("tencent driver without a template id fails", func(t *testing.T) {
		setupDB(t)
		rejectDelivery(t)
		t.Setenv("SMS_DRIVER", DriverTencent)
		t.Setenv("TENCENTCLOUD_SMS_TEMPLATE_ID", "")

		if _, err := send(t, testPhone, testIP); err == nil {
			t.Fatal("Send succeeded without TENCENTCLOUD_SMS_TEMPLATE_ID")
		}
	})

	t.Run("a rejected number surfaces the provider message", func(t *testing.T) {
		setupDB(t)
		t.Setenv("SMS_DRIVER", DriverTencent)
		t.Setenv("TENCENTCLOUD_SMS_TEMPLATE_ID", "1234567")

		stubSendSms(t, func(_ context.Context, in tencentcloud.SendSmsInput) (*tencentcloud.SendSmsResult, error) {
			return &tencentcloud.SendSmsResult{SendStatuses: []tencentcloud.SendStatus{
				{PhoneNumber: in.PhoneNumbers[0], Code: "FailedOperation.PhoneNumberInBlacklist", Message: "blacklisted"},
			}}, nil
		})

		_, err := send(t, testPhone, testIP)
		if err == nil || !strings.Contains(err.Error(), "blacklisted") {
			t.Fatalf("Send err = %v, want it to mention the provider message", err)
		}
	})
}

func TestVerifyAcceptsAndConsumes(t *testing.T) {
	setupDB(t)

	seedCode(t, testPhone, testIP, testCode, 0, time.Now())

	if err := Verify(context.Background(), testPhone, testCode); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if err := Verify(context.Background(), testPhone, testCode); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("second Verify err = %v, want ErrCodeInvalid", err)
	}
}

func TestVerifyRejects(t *testing.T) {
	tests := []struct {
		name     string
		phone    string
		code     string
		seed     bool
		attempts int
		age      time.Duration
		want     error
	}{
		{name: "no code was ever sent", phone: testPhone, code: testCode, want: ErrCodeInvalid},
		{name: "code belongs to another phone", phone: "13900139000", code: testCode, seed: true, want: ErrCodeInvalid},
		{name: "wrong code", phone: testPhone, code: "654321", seed: true, want: ErrCodeInvalid},
		{name: "expired", phone: testPhone, code: testCode, seed: true, age: CodeTTL + time.Minute, want: ErrCodeInvalid},
		{
			name: "attempts exhausted", phone: testPhone, code: "654321", seed: true,
			attempts: MaxVerifyAttempts, want: ErrCodeTooManyAttempts,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupDB(t)

			if test.seed {
				seedCode(t, testPhone, testIP, testCode, test.attempts, time.Now().Add(-test.age))
			}

			if err := Verify(context.Background(), test.phone, test.code); !errors.Is(err, test.want) {
				t.Fatalf("Verify err = %v, want %v", err, test.want)
			}
		})
	}
}

func TestVerifyCountsWrongAttempts(t *testing.T) {
	setupDB(t)

	seedCode(t, testPhone, testIP, testCode, 0, time.Now())

	if err := Verify(context.Background(), testPhone, "654321"); !errors.Is(err, ErrCodeInvalid) {
		t.Fatalf("Verify err = %v, want ErrCodeInvalid", err)
	}

	row, err := repo.FindOneBy[models.SmsVerification](sql.H{"phone": testPhone})
	if err != nil || row == nil {
		t.Fatalf("row = %#v, err = %v", row, err)
	}
	if row.Attempts != 1 {
		t.Fatalf("Attempts = %d, want 1", row.Attempts)
	}
	if row.Consumed {
		t.Fatal("a wrong guess consumed the code")
	}

	if err := Verify(context.Background(), testPhone, testCode); err != nil {
		t.Fatalf("the correct code was rejected after one wrong guess: %v", err)
	}
}
