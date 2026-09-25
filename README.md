# github.com/daqing/airway-tencentcloud-plugin

An [Airway](https://github.com/daqing/airway) plugin that integrates the
[Tencent Cloud Go SDK](https://github.com/TencentCloud/tencentcloud-sdk-go),
giving host applications ready-made APIs for Tencent Cloud services so each
Airway project doesn't wire the SDK itself. Currently supported: SMS.

## Develop

```bash
go get github.com/daqing/airway@latest
go mod tidy
```

To exercise the API without an Airway host application, run the local dev
server:

```bash
go run ./install/ignore/devserver   # listens on :3000, override with LISTEN
```

```bash
curl -X POST http://localhost:3000/api/v1/tencentcloud/sms/send/mock \
  -H 'Content-Type: application/json' \
  -d '{"phone_numbers":["+8618501234444"],"template_id":"1234567"}'
```

The mock endpoint needs no configuration; the real `/sms/send` reads the
`TENCENTCLOUD_*` environment variables (see Configuration) on first use.

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

## SMS API

Routes are mounted at `/api/v1/tencentcloud`.

### Send SMS

`POST /api/v1/tencentcloud/sms/send`

```bash
curl -X POST http://localhost:3000/api/v1/tencentcloud/sms/send \
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

> This endpoint hands a verification code to anyone who calls it. Never
> expose it outside local development.

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

## Install layout

Content reaches the host in two ways: code under `install/lib/` is compiled
into the host binary through this import, while the files under
`install/host/` and `install/deps/` are copied into the host project by
`plugin:install` — deploy configs, companion services, SQL migrations, the
things tools outside the Go build read from disk. `install/ignore/` and
anything matching the root `.gitignore` never leave the plugin checkout.

To ship SQL migrations, add `<version>_<name>.up.sql` / `.down.sql` files
under `install/host/db/migrate/` and expose them with
`plugin.MigrationProvider`.

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
