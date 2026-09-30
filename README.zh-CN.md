# github.com/daqing/airway-tencentcloud-plugin

一个 [Airway](https://github.com/daqing/airway) 插件,集成
[腾讯云 Go SDK](https://github.com/TencentCloud/tencentcloud-sdk-go),
为宿主项目提供现成的腾讯云服务调用 API,免去每个 Airway 项目自行接入
SDK。目前支持:短信(SMS)。

同一个模块还附带第二个插件 `smsverify`,提供完整的「手机号 + 验证码」流程
—— 发码、限流、图形验证码、投递与校验 —— 宿主也不必再写一遍。

## 开发

```bash
go get github.com/daqing/airway@latest
go mod tidy
```

不依赖 Airway 宿主项目也能测试 API,直接运行本地开发服务器:

```bash
go run ./install/ignore/devserver   # 默认监听 127.0.0.1:3000,可用 LISTEN 覆盖
```

开发服务器无条件注册 `smsverify` 的端点,并用一个内存 SQLite 库支撑,因此
手机号验证码流程可以端到端跑通:

```bash
curl -s localhost:3000/api/v1/captcha
curl -sX POST localhost:3000/api/v1/sms_codes \
  -H 'Content-Type: application/json' -d '{"phone":"18501234444"}'
```

启动时带上 `AIRWAY_ENV=local`,短信走 mock 驱动,验证码会以 `dev_code` 返回
而不是真的发出去。`SMS_DRIVER=tencent` 时驱动在首次调用时读取
`TENCENTCLOUD_*` 环境变量(见「配置」一节)。

把 `DB_DSN` 指向 PostgreSQL 或 MySQL,开发服务器就改从那个库提供接口,并顺带
把模块的迁移应用上去 —— 想确认迁移在宿主真正使用的数据库上跑得通,这是最
快的办法:

```bash
DB_DSN='postgres://user:secret@127.0.0.1:5432/app' go run ./install/ignore/devserver
```

## 在宿主项目中使用

```bash
go get github.com/daqing/airway-tencentcloud-plugin
```

然后在宿主的 `plugins.go` 中以空导入启用:

```go
import (
	_ "github.com/daqing/airway-tencentcloud-plugin"
)
```

模块里有两个插件,这一次导入会把两个都注册上:`smsverify` 提供手机号验证码
端点;`tencentcloud` 不提供任何 HTTP 路由,宿主直接在进程内调用它的 Go API,
它存在是为了让 `plugin:install` 能解析到本模块,并让它旁边编译进来的表结构
进入宿主二进制。`plugin:install` 以解析到的插件命名安装,所以安装名固定是
`tencentcloud`;两个插件共用的表是编译进来的,不需要安装(见[安装布局](#安装布局))。

这次导入同样是宿主拿到插件 Go API 的方式 —— 代码被编译进宿主二进制,调用
全程在进程内,不经过 HTTP:

```go
import "github.com/daqing/airway-tencentcloud-plugin/install/lib/tencentcloud"

result, err := tencentcloud.SendSms(ctx, tencentcloud.SendSmsInput{
	PhoneNumbers:     []string{"+8618501234444"},
	TemplateID:       "1234567",
	TemplateParamSet: []string{"654321"},
})
if err != nil {
	// 请求根本没到腾讯云
}

for _, status := range result.SendStatuses {
	if status.Code != "Ok" {
		// 该号码被拒,status.Message 是原因
	}
}
```

`err` 为 nil 只表示腾讯云接受了这次请求 —— 每个号码仍可能失败,所以要
逐个检查 `SendStatus.Code`,值为 `"Ok"` 才表示该号码发送成功。
`install/lib/tencentcloud` 返回的是插件自己的类型,不泄漏 SDK 类型,宿主
本身不依赖腾讯云 SDK。

## 配置

插件在首次调用时从环境变量读取配置(写在宿主的 `.env` 里):

| 变量 | 必填 | 说明 |
| --- | --- | --- |
| `TENCENTCLOUD_SECRET_ID` | 是 | 腾讯云 API 密钥 SecretId |
| `TENCENTCLOUD_SECRET_KEY` | 是 | 腾讯云 API 密钥 SecretKey |
| `TENCENTCLOUD_SMS_SDK_APP_ID` | 是 | [短信控制台](https://console.cloud.tencent.com/smsv2/app-manage)中的应用 SdkAppId,如 `1400006666` |
| `TENCENTCLOUD_SMS_SIGN_NAME` | 国内短信必填 | 默认签名内容(不是签名 ID);可在请求中覆盖 |
| `TENCENTCLOUD_REGION` | 否 | SDK 接入地域,默认 `ap-guangzhou` |

缺少变量只会在第一次 API 调用时报告,不会阻止宿主启动,尚未配置插件的
宿主可以正常启动。

## 手机号验证码(`smsverify`)

模块的第二个插件实现了登录/注册页面需要的那一整套:通过短信下发验证码、
限制同一个手机号和同一个 IP 的请求频率、对过于频繁的 IP 要求图形验证码,
以及校验验证码。

**这两个端点在所有环境下都会挂载** —— `AIRWAY_ENV` 不影响它们,它们也是本
模块仅有的 HTTP 接口。它们天生面向客户端直接调用,防护靠下面的限流而不是
鉴权。插件挂在 `/api/v1` 下,占用了这个公共前缀:宿主或其他插件如果也注册
`/api/v1/sms_codes` 或 `/api/v1/captcha`,启动时会直接冲突。

它需要的两张表(`sms_verifications`、`captchas`)是编译进模块的 Go 迁移,
所以宿主常规的 `go run . db:migrate` 会建表,`plugin:install` 没有任何文件要
拷。建表语句按宿主所用的数据库生成,PostgreSQL、MySQL、SQLite 都能直接用。
在这之前两个端点都返回 `10000`,并带上数据库自己的「表不存在」错误。

> 迁移只负责建表,不会接管已存在的表。宿主若已经通过自己的迁移建好了这两张
> 表,要么先删掉它们让 `db:migrate` 重建,要么在表结构已经与本模块定义一致时
> 把这个模块用的版本号记进去,避免 `db:migrate` 因为表已存在而失败:
>
> ```sql
> INSERT INTO schema_migrations (version, applied_at)
> VALUES ('20260929153501', NOW()), ('20260929153502', NOW());
> ```

### 发送验证码

`POST /api/v1/sms_codes`

```bash
curl -X POST http://127.0.0.1:3000/api/v1/sms_codes \
  -H 'Content-Type: application/json' \
  -d '{"phone":"18501234444"}'
```

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `phone` | 是 | 中国大陆手机号:`1[3-9]` 开头共 11 位,不带国家码 |
| `captcha_token` | 需要验证码时必填 | [获取验证码](#获取验证码)返回的 `token` |
| `captcha_answer` | 需要验证码时必填 | 图片上的四位数字 |

```json
{"code": 0, "data": {"sent": true}, "message": ""}
```

mock 驱动下验证码也会一并返回,便于本地联调而不必开通短信服务:

```json
{"code": 0, "data": {"dev_code": "017099", "sent": true}, "message": ""}
```

所有响应都是 HTTP 200,结果由 `code` 区分:

| `code` | 含义 |
| --- | --- |
| `0` | 已发送 |
| `40001` | 请求体无法解析、手机号非法,或验证码答错 |
| `40301` | 需要图形验证码;`data` 里带 `captcha_required: true` 与 `captcha_url` |
| `42901` | 触发限流 |
| `10000` | 其它错误 —— 投递被拒、配置缺失等;具体原因看 `message` |

### 获取验证码

`GET /api/v1/captcha`

返回一个绑定调用方 IP 的四位图形验证码,以 base64 PNG 形式给出,客户端无需
再发一次请求。`token` 就是发码端点要的 `captcha_token`:

```json
{
  "code": 0,
  "data": {
    "token": "16923e222c57be8d",
    "image": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAJYAAAA0CAIAAADnt1ZQ..."
  },
  "message": ""
}
```

与发码端点一样,所有响应都是 HTTP 200:

| `code` | 含义 |
| --- | --- |
| `0` | 已签发 |
| `42901` | 该 IP 已达每小时验证码上限 |
| `10000` | 其它错误,具体原因看 `message` |

验证码有效期 10 分钟且只能使用一次;答错同样会作废,所以无法把猜测分摊到
多次尝试上。同一 IP 每小时最多索取 30 张 —— 超过后返回 `42901`,既不生成
也不落库,所以被拒的请求只花掉一次计数查询。

### 限流参数

| 限制 | 取值 |
| --- | --- |
| 同一手机号重发间隔 | 60 秒 |
| 同一手机号 | 24 小时 10 条 |
| 同一 IP | 24 小时 30 条 |
| 同一 IP 要求验证码 | 每小时超过 5 条之后 |
| 同一 IP 索取验证码 | 每小时 30 张 |
| 验证码有效期 | 5 分钟 |
| 单个验证码允许答错次数 | 5 次 |

按 IP 的限制取自请求中的客户端 IP,所以处在反向代理后面的宿主必须调用
`router.SetTrustedProxies` —— gin 默认信任所有代理,调用方可以轮换
`X-Forwarded-For` 绕过这些限制。按手机号的限制不受此影响。

### Go API

宿主若想自己写端点,或想自己决定「号码验证通过」意味着什么,可以直接调用
包内的函数。`Send` 就是 HTTP handler 包的那一层;`Verify` 刻意不提供端点,
因为签发会话、创建用户这类决定属于宿主:

```go
import (
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/captcha"
	"github.com/daqing/airway-tencentcloud-plugin/install/lib/smsverify"
)

result, err := smsverify.Send(ctx, "18501234444", clientIP, captchaToken, captchaAnswer)
if errors.Is(err, smsverify.ErrCaptchaRequired) {
	// 下发一张验证码,再带 captcha_token 与 captcha_answer 重试
}
if errors.Is(err, smsverify.ErrRateLimited) {
	// 冷却期未过或触达日上限,提示用户稍后再试
}

if err := smsverify.Verify(ctx, "18501234444", code); err != nil {
	// ErrCodeInvalid —— 不存在、已过期、已消费,或就是错的
	// ErrCodeTooManyAttempts —— 试错次数用尽,让用户重新获取
}

// 验证码端点包的就是这两个,宿主自己画页面时可以直接用:
// token 交给客户端,png 是直接可返回的图片。
token, png, err := captcha.Issue(ctx, clientIP) // 超过每小时 30 张返回 ErrRateLimited
ok := captcha.Verify(ctx, clientIP, token, answer)
```

`Verify` 会消费验证码,所以同一个验证码最多只会返回一次 nil。「验证码错误」和
「该号码没有在途验证码」刻意返回同一个错误:调用方不该能借此探测哪些号码正在
收验证码。`captcha.Verify` 同样会消费验证码,而且无论答对答错都是先到者生效。

### 投递方式

`SMS_DRIVER` 决定走哪个通道:

| 取值 | 效果 |
| --- | --- |
| `mock` | 只打日志不发短信,并把验证码作为 `dev_code` 返回 |
| `tencent` | 通过腾讯云发送真实的模板短信 |

不设置时驱动跟随环境:本地模式(`AIRWAY_ENV=local`)为 `mock`,其它环境为
`tencent` —— 生产环境不会因为忘记配置而静默降级成 mock。设了无法识别的值会
直接报错,而不是回退。

`tencent` 驱动除了上面的 `TENCENTCLOUD_*` 变量外,还需要
`TENCENTCLOUD_SMS_TEMPLATE_ID`(已审核的模板,验证码作为唯一参数),并在号码
前加 `+86`。

> 在非本地模式下显式设成 `SMS_DRIVER=mock` 仍然不发短信,也仍然会把验证码写回
> 响应体。切勿在生产宿主上这样配置。

## 安装布局

内容通过两种方式到达宿主:`install/lib/` 下的代码通过空导入编译进宿主
二进制;`install/deps/` 下的文件由 `plugin:install` 拷贝进宿主项目 ——
部署配置、伴生服务、这些 Go 构建之外、由磁盘上的工具读取的东西。本模块
不再发布别的:`install/host/` 目录已经没有了,`smsverify` 背后的两张表属于
「编译进去」那一类。`install/ignore/` 和根 `.gitignore` 匹配的内容永远不会
离开插件仓库。

表结构交给插件自己管时,用 Go 迁移发布 —— 在 `install/lib/migrations/` 下的
`init()` 里调用 `schema.RegisterChange`,并空导入,让每个宿主二进制都带着这些
定义:`lib/migrate` 会针对宿主的数据库生成 DDL,`db:migrate` 无需任何安装
步骤就能看到它们。需要让宿主能读能改那段 SQL 时,才改用
`install/host/db/migrate/` 下的 `<version>_<name>.up.sql` / `.down.sql`,
并通过 `plugin.MigrationProvider` 暴露:安装器会给每个文件在宿主的
`db/migrate/` 里分配一个新时间戳(宿主已有同名文件则跳过),之后由迁移引擎
原样执行,所以那些 SQL 必须自己适配宿主的数据库。

要发布额外的项目文件(伴生服务、部署配置等),放在
`install/deps/tencentcloud/` 下 —— `plugin:install` 会把整个目录树合并
进宿主项目的 `deps/` 目录,已存在的文件会被跳过。名为 `go.mod` 的文件
要以 `go.mod.templ` 发布(安装为 `go.mod`):Go module zip 会丢弃嵌套
模块,`install/deps/` 里真实的 `go.mod` 永远到不了宿主。为了本地构建把
真实 `go.mod` 放在 `.templ` 旁边没问题 —— 安装只发布 `.templ` 内容,
两者内容漂移时安装会失败。

需要留在插件仓库但绝不能安装的文件放在 `install/ignore/` 下:
`plugin:install` 只读取 `install/host/` 和 `install/deps/`,并会跳过其中
的所有 `ignore/` 目录,放在那里的内容不会到达宿主项目。根 `.gitignore`
同样被遵守:被忽略的文件(`node_modules/`、`.env` 等)不会被安装进宿主
项目。

完整插件指南见
https://github.com/daqing/airway/blob/main/docs/plugin.md
