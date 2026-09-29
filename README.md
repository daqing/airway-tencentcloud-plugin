# github.com/daqing/airway-tencentcloud-plugin

An [Airway](https://github.com/daqing/airway) plugin that integrates the
[Tencent Cloud Go SDK](https://github.com/TencentCloud/tencentcloud-sdk-go),
giving host applications ready-made APIs for Tencent Cloud services so each
Airway project doesn't wire the SDK itself. Currently supported: SMS.

The same module ships a second plugin, `smsverify`, with the whole phone +
verification code flow — issuing, rate limiting, image captchas, delivery and
checking — so a host does not write that again either.

## Develop

```bash
go get github.com/daqing/airway@latest
go mod tidy
```

To exercise the API without an Airway host application, run the local dev
server:

```bash
go run ./install/ignore/devserver   # listens on 127.0.0.1:3000, override with LISTEN
```

```bash
curl -X POST http://127.0.0.1:3000/api/v1/tencentcloud/sms/send/mock \
  -H 'Content-Type: application/json' \
  -d '{"phone_numbers":["+8618501234444"],"template_id":"1234567"}'
```

The dev server registers every endpoint unconditionally and serves them from
an in-memory SQLite database, so the phone verification flow works end to end:

```bash
curl -s localhost:3000/api/v1/captcha
curl -sX POST localhost:3000/api/v1/sms_codes \
  -H 'Content-Type: application/json' -d '{"phone":"18501234444"}'
```

Start it with `AIRWAY_ENV=local` to get the mock SMS driver, which hands the
code back as `dev_code` instead of sending it. The mock tencentcloud endpoint
needs no configuration either; the real `/sms/send` reads the `TENCENTCLOUD_*`
environment variables (see Configuration) on first use.

## Use in a host application

```bash
go get github.com/daqing/airway-tencentcloud-plugin
```

Then enable it with a blank import in the host's `plugins.go`:

```go
import (
	_ "github.com/daqing/airway-tencentcloud-plugin"
)
```

The module holds two plugins and this one import registers both:
`tencentcloud` (the debug endpoints, local mode only) and `smsverify` (the
public phone verification endpoints). `plugin:install` resolves a plugin from
the module path, so it always names the install `tencentcloud` and installs
the module's migrations — the tables both plugins use.

The import is also how the host gets the plugins' Go API — the code is
compiled into the host binary, so calls stay in-process and no HTTP hop is
involved:

```go
import "github.com/daqing/airway-tencentcloud-plugin/install/lib/tencentcloud"

result, err := tencentcloud.SendSms(ctx, tencentcloud.SendSmsInput{
	PhoneNumbers:     []string{"+8618501234444"},
	TemplateID:       "1234567",
	TemplateParamSet: []string{"654321"},
})
if err != nil {
	// the request never reached Tencent Cloud
}

for _, status := range result.SendStatuses {
	if status.Code != "Ok" {
		// this number was rejected; status.Message says why
	}
}
```

A nil `err` only means Tencent Cloud accepted the request — each number can
still fail, so check every `SendStatus.Code`, where `"Ok"` means that number
was accepted. The `install/lib/tencentcloud` package returns plugin-owned
types and never leaks SDK types, so the host does not depend on the Tencent
Cloud SDK itself.

## Configuration

The plugin reads its configuration from environment variables on first use
(set them in the host's `.env`):

| Variable | Required | Description |
| --- | --- | --- |
| `TENCENTCLOUD_SECRET_ID` | yes | Tencent Cloud API secret ID |
| `TENCENTCLOUD_SECRET_KEY` | yes | Tencent Cloud API secret key |
| `TENCENTCLOUD_SMS_SDK_APP_ID` | yes | SMS SdkAppId from the [SMS console](https://console.cloud.tencent.com/smsv2/app-manage), e.g. `1400006666` |
| `TENCENTCLOUD_SMS_SIGN_NAME` | for domestic SMS | Default signature content (not the signature ID); can be overridden per request |
| `TENCENTCLOUD_REGION` | no | SDK region, defaults to `ap-guangzhou` |

Missing variables are reported on the first API call, not at boot, so hosts
that haven't configured the plugin yet still start normally.

## HTTP endpoints (local development only)

`Routes` mounts the two endpoints below at `/api/v1/tencentcloud` **only when
the host runs in local mode** (`AIRWAY_ENV=local`). A production host serves
neither of them and calls the Go API instead. Neither endpoint authenticates
its caller: `/sms/send` spends real SMS quota for whoever can reach it, and
`/sms/send/mock` returns the verification code in the response body.

To serve them outside local mode, mount them yourself behind your own
authentication — never on a public router:

```go
tencentcloud_api.DebugRoutes(r.Group("/api/v1/tencentcloud", requireInternalAuth))
```

Because the routes follow the mode, `airway openapi:generate` documents them
only when it runs with `AIRWAY_ENV=local`; the generated document always
matches what that binary actually serves.

### Send SMS

`POST /api/v1/tencentcloud/sms/send`

```bash
curl -X POST http://127.0.0.1:3000/api/v1/tencentcloud/sms/send \
  -H 'Content-Type: application/json' \
  -d '{
        "phone_numbers": ["+8618501234444"],
        "template_id": "1234567",
        "template_param_set": ["654321"]
      }'
```

| Field | Required | Description |
| --- | --- | --- |
| `phone_numbers` | yes | E.164 numbers (`+8618501234444`); a bare 11-digit domestic number also works. At most 200 per request, all domestic or all international |
| `template_id` | yes | An approved template ID |
| `template_param_set` | no | Template variables, in the order the template defines them |
| `sign_name` | no | Overrides `TENCENTCLOUD_SMS_SIGN_NAME` for this request |
| `session_context` | no | User context echoed back in the status (max 512 bytes) |

Response (a zero `code` only means Tencent Cloud accepted the request; check
each number's status):

```json
{
  "code": 0,
  "data": {
    "request_id": "a0d44e6f-606b-4572-89f3-209ea1a6cf5a",
    "send_status_set": [
      {
        "serial_no": "5000:10933456789012345678901234567",
        "phone_number": "+8618501234444",
        "fee": 1,
        "code": "Ok",
        "message": "send success"
      }
    ]
  },
  "message": ""
}
```

### Mock send (local debugging)

`POST /api/v1/tencentcloud/sms/send/mock`

Accepts the same request fields, applies the same validation, and renders
the exact same response as [Send SMS](#send-sms) — same fields, same
types, nothing added or removed — so a client can switch between the two
endpoints without any code changes. The only difference: nothing reaches
Tencent Cloud (no credentials or SMS configuration needed), and the
generated verification code comes back as the value of `request_id` and
each status's `serial_no` (a six-digit string, leading zeros preserved).

> This endpoint hands a verification code to anyone who calls it, which is
> why it is registered only in local mode. Never mount it anywhere else.

```json
{
  "code": 0,
  "data": {
    "request_id": "654321",
    "send_status_set": [
      {
        "serial_no": "654321",
        "phone_number": "+8618501234444",
        "fee": 1,
        "session_context": "order-42",
        "code": "Ok",
        "message": "mock send success"
      }
    ]
  },
  "message": ""
}
```

## Phone verification (`smsverify`)

The module's second plugin implements what a login or sign-up screen needs: it
issues a verification code over SMS, caps how often a phone number and an IP
may ask for one, demands an image captcha once an IP gets greedy, and checks
the code back.

Unlike the tencentcloud debug routes, **these two endpoints are mounted in
every environment** — `AIRWAY_ENV` does not change them. Clients call them
directly, and the rate limits below, not authentication, are what protects
them. The plugin mounts at `/api/v1`, claiming that shared prefix: a host or
another plugin that registers `/api/v1/sms_codes` or `/api/v1/captcha` will
collide with it at boot.

### Send a code

`POST /api/v1/sms_codes`

```bash
curl -X POST http://127.0.0.1:3000/api/v1/sms_codes \
  -H 'Content-Type: application/json' \
  -d '{"phone":"18501234444"}'
```

| Field | Required | Description |
| --- | --- | --- |
| `phone` | yes | Mainland China mobile number: `1[3-9]` followed by nine digits, no country code |
| `captcha_id` | once a captcha is required | The `id` from [Get a captcha](#get-a-captcha) |
| `captcha_answer` | once a captcha is required | The four digits the image shows |

```json
{"code": 0, "data": {"sent": true}, "message": ""}
```

Under the mock driver the code comes back too, so a client can be tested
locally without an SMS account:

```json
{"code": 0, "data": {"dev_code": "017099", "sent": true}, "message": ""}
```

Every response is HTTP 200 — the outcome is in `code`:

| `code` | Meaning |
| --- | --- |
| `0` | Code sent |
| `40001` | Unreadable body, invalid phone number, or a wrong captcha answer |
| `40301` | A captcha is required; `data` carries `captcha_required: true` and `captcha_url` |
| `42901` | Rate limited |
| `10000` | Anything else — delivery rejected, configuration missing; `message` says what |

### Get a captcha

`GET /api/v1/captcha`

Returns a four-digit captcha bound to the caller's IP, rendered as a base64
PNG so the client needs no second request:

```json
{
  "code": 0,
  "data": {
    "id": "16923e222c57be8d",
    "image": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAJYAAAA0CAIAAADnt1ZQ..."
  },
  "message": ""
}
```

Captchas live for 10 minutes and are single use; a wrong answer consumes one
too, so guessing cannot be spread over many attempts. An IP may ask for 30 per
hour — past that the response is `42901`, and the captcha is neither generated
nor stored.

### Limits

| Limit | Value |
| --- | --- |
| Resend cooldown, per phone | 60 seconds |
| Sends, per phone | 10 per 24 hours |
| Sends, per IP | 30 per 24 hours |
| Captcha required, per IP | past 5 sends per hour |
| Captchas issued, per IP | 30 per hour |
| Code lifetime | 5 minutes |
| Wrong guesses per code | 5 |

The per-IP limits use the client IP from the request, so a host behind a proxy
must call `router.SetTrustedProxies` — gin trusts every proxy by default,
which lets a caller rotate `X-Forwarded-For` and slip past them. The per-phone
limits do not depend on that.

### Go API

A host that wants its own endpoints, or that wants to decide what a verified
number means, calls the package directly. `Send` is what the HTTP handler
wraps; `Verify` deliberately has no endpoint, because issuing a session or
creating the user is the host's decision:

```go
import "github.com/daqing/airway-tencentcloud-plugin/install/lib/smsverify"

result, err := smsverify.Send(ctx, "18501234444", clientIP, captchaID, captchaAnswer)
if errors.Is(err, smsverify.ErrCaptchaRequired) {
	// serve a captcha, then ask again with captcha_id and captcha_answer
}

if err := smsverify.Verify(ctx, "18501234444", code); err != nil {
	// ErrCodeInvalid - missing, expired, consumed, or simply wrong
	// ErrCodeTooManyAttempts - too many guesses; ask for a new code
}
```

`Verify` consumes the code, so it returns nil at most once per code. It
reports "wrong code" and "no code in flight" the same way on purpose: a caller
must not be able to probe which numbers have a code outstanding.

### Delivery

`SMS_DRIVER` selects the channel:

| Value | Effect |
| --- | --- |
| `mock` | Logs the code instead of sending it, and returns it as `dev_code` |
| `tencent` | Sends a real template SMS through Tencent Cloud |

Leave it unset and the driver follows the environment: `mock` when
`AIRWAY_ENV=local`, `tencent` everywhere else — so a production host cannot
fall back to mock by forgetting to set it. Any other value is an error rather
than a fallback.

The `tencent` driver needs `TENCENTCLOUD_SMS_TEMPLATE_ID` (an approved
template taking the code as its single parameter) alongside the
`TENCENTCLOUD_*` variables above, and sends to `+86` plus the number.

> `SMS_DRIVER=mock` outside local mode still sends nothing and still hands the
> code back in the response. Never set it on a production host.

## Install layout

Content reaches the host in two ways: code under `install/lib/` is compiled
into the host binary through this import, while the files under
`install/host/` and `install/deps/` are copied into the host project by
`plugin:install` — deploy configs, companion services, SQL migrations, the
things tools outside the Go build read from disk. `install/ignore/` and
anything matching the root `.gitignore` never leave the plugin checkout.

To ship SQL migrations, add `<version>_<name>.up.sql` / `.down.sql` files
under `install/host/db/migrate/` and expose them with
`plugin.MigrationProvider`. The migrations this module ships — the two tables
behind `smsverify` — are written for PostgreSQL (`BIGSERIAL`, `TIMESTAMPTZ`,
`DEFAULT NOW()`); a host on MySQL or SQLite must translate them before
`db:migrate`, even though the Go code above them stays dialect-neutral.
`plugin:install` skips a migration whose name it already finds in the host's
`db/migrate/`, so installing into a project that already has these tables from
its own migrations is a no-op.

To ship extra project files (companion services, deploy configs, ...), put
them under `install/deps/tencentcloud/` — `plugin:install` merges the whole
tree into the host project's `deps/` directory, skipping files that already
exist. Ship files named `go.mod` as `go.mod.templ` (installed as `go.mod`):
Go module zips drop nested modules, so a real `go.mod` inside
`install/deps/` would never reach the host. Keeping the real `go.mod` beside
its `.templ` for local builds is fine — the install ships the `.templ`
content only, and fails if the two drift apart.

Files that stay in the plugin checkout but must never be installed go under
`install/ignore/`: `plugin:install` only reads `install/host/` and
`install/deps/`, and skips every `ignore/` directory inside them as well, so
anything placed there never reaches the host project. The root `.gitignore`
is honored too: ignored files (`node_modules/`, `.env`, ...) are never
installed into a host project.

See https://github.com/daqing/airway/blob/main/docs/plugin.md for the full
plugin guide.
