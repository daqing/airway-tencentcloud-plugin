package sms_codes_api

import "github.com/daqing/airway/lib/openapi"

func init() {
	openapi.Post("/api/v1/sms_codes", func(o *openapi.Operation) {
		o.Summary("Send an SMS verification code").Tag("sms").
			Description("Sends a 6-digit login code to a Chinese mobile number. " +
				"Rate-limited per phone (60s resend cooldown, 10/day) and per IP (30/day); " +
				"a captcha is required once an IP exceeds 5 sends/hour. " +
				"Errors answer with HTTP 200 and an in-band code: 40001 invalid phone or " +
				"captcha answer, 40301 captcha required (data carries captcha_required " +
				"and captcha_url), 42901 rate limited.")

		o.Body(openapi.Item[SendParams]())

		o.OK(openapi.Obj(map[string]*openapi.Schema{
			"sent":     openapi.Bool(),
			"dev_code": openapi.Str(),
		}))
	})
}
