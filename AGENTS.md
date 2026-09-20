# AGENTS.md

Airway plugin that integrates Tencent Cloud. Go module
`github.com/daqing/airway-tencentcloud-plugin`, built against the
[`airway`](https://github.com/daqing/airway) host framework (gin-based).

## Layout

- `plugin.go` — plugin entry point. Registers the plugin via `init()`,
  sets `Name()` (`tencentcloud`) and `MountPath()` (`/api/v1/tencentcloud`),
  delegates routing to `install/lib/api/tencentcloud_api.Routes`.
- `install/lib/` — Go code compiled into the host binary through the host's
  blank import. API handlers live in `install/lib/api/tencentcloud_api/`,
  data models in `install/lib/models/`.
- `install/host/` — files copied verbatim into the host project's own tree by
  `plugin:install`. SQL migrations go in `install/host/db/migrate/`.
- `install/deps/tencentcloud/` — companion services / deploy configs merged
  into the host project's `deps/` directory (existing files are kept).
- `install/ignore/` — anything here never reaches a host project. `ignore/`
  directories inside `install/host/` and `install/deps/` are skipped too, as
  are files matching the root `.gitignore`.

## Commands

```bash
go get github.com/daqing/airway@latest && go mod tidy  # sync with host framework
go build ./...                                          # compile
go vet ./...                                            # lint
go test ./...                                           # tests (none yet)
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
- Respond with helpers from `github.com/daqing/airway/lib/render`
  (e.g. `render.OK(c, gin.H{...})`), not raw `c.JSON`.
- Migrations: `<version>_<name>.up.sql` / `.down.sql` pairs under
  `install/host/db/migrate/`, exposed to the host via
  `plugin.MigrationProvider`.
- Nested-module gotcha: any `go.mod` under `install/deps/` must be shipped as
  `go.mod.templ` (installed as `go.mod`) — Go module zips drop nested modules.
  A real `go.mod` may sit beside its `.templ` for local builds; `plugin:install`
  fails if the two drift apart.

Before changing install/mount behavior, read the plugin guide:
https://github.com/daqing/airway/blob/main/docs/plugin.md
