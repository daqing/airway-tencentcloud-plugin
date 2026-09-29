# AGENTS.md

Airway plugins that integrate the Tencent Cloud Go SDK — giving host
applications APIs for Tencent Cloud services (SMS today) and a complete phone
+ verification code login flow. Go module
`github.com/daqing/airway-tencentcloud-plugin`, built against the
[`airway`](https://github.com/daqing/airway) host framework (gin-based).

## Layout

The module holds **two plugins**. One blank import registers both, because
`plugin.Register` runs in one `init()`; `plugin:install` resolves a plugin
from the module path, so it always installs under the name `tencentcloud`.
There is no file channel for the schema: the tables `smsverify` uses are Go
migrations compiled in (see Migrations).

- `plugin.go` — both plugin entry points.
  - `Plugin` — `Name()` `tencentcloud`, `MountPath()` `/api/v1/tencentcloud`,
    delegates to `install/lib/api/tencentcloud_api.Routes`, which mounts
    nothing outside local mode (see Conventions).
  - `SmsVerifyPlugin` — `Name()` `smsverify`, `MountPath()` `/api/v1`. Mounts
    `sms_codes_api` and `captcha_api` **unconditionally**: those endpoints
    exist for clients to call, and rate limits rather than an auth gate are
    what protects them. It claims the shared `/api/v1` prefix, so a host route
    or another plugin at `/api/v1/sms_codes` or `/api/v1/captcha` collides at
    boot.
- `install/lib/` — Go code compiled into the host binary through the host's
  blank import. SDK wrappers live in `install/lib/tencentcloud/`
  (config + cached client), HTTP handlers in
  `install/lib/api/<feature>_api/`, data models in `install/lib/models/`.
  `install/lib/smsverify/` is the send/verify/rate-limit service and
  `install/lib/captcha/` the image captcha (standard library only: `image` +
  `image/png`, no fonts, no third-party packages).
  `install/lib/migrations/` is the schema (see Migrations).
- `install/deps/tencentcloud/` — companion services / deploy configs merged
  into the host project's `deps/` directory (existing files are kept). This
  module has no `install/host/` tree: everything a host needs is either
  compiled in or, here, merged into `deps/`.
- `install/ignore/` — anything here never reaches a host project; the local
  dev server lives at `install/ignore/devserver/` and the shared in-memory
  SQLite database at `install/ignore/testdb/`. `ignore/` directories inside
  `install/deps/` are skipped too, as are files matching the root
  `.gitignore`.

## Commands

```bash
go get github.com/daqing/airway@latest && go mod tidy  # sync with host framework
go build ./...                                          # compile
go vet ./...                                            # lint
go test ./...                                           # unit tests (stdlib testing, no assertion libs)
go run ./install/ignore/devserver                       # serve the API without an airway host
                                                        # (127.0.0.1:3000, override with LISTEN)
                                                        # for curl testing, on in-memory SQLite
                                                        # (DB_DSN points it at PostgreSQL/MySQL);
                                                        # AIRWAY_ENV=local selects the mock SMS driver
```

Installing into a host application (run from the host project, not here):

```bash
go run . plugin:install github.com/daqing/airway-tencentcloud-plugin
```

## Conventions

- API pattern: one package per feature area under
  `install/lib/api/<feature>_api/`, exposing `Routes(r *gin.RouterGroup)`
  plus one `<name>_action.go` file per action. The router group is already
  mounted at the plugin's `MountPath()`.
- HTTP surface: the host mounts plugins into its public router with no
  middleware hook, so a plugin decides for itself what is safe to expose.
  `tencentcloud` answers that by registering nothing outside local mode
  (`AIRWAY_ENV=local`, via `utils.AppConfig().IsLocal`); endpoints that only
  suit local debugging go in its exported `DebugRoutes`, which callers mount
  themselves (the devserver does), and their `openapi.Post` declarations sit
  next to the registration behind a `sync.Once`: `openapi.Post` panics on a
  duplicate, and declaring a route that is never mounted warns on every host's
  `airway openapi:generate`. `smsverify` answers it by mounting always — its
  routes are unconditionally reached, so their declarations live in plain
  `init()` and need no guard.
- Responses use the framework envelope (`{code, data, message}`, always HTTP
  200) via `github.com/daqing/airway/lib/render`. Where a body-level error
  code has to carry a `data` payload, `sms_codes_api` wraps `c.JSON` in a
  local `fail` helper, because `render.ErrorCodeMsg` hard-codes `data: null`.
- Migrations: Go, not SQL. `install/lib/migrations` registers one
  `schema.RegisterChange` per table in `init()`, and `plugin.go` blank-imports
  the package so every host binary carries the definitions — `plugin:install`
  copies nothing, and the host's own `go run . db:migrate` creates the tables
  from them. `lib/migrate` compiles the ops per dialect, so PostgreSQL, MySQL
  and SQLite all get correct DDL, which the SQL files this replaced did not
  (they were PostgreSQL-only). Two things the DSL decides: the versions are the
  ones those SQL files used (`20260929153501`, `20260929153502`), so a host that
  already applied them skips these rather than re-creating the tables; and the
  down migrations are derived by reversing the up ops, so anything that cannot
  be reversed needs `m.Reversible` or an explicit `schema.Register`. The DSL's
  only primary key builder is `Table.ID()`, which is autoincrement, so a table
  whose public identifier is a random string keeps them apart: `captchas` has
  an `ID()` key of its own and a unique `token VARCHAR(32)` column, and only the
  token is ever handed to a client.
- Nested-module gotcha: any `go.mod` under `install/deps/` must be shipped as
  `go.mod.templ` (installed as `go.mod`) — Go module zips drop nested modules.
  A real `go.mod` may sit beside its `.templ` for local builds; `plugin:install`
  fails if the two drift apart.
- Tencent Cloud SDK: wrappers stay in `install/lib/tencentcloud/` and return
  plugin-owned, snake_case JSON structs; the `*_api` layer never imports SDK
  packages. The SDK client is lazy (`sync.Once`): config errors surface on
  first API call via `render.Error`, never at host boot, so unconfigured hosts
  still start. Required env: `TENCENTCLOUD_SECRET_ID`,
  `TENCENTCLOUD_SECRET_KEY`, `TENCENTCLOUD_SMS_SDK_APP_ID`; optional:
  `TENCENTCLOUD_SMS_SIGN_NAME`, `TENCENTCLOUD_REGION` (default
  `ap-guangzhou`). A zero `code` in the response means Tencent Cloud accepted
  the request — per-number results are in `send_status_set` with `code: "Ok"`.
- Phone verification: `install/lib/smsverify` owns the codes table and every
  rate limit on it (per-phone cooldown and daily cap, per-IP daily cap and
  captcha threshold), `install/lib/captcha` the captchas table plus that
  endpoint's own per-IP hourly issuance cap — the captcha route is public and
  unauthenticated too, so it needs a limit of its own rather than only
  appearing in one. `SMS_DRIVER` picks mock or tencent; unset, it follows the
  environment — mock in local mode, tencent everywhere else — so production
  cannot silently fall back to a driver that hands the code back. The COUNTs
  and the following INSERT are separate statements, so concurrent requests can
  overshoot a cap; the limits are abuse bounds, not correctness invariants, and
  closing the window costs the query builders their dialect neutrality.
- `install/lib/models/registry.go` exposes models to the host REPL. Keep the
  keys prefixed (`SmsCaptcha`, not `Captcha`): `plugin.REPLNamespaces` errors
  on a collision with a host model, and the host then drops *every* plugin's
  REPL models, not just this one's.
- Testing: stdlib `testing` with `t.Setenv` + table-driven subtests; no
  assertion libraries, no `t.Parallel` (tests mutate package env state).
  Databases come from `install/ignore/testdb` — an in-memory SQLite database
  with the module's real migrations applied by `lib/migrate`, so no Postgres
  and no Docker, and no schema to keep in step by hand. Point the dev server at
  a real server with `DB_DSN` when a change has to be checked on another
  dialect. External boundaries are stubbed,
  never hit for real: `tencentcloud.sendSmsCall` (in-package) and
  `tencentcloud_api.sendSms` replace the call chain, `resetForTest` re-arms
  the lazy `sync.Once` setup between configurations, and `smsverify.deliverSMS`
  stands in for the SDK in delivery tests.

Before changing install/mount behavior, read the plugin guide:
https://github.com/daqing/airway/blob/main/docs/plugin.md
