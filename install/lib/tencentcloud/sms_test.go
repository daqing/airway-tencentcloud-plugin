package tencentcloud

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	smsapi "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

// configuredForTest resets the lazy setup and configures fake credentials
// so SendSms gets past configuration; the SDK call itself is stubbed.
func configuredForTest(t *testing.T) {
	t.Helper()
	resetForTest()
	t.Cleanup(resetForTest)
	setTencentCloudEnv(t, map[string]string{
		"TENCENTCLOUD_SECRET_ID":      "test-secret-id",
		"TENCENTCLOUD_SECRET_KEY":     "test-secret-key",
		"TENCENTCLOUD_SMS_SDK_APP_ID": "1400006666",
	})
}

func stubSendSmsCall(t *testing.T, fn func(ctx context.Context, client *smsapi.Client, req *smsapi.SendSmsRequest) (*smsapi.SendSmsResponse, error)) {
	t.Helper()
	original := sendSmsCall
	sendSmsCall = fn
	t.Cleanup(func() { sendSmsCall = original })
}

func derefStrings(values []*string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v != nil {
			out = append(out, *v)
		} else {
			out = append(out, "")
		}
	}
	return out
}

func TestSendSmsBuildsSDKRequest(t *testing.T) {
	configuredForTest(t)

	var captured *smsapi.SendSmsRequest
	stubSendSmsCall(t, func(_ context.Context, _ *smsapi.Client, req *smsapi.SendSmsRequest) (*smsapi.SendSmsResponse, error) {
		captured = req
		return &smsapi.SendSmsResponse{Response: &smsapi.SendSmsResponseParams{
			RequestId: common.StringPtr("req-1"),
		}}, nil
	})

	result, err := SendSms(context.Background(), SendSmsInput{
		PhoneNumbers:     []string{"+8613800138000", "+8613800138001"},
		TemplateID:       "1234567",
		TemplateParamSet: []string{"654321"},
		SessionContext:   "order-42",
	})
	if err != nil {
		t.Fatalf("SendSms: %v", err)
	}
	if result.RequestID != "req-1" {
		t.Fatalf("RequestID = %q, want %q", result.RequestID, "req-1")
	}

	if got := *captured.SmsSdkAppId; got != "1400006666" {
		t.Fatalf("SmsSdkAppId = %q, want 1400006666", got)
	}
	if got := *captured.TemplateId; got != "1234567" {
		t.Fatalf("TemplateId = %q, want 1234567", got)
	}
	if got := derefStrings(captured.PhoneNumberSet); !reflect.DeepEqual(got, []string{"+8613800138000", "+8613800138001"}) {
		t.Fatalf("PhoneNumberSet = %#v", got)
	}
	if got := derefStrings(captured.TemplateParamSet); !reflect.DeepEqual(got, []string{"654321"}) {
		t.Fatalf("TemplateParamSet = %#v", got)
	}
	if got := *captured.SessionContext; got != "order-42" {
		t.Fatalf("SessionContext = %q, want order-42", got)
	}
}

func TestSendSmsSignNamePrecedence(t *testing.T) {
	tests := []struct {
		name        string
		envSignName string
		inputSign   string
		want        string // empty means the request must omit SignName
	}{
		{name: "request overrides config", envSignName: "ConfiguredSign", inputSign: "RequestSign", want: "RequestSign"},
		{name: "falls back to config", envSignName: "ConfiguredSign", want: "ConfiguredSign"},
		{name: "omitted when neither set", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetForTest()
			t.Cleanup(resetForTest)
			setTencentCloudEnv(t, map[string]string{
				"TENCENTCLOUD_SECRET_ID":      "test-secret-id",
				"TENCENTCLOUD_SECRET_KEY":     "test-secret-key",
				"TENCENTCLOUD_SMS_SDK_APP_ID": "1400006666",
				"TENCENTCLOUD_SMS_SIGN_NAME":  test.envSignName,
			})

			var captured *smsapi.SendSmsRequest
			stubSendSmsCall(t, func(_ context.Context, _ *smsapi.Client, req *smsapi.SendSmsRequest) (*smsapi.SendSmsResponse, error) {
				captured = req
				return &smsapi.SendSmsResponse{Response: &smsapi.SendSmsResponseParams{}}, nil
			})

			_, err := SendSms(context.Background(), SendSmsInput{
				PhoneNumbers: []string{"+8613800138000"},
				TemplateID:   "1234567",
				SignName:     test.inputSign,
			})
			if err != nil {
				t.Fatalf("SendSms: %v", err)
			}

			if test.want == "" {
				if captured.SignName != nil {
					t.Fatalf("SignName = %q, want omitted", *captured.SignName)
				}
				return
			}
			if captured.SignName == nil || *captured.SignName != test.want {
				t.Fatalf("SignName = %#v, want %q", captured.SignName, test.want)
			}
		})
	}
}

func TestSendSmsOmitsEmptyOptionalFields(t *testing.T) {
	configuredForTest(t)

	var captured *smsapi.SendSmsRequest
	stubSendSmsCall(t, func(_ context.Context, _ *smsapi.Client, req *smsapi.SendSmsRequest) (*smsapi.SendSmsResponse, error) {
		captured = req
		return &smsapi.SendSmsResponse{Response: &smsapi.SendSmsResponseParams{}}, nil
	})

	if _, err := SendSms(context.Background(), SendSmsInput{
		PhoneNumbers: []string{"+8613800138000"},
		TemplateID:   "1234567",
	}); err != nil {
		t.Fatalf("SendSms: %v", err)
	}

	if captured.TemplateParamSet != nil {
		t.Fatalf("TemplateParamSet = %#v, want nil", captured.TemplateParamSet)
	}
	if captured.SessionContext != nil {
		t.Fatalf("SessionContext = %#v, want nil", captured.SessionContext)
	}
}

func TestSendSmsMapsResponse(t *testing.T) {
	result := newSendSmsResult(&smsapi.SendSmsResponseParams{
		RequestId: common.StringPtr("req-42"),
		SendStatusSet: []*smsapi.SendStatus{
			{
				SerialNo:       common.StringPtr("sn-1"),
				PhoneNumber:    common.StringPtr("+8613800138000"),
				Fee:            common.Uint64Ptr(1),
				SessionContext: common.StringPtr("order-42"),
				Code:           common.StringPtr("Ok"),
				Message:        common.StringPtr("send success"),
			},
			{
				// nil fields must map to zero values, not panic
				Code: common.StringPtr("LimitExceeded.PhoneNumberDailyLimit"),
			},
		},
	})

	if result.RequestID != "req-42" {
		t.Fatalf("RequestID = %q, want req-42", result.RequestID)
	}
	if len(result.SendStatuses) != 2 {
		t.Fatalf("len(SendStatuses) = %d, want 2", len(result.SendStatuses))
	}

	first := result.SendStatuses[0]
	if first.SerialNo != "sn-1" || first.PhoneNumber != "+8613800138000" || first.Fee != 1 ||
		first.SessionContext != "order-42" || first.Code != "Ok" || first.Message != "send success" {
		t.Fatalf("unexpected first status: %#v", first)
	}

	second := result.SendStatuses[1]
	if second.Code != "LimitExceeded.PhoneNumberDailyLimit" || second.Fee != 0 || second.PhoneNumber != "" {
		t.Fatalf("unexpected second status: %#v", second)
	}
}

func TestSendSmsFailsWithoutConfig(t *testing.T) {
	resetForTest()
	t.Cleanup(resetForTest)
	setTencentCloudEnv(t, map[string]string{})

	_, err := SendSms(context.Background(), SendSmsInput{
		PhoneNumbers: []string{"+8613800138000"},
		TemplateID:   "1234567",
	})
	if err == nil {
		t.Fatal("SendSms succeeded without configuration")
	}
	for _, name := range []string{"TENCENTCLOUD_SECRET_ID", "TENCENTCLOUD_SECRET_KEY", "TENCENTCLOUD_SMS_SDK_APP_ID"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("error %q does not mention %s", err, name)
		}
	}
}

func TestSendSmsPropagatesSDKError(t *testing.T) {
	configuredForTest(t)

	sdkErr := errors.New("[TencentCloudSDKError] Code=AuthFailure.SecretIdNotFound")
	stubSendSmsCall(t, func(_ context.Context, _ *smsapi.Client, _ *smsapi.SendSmsRequest) (*smsapi.SendSmsResponse, error) {
		return nil, sdkErr
	})

	result, err := SendSms(context.Background(), SendSmsInput{
		PhoneNumbers: []string{"+8613800138000"},
		TemplateID:   "1234567",
	})
	if !errors.Is(err, sdkErr) {
		t.Fatalf("err = %v, want the SDK error", err)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
}
