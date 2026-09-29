package tencentcloud_api

import (
	"sync"

	"github.com/daqing/airway/lib/openapi"

	"github.com/daqing/airway-tencentcloud-plugin/install/lib/tencentcloud"
)

// declareOnce makes the declarations idempotent: openapi.Post panics on a
// duplicate declaration, and one process can register these routes more than
// once (every test router, or a host that mounts the debug routes twice).
var declareOnce sync.Once

// declareOpenAPI documents the SMS endpoints. It runs alongside the route
// registration rather than from init(), so hosts that never mount the routes
// — every production host — have nothing to declare and `airway
// openapi:generate` reports no dangling declarations.
func declareOpenAPI() {
	declareOnce.Do(func() {
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
	})
}
