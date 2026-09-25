# github.com/daqing/airway-tencentcloud-plugin

一个 [Airway](https://github.com/daqing/airway) 插件,集成
[腾讯云 Go SDK](https://github.com/TencentCloud/tencentcloud-sdk-go),
为宿主项目提供现成的腾讯云服务调用 API,免去每个 Airway 项目自行接入
SDK。目前支持:短信(SMS)。

## 开发

```bash
go get github.com/daqing/airway@latest
go mod tidy
```

不依赖 Airway 宿主项目也能测试 API,直接运行本地开发服务器:

```bash
go run ./install/ignore/devserver   # 默认监听 :3000,可用 LISTEN 覆盖
```

```bash
curl -X POST http://localhost:3000/api/v1/tencentcloud/sms/send/mock \
  -H 'Content-Type: application/json' \
  -d '{"phone_numbers":["+8618501234444"],"template_id":"1234567"}'
```

mock 接口无需任何配置;真实的 `/sms/send` 在首次调用时读取
`TENCENTCLOUD_*` 环境变量(见「配置」一节)。

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

## 短信 API

路由挂载在 `/api/v1/tencentcloud` 下。

### 发送短信

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

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `phone_numbers` | 是 | E.164 格式号码(`+8618501234444`);裸 11 位国内手机号也可以。单次最多 200 个,须全为境内或全为境外号码 |
| `template_id` | 是 | 已审核通过的模板 ID |
| `template_param_set` | 否 | 模板参数,按模板定义的顺序填写 |
| `sign_name` | 否 | 覆盖本次请求的 `TENCENTCLOUD_SMS_SIGN_NAME` |
| `session_context` | 否 | 用户上下文,会在状态中原样返回(最长 512 字节) |

响应(`code` 为 0 只表示腾讯云接受了请求;每个号码的发送结果要看对应的
status):

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

### Mock 发送(本地调试)

`POST /api/v1/tencentcloud/sms/send/mock`

请求字段、校验规则和响应结构与[发送短信](#发送短信)完全一致 —— 字段和
类型没有任何增减,客户端在两个接口之间切换不需要改动任何代码。区别仅
有一点:mock 不触碰腾讯云(不需要密钥和短信配置),生成的验证码就是
响应里 `request_id` 和每条 status 的 `serial_no` 的值(6 位数字字符串,
保留前导零)。

> 该接口会把验证码发给任何调用者,切勿暴露到本地开发以外的环境。

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

## 安装布局

内容通过两种方式到达宿主:`install/lib/` 下的代码通过空导入编译进宿主
二进制;`install/host/` 和 `install/deps/` 下的文件由 `plugin:install`
拷贝进宿主项目 —— 部署配置、伴生服务、SQL 迁移等 Go 构建之外的磁盘
文件。`install/ignore/` 和根 `.gitignore` 匹配的内容永远不会离开插件
仓库。

要发布 SQL 迁移,在 `install/host/db/migrate/` 下添加
`<version>_<name>.up.sql` / `.down.sql` 文件,并通过
`plugin.MigrationProvider` 暴露。

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
