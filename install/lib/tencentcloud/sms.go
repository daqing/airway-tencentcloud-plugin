package tencentcloud

import (
	"context"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	smsapi "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

// SendSmsInput is the normalized send request. PhoneNumbers uses E.164
// (+8618501234444); a bare 11-digit domestic number is also accepted by
// Tencent Cloud and prefixed with +86 server-side.
type SendSmsInput struct {
	PhoneNumbers     []string
	TemplateID       string
	SignName         string
	TemplateParamSet []string
	SessionContext   string
}

// SendStatus is the delivery result for one phone number; Code "Ok" means
// the message was accepted.
type SendStatus struct {
	SerialNo       string `json:"serial_no"`
	PhoneNumber    string `json:"phone_number"`
	Fee            uint64 `json:"fee"`
	SessionContext string `json:"session_context,omitempty"`
	Code           string `json:"code"`
	Message        string `json:"message"`
}

// SendSmsResult carries the per-number statuses Tencent Cloud returned.
type SendSmsResult struct {
	RequestID    string       `json:"request_id"`
	SendStatuses []SendStatus `json:"send_status_set"`
}

// sendSmsCall is the SDK boundary; tests replace it to avoid real network
// calls.
var sendSmsCall = func(ctx context.Context, client *smsapi.Client, req *smsapi.SendSmsRequest) (*smsapi.SendSmsResponse, error) {
	return client.SendSmsWithContext(ctx, req)
}

// SendSms calls Tencent Cloud SendSms. A nil error means the request went
// through; individual numbers can still fail — check each SendStatus.Code.
func SendSms(ctx context.Context, in SendSmsInput) (*SendSmsResult, error) {
	setup()
	if setupErr != nil {
		return nil, setupErr
	}

	req := smsapi.NewSendSmsRequest()
	req.SmsSdkAppId = common.StringPtr(cfg.SmsSdkAppID)
	req.TemplateId = common.StringPtr(in.TemplateID)
	req.PhoneNumberSet = common.StringPtrs(in.PhoneNumbers)

	if in.SignName != "" {
		req.SignName = common.StringPtr(in.SignName)
	} else if cfg.SignName != "" {
		req.SignName = common.StringPtr(cfg.SignName)
	}
	if len(in.TemplateParamSet) > 0 {
		req.TemplateParamSet = common.StringPtrs(in.TemplateParamSet)
	}
	if in.SessionContext != "" {
		req.SessionContext = common.StringPtr(in.SessionContext)
	}

	resp, err := sendSmsCall(ctx, smsClient, req)
	if err != nil {
		return nil, err
	}

	return newSendSmsResult(resp.Response), nil
}

func newSendSmsResult(resp *smsapi.SendSmsResponseParams) *SendSmsResult {
	result := &SendSmsResult{
		RequestID:    stringValue(resp.RequestId),
		SendStatuses: make([]SendStatus, 0, len(resp.SendStatusSet)),
	}

	for _, s := range resp.SendStatusSet {
		result.SendStatuses = append(result.SendStatuses, SendStatus{
			SerialNo:       stringValue(s.SerialNo),
			PhoneNumber:    stringValue(s.PhoneNumber),
			Fee:            uint64Value(s.Fee),
			SessionContext: stringValue(s.SessionContext),
			Code:           stringValue(s.Code),
			Message:        stringValue(s.Message),
		})
	}

	return result
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func uint64Value(v *uint64) uint64 {
	if v == nil {
		return 0
	}
	return *v
}
