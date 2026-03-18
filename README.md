# simple-abtest

`simple-abtest` 是一套可自部署的 A/B 实验平台，覆盖实验配置、在线分流、本地判定和结果校验的完整链路，适合需要自己掌控实验模型、访问链路和数据资产的团队。

项目提供三类核心能力：

- 管理端：创建应用、实验、Layer、Segment、Group，维护配置版本和访问权限。
- 分流引擎：按 `appid + key + context` 在线返回命中的配置和标签。
- 本地 SDK：拉取引擎快照后在业务进程内完成过滤和分流，降低线上决策请求开销。

## 核心功能

### 1. 应用与实验管理

- 以应用为顶层管理多个实验。
- 支持实验的创建、编辑、启停和删除。
- 支持为实验配置过滤条件，只让符合业务上下文的请求进入实验。

### 2. 分层流量模型

- 支持 `Experiment -> Layer -> Segment -> Group` 的分层建模。
- 一个实验可以包含多个 Layer，每个 Layer 输出一份独立配置。
- Layer 下可以拆分多个 Segment，用于表达复杂实验中的流量段关系。
- Segment 下可以配置多个 Group，用于控制组和实验组分流。

### 3. 配置版本管理

- Group 绑定当前生效配置，同时保留历史版本。
- 配置内容不限定格式，JSON、文本或自定义串都可以直接存储。
- 可以按时间回看配置历史，并切换当前生效版本。

### 4. 在线校验与强制命中

- 引擎返回每个 Layer 命中的配置和标签，方便联调与埋点。
- 支持按 key 强制命中指定 Group，便于灰度验收、问题复现和定向排查。
- UI 内置在线验证页面，可直接输入 key 和 context 观察命中结果。

### 5. 权限协作

- 支持用户注册、登录、会话管理。
- 支持按应用维度授予只读、读写、管理员权限。
- 适合多角色协作维护同一应用下的实验。

## 服务组成

- `admin`：管理后台发布站，提供管理 API、`/ui/...` 静态资源和 `/engine` 代理。
- `engine`：在线分流服务，周期性从 MySQL 拉取配置并构建内存模型。
- `ui`：Web 管理台前端工程，构建产物由 `admin` 以 `/ui/...` 路径发布。
- `sdk-go`、`sdk-java`、`sdk-cpp`：本地判定 SDK，拉取快照后在进程内执行分流。

## 流量模型说明

这个项目的流量模型里有两套桶位，职责不同：

- 百分桶：范围是 `[0,100)`，核心作用是跨 Layer 对齐 Segment。
- 千分桶：范围是 `[0,1000)`，核心作用是做 Segment 内各 Group 的份额均衡。

两者不要混用：

- 百分桶解决的是“不同 Layer 如何共享同一批流量段”。
- 千分桶解决的是“进入同一个决策段后，各 Group 如何分配份额”。

需要特别注意一个例外：

- 简单实验只有一个 Layer，此时 `Segment` 和百分桶都不参与决策。
- 简单实验实际只使用组内千分桶做分流。

这样设计的好处是：

- 多个 Layer 可以共享同一套实验级百分桶入口，保证跨层命中同一段流量。
- Group 份额调整只改千分桶位图，不需要破坏跨层对齐关系。
- 简单实验不必承担额外的 Segment / 百分桶心智负担。

更完整的设计说明见 [doc/traffic-model.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/doc/traffic-model.md) 和 [doc/data-model.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/doc/data-model.md)。

## 典型使用流程

1. 在管理端创建应用，获得 `app_id` 和 `access_token`。
2. 创建实验。系统会自动生成默认的 Layer、Segment 和默认 Group。
3. 如果需要更复杂的实验结构：
   - 新增 Layer，表示一份独立配置输出。
   - 在 Layer 中新增 Segment，并通过重平衡调整 `[0,100)` 的百分桶区间。
   - 在 Segment 中新增 Group，并通过重平衡调整组间千分桶份额。
4. 为 Group 上传配置内容，并将 `cfg_id` 绑定为当前生效配置。
5. 配置实验过滤条件并启用实验。
6. 业务侧通过在线接口或 SDK 获取命中结果。

## 请求方式

### 在线分流接口

向 `engine` 发送请求：

```http
POST /
ACCESS_TOKEN: <app-access-token>
Content-Type: application/json
```

```json
{
  "appid": 1001,
  "key": "user-123",
  "context": {
    "country": "CN",
    "platform": "ios"
  }
}
```

返回示例：

```json
{
  "config": {
    "feed_rank": "{\"version\":\"B\"}",
    "card_style": "{\"style\":\"large\"}"
  },
  "tags": [
    "feed_rank:variant_b",
    "card_style:control"
  ]
}
```

适用场景：

- 需要中心化决策。
- 希望业务端无状态接入。
- 需要和管理端配置实时保持一致。

接口细节见 [doc/engine-api.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/doc/engine-api.md)。

### 本地 SDK 判定

SDK 会先从 `engine` 拉取应用实验快照，再在本地做过滤与分流。适合高频调用场景。

Go 示例：

```go
package main

import (
	"fmt"
	"time"

	sdk "github.com/peterrk/simple-abtest/sdk-go"
)

func main() {
	client, err := sdk.NewClient("http://127.0.0.1:8080", 1001, "your-token", 5*time.Minute)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	cfg, tags := client.AB("user-123", map[string]string{
		"country":  "CN",
		"platform": "ios",
	})
	fmt.Println(cfg)
	fmt.Println(tags)
}
```

更多 SDK 说明见：

- [sdk-java/README.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/sdk-java/README.md)
- [sdk-cpp/README.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/sdk-cpp/README.md)

## 快速部署

### 环境依赖

- Go `1.24+`
- Node.js `22+`
- MySQL `8+`
- Redis `6+`

### 初始化数据库

先创建数据库，例如 `abtest`，然后执行：

```bash
mysql -uroot -p abtest < db/admin.sql
mysql -uroot -p abtest < db/engine.sql
```

### 准备配置文件

`admin/config.yaml` 示例：

```yaml
log:
  max_backups: 7
  max_days: 7
test: true
db: "user:password@tcp(127.0.0.1:3306)/abtest?parseTime=true&charset=utf8mb4"
redis:
  address: "127.0.0.1:6379"
  password: ""
  pool_size: 10
  idle_size: 2
session_prefix: "abtest:sess:"
privilege_prefix: "abtest:priv:"
```

`engine/config.yaml` 示例：

```yaml
log:
  max_backups: 7
  max_days: 7
test: true
db: "user:password@tcp(127.0.0.1:3306)/abtest?parseTime=true&charset=utf8mb4"
interval_s: 300
```

### 启动服务

```bash
go run ./admin -config admin/config.yaml -port 8001
go run ./engine -config engine/config.yaml -port 8080
```

### 前端开发模式

```bash
cd ui
npm install
npm run dev
```

默认开发代理见 [ui/vite.config.ts](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/ui/vite.config.ts)：

- `/api/* -> http://localhost:8001`
- `/engine/* -> http://localhost:8001`

此时访问 Vite 开发服务器即可，前端会经由 `admin` 代理调用 `engine`，不会直接跨站访问 `engine`。

### 前端生产部署

先构建 UI：

```bash
cd ui
npm install
npm run build
```

生产部署时建议显式指定 UI 产物目录和 engine 地址：

```bash
go run ./admin -config admin/config.yaml -port 8001 \
  -ui-resource ./ui/dist \
  -engine http://127.0.0.1:8080
go run ./engine -config engine/config.yaml -port 8080
```

发布后的访问方式：

- 管理端 API：`/api/...`
- UI：`/ui/`
- UI 静态资源：`/ui/assets/...`
- UI 调用的在线校验接口：`/engine`

也就是说，最终对外只需要暴露 `admin`；前端不再直接访问 `engine`，跨站问题由 `admin` 侧代理统一解决。

## 文档索引

- [doc/traffic-model.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/doc/traffic-model.md)：流量模型、百分桶与千分桶说明。
- [doc/data-model.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/doc/data-model.md)：数据模型与实体关系。
- [doc/admin-api.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/doc/admin-api.md)：管理端 API。
- [doc/engine-api.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/doc/engine-api.md)：引擎 API。
