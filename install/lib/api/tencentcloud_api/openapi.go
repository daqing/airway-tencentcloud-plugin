package tencentcloud_api

import (
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/tencentcloud"
	"github.com/daqing/airway/lib/openapi"
)

func init() {
	openapi.Post(
		"/api/v1/tencentcloud/sms/send",
		func(o *openapi.Operation) {
			o.Summary("Send SMS through Tencent Cloud").Tag("sms").
				Body(openapi.Item[SmsSendRequest]()).
				OK(openapi.Item[tencentcloud.SendSmsResult]())
		})

	openapi.Post(
		"/api/v1/tencentcloud/sms/send/mock",
		func(o *openapi.Operation) {
			o.Summary("Mock API for sending SMS").Tag("sms").
				Body(openapi.Item[SmsSendRequest]()).
				OK(openapi.Item[tencentcloud.SendSmsResult]())
		})
}
