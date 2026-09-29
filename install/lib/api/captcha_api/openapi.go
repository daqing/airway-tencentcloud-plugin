package captcha_api

import "github.com/daqing/airway/lib/openapi"

func init() {
	openapi.Get("/api/v1/captcha", func(o *openapi.Operation) {
		o.Summary("Issue an image captcha").Tag("sms").
			Description("Returns a 4-digit image captcha bound to the caller's IP and valid for " +
				"10 minutes. The send endpoint asks for one once an IP exceeds 5 sends/hour; " +
				"submit it back as captcha_id and captcha_answer. A captcha is single-use — " +
				"a wrong answer consumes it too. Capped at 30 per IP per hour; past that the " +
				"response is HTTP 200 with in-band code 42901.")

		o.OK(openapi.Obj(map[string]*openapi.Schema{
			"id":    openapi.Str(),
			"image": openapi.Str(),
		}))
	})
}
