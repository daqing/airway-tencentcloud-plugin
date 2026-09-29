// Package smsverify issues and verifies phone verification codes for
// login/sign-up. Sending is rate-limited per phone and per IP; once an IP
// sends too many codes per hour, a captcha is required. Codes live for 5
// minutes and are consumed by Verify.
//
// Delivery goes through the tencentcloud package in this same plugin. The
// SMS_DRIVER environment variable selects the channel: "mock" only logs the
// code (and lets Send hand it back to the caller), "tencent" sends a real
// template SMS using the TENCENTCLOUD_SMS_TEMPLATE_ID template with the code
// as its single parameter. When SMS_DRIVER is unset the driver follows the
// environment — mock in local mode, tencent everywhere else — so a production
// host can never fall back to mock by forgetting to set it.
package smsverify

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/daqing/airway/lib/repo"
	"github.com/daqing/airway/lib/sql"
	"github.com/daqing/airway/lib/utils"

	"github.com/daqing/airway-tencentcloud-plugin/install/lib/captcha"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/models"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/tencentcloud"
)

// Delivery drivers, selected by SMS_DRIVER.
const (
	// DriverMock logs the code instead of sending it, and is the only driver
	// whose code may be handed back to the caller.
	DriverMock = "mock"
	// DriverTencent sends a real template SMS through Tencent Cloud.
	DriverTencent = "tencent"
)

var (
	ErrInvalidPhone    = errors.New("invalid phone number")
	ErrRateLimited     = errors.New("too many requests, please try again later")
	ErrCaptchaRequired = errors.New("captcha required")
	ErrCaptchaInvalid  = errors.New("invalid captcha")

	// ErrCodeInvalid covers a missing, expired, already-consumed, or simply
	// wrong code. Verify reports all four the same way on purpose: a caller
	// must not be able to probe which phone numbers have a code in flight.
	ErrCodeInvalid = errors.New("invalid or expired code")
	// ErrCodeTooManyAttempts means the code is burned; ask for a new one.
	ErrCodeTooManyAttempts = errors.New("too many attempts, request a new code")
)

const (
	// ResendCooldown is how long a phone must wait between sends.
	ResendCooldown = 60 * time.Second
	// MaxPerPhonePerDay caps sends per phone over a rolling 24 hours.
	MaxPerPhonePerDay = 10
	// MaxPerIPPerDay caps sends per IP over a rolling 24 hours.
	MaxPerIPPerDay = 30
	// CaptchaThresholdPerHour is the per-IP hourly send count past which a
	// captcha is required.
	CaptchaThresholdPerHour = 5
	// CodeTTL is how long a delivered code stays valid.
	CodeTTL = 5 * time.Minute
	// MaxVerifyAttempts is how many wrong guesses a code survives.
	MaxVerifyAttempts = 5

	codeLength = 6
)

var phonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

// deliverSMS indirects the tencentcloud package so tests can stub the SDK call.
var deliverSMS = tencentcloud.SendSms

// Result describes one successful send.
type Result struct {
	// Code is the generated verification code. Hand it to a caller only when
	// Mock is true — it is the credential the whole flow exists to protect.
	Code string
	// Mock reports whether delivery was mocked, i.e. nothing was sent.
	Mock bool
}

// Send validates the phone, enforces the rate limits, generates a code and
// delivers it. It returns the code so a mock-mode API can expose it locally.
func Send(ctx context.Context, phone, ip, captchaID, captchaAnswer string) (Result, error) {
	driver, err := driver()
	if err != nil {
		return Result{}, err
	}

	if !phonePattern.MatchString(phone) {
		return Result{}, ErrInvalidPhone
	}

	limited, err := rateLimited(phone, ip)
	if err != nil {
		return Result{}, err
	}
	if limited {
		return Result{}, ErrRateLimited
	}

	needed, err := captchaNeeded(ip)
	if err != nil {
		return Result{}, err
	}
	if needed {
		if captchaID == "" {
			return Result{}, ErrCaptchaRequired
		}
		if !captcha.Verify(ctx, ip, captchaID, captchaAnswer) {
			return Result{}, ErrCaptchaInvalid
		}
	}

	code, err := randomCode()
	if err != nil {
		return Result{}, err
	}

	now := time.Now()

	// One live code per phone: retire any earlier unconsumed code before
	// issuing a new one, so Verify has exactly one row to compare against and
	// an old code cannot outlive the one that replaced it.
	if err := repo.UpdateWhere[models.SmsVerification](
		sql.H{"consumed": true, "updated_at": now},
		sql.AllOf(sql.Eq("phone", phone), sql.Eq("consumed", false)),
	); err != nil {
		return Result{}, err
	}

	if _, err := repo.CreateFrom[models.SmsVerification](sql.H{
		"phone":      phone,
		"code":       code,
		"ip":         ip,
		"expires_at": now.Add(CodeTTL),
		"created_at": now,
		"updated_at": now,
	}); err != nil {
		return Result{}, err
	}

	return Result{Code: code, Mock: driver == DriverMock}, deliver(ctx, driver, phone, code)
}

// Verify checks a code against the newest unconsumed one for the phone and
// consumes it. It returns nil only on a match; every other outcome is a
// sentinel error.
func Verify(ctx context.Context, phone, code string) error {
	rows, err := repo.FindBy[models.SmsVerification](sql.H{"phone": phone, "consumed": false})
	if err != nil {
		return err
	}

	latest := newest(rows)
	if latest == nil || latest.Expired() {
		return ErrCodeInvalid
	}
	if latest.Attempts >= MaxVerifyAttempts {
		return ErrCodeTooManyAttempts
	}

	now := time.Now()

	if latest.Code != code {
		if err := repo.UpdateByID[models.SmsVerification](latest.ID, sql.H{
			"attempts":   latest.Attempts + 1,
			"updated_at": now,
		}); err != nil {
			return err
		}

		return ErrCodeInvalid
	}

	// Consume conditionally so two concurrent callers cannot both accept the
	// same code: whoever loses the race sees zero affected rows.
	consumed, err := repo.UpdateAffected(repo.CurrentDB(),
		sql.UpdateAll(models.SmsVerification{}, sql.H{"consumed": true, "updated_at": now}).
			Where(sql.AllOf(sql.Eq("id", latest.ID), sql.Eq("consumed", false))))
	if err != nil {
		return err
	}
	if consumed == 0 {
		return ErrCodeInvalid
	}

	return nil
}

func newest(rows []*models.SmsVerification) *models.SmsVerification {
	var latest *models.SmsVerification
	for _, row := range rows {
		if latest == nil || row.ID > latest.ID {
			latest = row
		}
	}

	return latest
}

// driver resolves the delivery channel. An explicit SMS_DRIVER wins; an
// unrecognized value is an error rather than a silent fallback, because the
// fallback would be mock and mock leaks the code.
func driver() (string, error) {
	if value := strings.TrimSpace(os.Getenv("SMS_DRIVER")); value != "" {
		if value != DriverMock && value != DriverTencent {
			return "", fmt.Errorf("unknown SMS_DRIVER %q, want %q or %q", value, DriverMock, DriverTencent)
		}

		return value, nil
	}

	if utils.AppConfig().IsLocal {
		return DriverMock, nil
	}

	return DriverTencent, nil
}

// rateLimited applies the per-phone and per-IP send caps. The counts and the
// insert in Send are separate statements, so concurrent requests can each pass
// the checks and overshoot a cap slightly; these are abuse limits, not
// correctness invariants, and closing the window would cost the query
// builders their dialect neutrality (it needs a per-phone lock).
func rateLimited(phone, ip string) (bool, error) {
	cooldown, err := repo.Join(models.SmsVerification{}).
		Where(sql.AllOf(
			sql.Eq("sms_verifications.phone", phone),
			sql.Gte("sms_verifications.created_at", time.Now().Add(-ResendCooldown)),
		)).
		Count()
	if err != nil {
		return false, err
	}
	if cooldown > 0 {
		return true, nil
	}

	phoneToday, err := repo.Join(models.SmsVerification{}).
		Where(sql.AllOf(
			sql.Eq("sms_verifications.phone", phone),
			sql.Gte("sms_verifications.created_at", time.Now().Add(-24*time.Hour)),
		)).
		Count()
	if err != nil {
		return false, err
	}
	if phoneToday >= MaxPerPhonePerDay {
		return true, nil
	}

	ipToday, err := repo.Join(models.SmsVerification{}).
		Where(sql.AllOf(
			sql.Eq("sms_verifications.ip", ip),
			sql.Gte("sms_verifications.created_at", time.Now().Add(-24*time.Hour)),
		)).
		Count()
	if err != nil {
		return false, err
	}

	return ipToday >= MaxPerIPPerDay, nil
}

func captchaNeeded(ip string) (bool, error) {
	ipHour, err := repo.Join(models.SmsVerification{}).
		Where(sql.AllOf(
			sql.Eq("sms_verifications.ip", ip),
			sql.Gte("sms_verifications.created_at", time.Now().Add(-time.Hour)),
		)).
		Count()
	if err != nil {
		return false, err
	}

	return ipHour >= CaptchaThresholdPerHour, nil
}

func randomCode() (string, error) {
	var b strings.Builder
	for i := 0; i < codeLength; i++ {
		v, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b.WriteByte(byte('0' + v.Int64()))
	}
	return b.String(), nil
}

func deliver(ctx context.Context, driver, phone, code string) error {
	if driver == DriverMock {
		if !utils.AppConfig().IsLocal {
			log.Printf("[smsverify] mock delivery while AIRWAY_ENV=%q: no SMS is being sent", utils.AppConfig().Env)
		}
		log.Printf("[smsverify] mock delivery to %s: %s", phone, code)

		return nil
	}

	templateID := strings.TrimSpace(os.Getenv("TENCENTCLOUD_SMS_TEMPLATE_ID"))
	if templateID == "" {
		return errors.New("TENCENTCLOUD_SMS_TEMPLATE_ID is not configured")
	}

	result, err := deliverSMS(ctx, tencentcloud.SendSmsInput{
		PhoneNumbers:     []string{"+86" + phone},
		TemplateID:       templateID,
		TemplateParamSet: []string{code},
	})
	if err != nil {
		return err
	}
	for _, status := range result.SendStatuses {
		if status.Code != "Ok" {
			return errors.New("sms delivery failed: " + status.Message)
		}
	}

	return nil
}
