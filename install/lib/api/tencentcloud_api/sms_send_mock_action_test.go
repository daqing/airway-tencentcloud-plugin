package tencentcloud_api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/daqing/airway-tencentcloud-plugin/install/lib/tencentcloud"
)

func stubRandomCode(t *testing.T, code string) {
	t.Helper()
	original := randomCode
	randomCode = func() (string, error) { return code, nil }
	t.Cleanup(func() { randomCode = original })
}

func postSmsMock(r *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tencentcloud/sms/send/mock", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSmsSendMockValidatesInput(t *testing.T) {
	overLimit := make([]string, maxPhoneNumbers+1)
	for i := range overLimit {
		overLimit[i] = fmt.Sprintf("+8613800138%04d", i)
	}
	overLimitBody := `{"phone_numbers":["` + strings.Join(overLimit, `","`) + `"],"template_id":"1234567"}`

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "invalid json", body: `not json`, want: "invalid request body"},
		{name: "empty phone numbers", body: `{"phone_numbers":[],"template_id":"1234567"}`, want: "phone_numbers must not be empty"},
		{name: "over 200 phone numbers", body: overLimitBody, want: fmt.Sprintf("at most %d", maxPhoneNumbers)},
		{name: "missing template id", body: `{"phone_numbers":["+8613800138000"]}`, want: "template_id must not be empty"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stubSendSms(t, func(context.Context, tencentcloud.SendSmsInput) (*tencentcloud.SendSmsResult, error) {
				t.Fatal("mock send must not reach the real send chain")
				return nil, nil
			})

			w := postSmsMock(setupSmsTestRouter(t), test.body)
			code, message, _ := decodeEnvelope(t, w)

			if code == 0 {
				t.Fatalf("code = 0, body = %s", w.Body.String())
			}
			if !strings.Contains(message, test.want) {
				t.Fatalf("message = %q, want it to contain %q", message, test.want)
			}
		})
	}
}

// TestSmsSendMockMatchesRealSendContract pins the mock response to the
// exact fields and types the real endpoint renders — no additions; the
// verification code travels as request_id and each status's serial_no.
func TestSmsSendMockMatchesRealSendContract(t *testing.T) {
	stubSendSms(t, func(context.Context, tencentcloud.SendSmsInput) (*tencentcloud.SendSmsResult, error) {
		t.Fatal("mock send must not reach the real send chain")
		return nil, nil
	})
	stubRandomCode(t, "654321")

	w := postSmsMock(setupSmsTestRouter(t), `{
		"phone_numbers": ["+8613800138000", "+8613800138001"],
		"template_id": "1234567",
		"sign_name": "Airway",
		"template_param_set": ["654321"],
		"session_context": "order-42"
	}`)
	code, message, data := decodeEnvelope(t, w)

	if code != 0 || message != "" {
		t.Fatalf("unexpected envelope: code = %d, message = %q, body = %s", code, message, w.Body.String())
	}

	want := map[string]any{
		"request_id": "654321",
		"send_status_set": []any{
			map[string]any{
				"serial_no":       "654321",
				"phone_number":    "+8613800138000",
				"fee":             float64(1),
				"session_context": "order-42",
				"code":            "Ok",
				"message":         "mock send success",
			},
			map[string]any{
				"serial_no":       "654321",
				"phone_number":    "+8613800138001",
				"fee":             float64(1),
				"session_context": "order-42",
				"code":            "Ok",
				"message":         "mock send success",
			},
		},
	}
	if !reflect.DeepEqual(data, want) {
		t.Fatalf("data = %#v, want %#v", data, want)
	}
}

func TestSmsSendMockGeneratesSixDigitCode(t *testing.T) {
	sixDigits := regexp.MustCompile(`^\d{6}$`)
	for i := 0; i < 20; i++ {
		w := postSmsMock(setupSmsTestRouter(t), `{"phone_numbers":["+8613800138000"],"template_id":"1234567"}`)
		_, _, data := decodeEnvelope(t, w)

		statuses, ok := data["send_status_set"].([]any)
		if !ok || len(statuses) == 0 {
			t.Fatalf("send_status_set = %#v", data["send_status_set"])
		}
		status := statuses[0].(map[string]any)
		got, _ := status["serial_no"].(string)
		if !sixDigits.MatchString(got) {
			t.Fatalf("serial_no = %q, want 6 digits", got)
		}
	}
}
