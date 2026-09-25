package tencentcloud_api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/daqing/airway-tencentcloud-plugin/install/lib/tencentcloud"
)

func stubSendSms(t *testing.T, fn func(ctx context.Context, in tencentcloud.SendSmsInput) (*tencentcloud.SendSmsResult, error)) {
	t.Helper()
	original := sendSms
	sendSms = fn
	t.Cleanup(func() { sendSms = original })
}

func setupSmsTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Routes(r.Group("/api/v1/tencentcloud"))
	return r
}

func postSms(r *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tencentcloud/sms/send", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeEnvelope(t *testing.T, w *httptest.ResponseRecorder) (code int, message string, data map[string]any) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body struct {
		Code    int            `json:"code"`
		Data    map[string]any `json:"data"`
		Message string         `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body.Code, body.Message, body.Data
}

func TestSmsSendValidatesInput(t *testing.T) {
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
				t.Fatal("service must not be called for invalid input")
				return nil, nil
			})

			w := postSms(setupSmsTestRouter(t), test.body)
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

func TestSmsSendPassesRequestToService(t *testing.T) {
	var captured tencentcloud.SendSmsInput
	stubSendSms(t, func(_ context.Context, in tencentcloud.SendSmsInput) (*tencentcloud.SendSmsResult, error) {
		captured = in
		return &tencentcloud.SendSmsResult{
			RequestID: "req-42",
			SendStatuses: []tencentcloud.SendStatus{{
				SerialNo:    "sn-1",
				PhoneNumber: "+8613800138000",
				Fee:         1,
				Code:        "Ok",
				Message:     "send success",
			}},
		}, nil
	})

	w := postSms(setupSmsTestRouter(t), `{
		"phone_numbers": ["+8613800138000"],
		"template_id": "1234567",
		"sign_name": "Airway",
		"template_param_set": ["654321"],
		"session_context": "order-42"
	}`)
	code, message, data := decodeEnvelope(t, w)

	if code != 0 || message != "" {
		t.Fatalf("unexpected envelope: code = %d, message = %q, body = %s", code, message, w.Body.String())
	}

	want := tencentcloud.SendSmsInput{
		PhoneNumbers:     []string{"+8613800138000"},
		TemplateID:       "1234567",
		SignName:         "Airway",
		TemplateParamSet: []string{"654321"},
		SessionContext:   "order-42",
	}
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("service input = %#v, want %#v", captured, want)
	}

	if data["request_id"] != "req-42" {
		t.Fatalf("request_id = %#v", data["request_id"])
	}
	statuses, ok := data["send_status_set"].([]any)
	if !ok || len(statuses) != 1 {
		t.Fatalf("send_status_set = %#v", data["send_status_set"])
	}
	status := statuses[0].(map[string]any)
	if status["code"] != "Ok" || status["phone_number"] != "+8613800138000" || status["serial_no"] != "sn-1" {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestSmsSendRendersServiceError(t *testing.T) {
	stubSendSms(t, func(context.Context, tencentcloud.SendSmsInput) (*tencentcloud.SendSmsResult, error) {
		return nil, errors.New("[TencentCloudSDKError] Code=AuthFailure.SecretIdNotFound")
	})

	w := postSms(setupSmsTestRouter(t), `{"phone_numbers":["+8613800138000"],"template_id":"1234567"}`)
	code, message, data := decodeEnvelope(t, w)

	if code == 0 {
		t.Fatalf("code = 0, body = %s", w.Body.String())
	}
	if !strings.Contains(message, "AuthFailure.SecretIdNotFound") {
		t.Fatalf("message = %q", message)
	}
	if data != nil {
		t.Fatalf("data = %#v, want nil", data)
	}
}
