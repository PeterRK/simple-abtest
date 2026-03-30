# simple-abtest

[中文](README.md) | [English](README-EN.md) | [Español](README-ES.md) | [Français](README-FR.md) | [日本語](README-JA.md) | [Deutsch](README-DE.md)

`simple-abtest`是一套可自部署的A/B实验平台，帮助团队统一管理实验、在线分流，并支持本地SDK判定。适合希望自己掌控实验配置、访问链路和数据资产的团队。

## 功能亮点

- 实验管理：按应用维护实验，支持创建、编辑、启停和删除。
- 权限管理：支持多用户登录，并按应用分配只读、读写、管理员权限。
- 条件过滤：可按业务上下文设置过滤条件，只让符合条件的请求进入实验。
- 在线验流：可直接输入key和context查看命中结果，方便调试。
- 强制命中：支持按key定向命中指定实验组，用于测试验证。
- 复杂实验：双重分流，支持多层贯穿流量；支持实验组流量轮转和配置回溯，方便长期实验。
- 多种接入：既可通过在线接口实时决策，也可通过本地SDK降低请求开销。

## 管理站
### 列表页
![](images/list.png)
### 详情页
![](images/detail.png)
### 验流页
![](images/verify.png)
### 使用注意
- 管理端目前对命名以及强制命中规则没有做排他性检查，出现冲突时的表现不可控，使用时请注意避免冲突。
- 出于缓解长期实验中的流量极化考虑，非默认组在调整流量时会和默认组做流量槽轮转（即时流量比例不变）。由于这种特性，默认组有些时候不适合作为控制组，需要单独划分控制组。
- 在线验流是调用分流服务做的，从管理端操作完成到配置生效会有延迟。
- 数据可视化功能暂未实现，请根据流量标签在其他数据可视化平台上实施。

## 服务组成

```
User -> Admin - UI
          \
Client ->- -> Engine
```

- `admin`：管理站。
- `engine`：分流服务。
- `sdk-go`、`sdk-java`、`sdk-cpp`：本地判定SDK。

## 接入方式

### 在线分流

业务侧向`engine`发送请求，获取命中的配置和标签。管理站的在线验流页也走这条路径，因此同样有生效延迟。

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

### 本地SDK

SDK会定期拉取实验快照，在业务进程内完成判定，适合高频调用场景。

Go示例：

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

其他SDK说明：

- [sdk-java/README.md](sdk-java/README.md)
- [sdk-cpp/README.md](sdk-cpp/README.md)

## 快速部署

依赖环境（推荐）：

- Go `1.26+`
- Node.js `22+`
- MySQL `8+`
- Redis `6+`

初始化数据库：

```bash
mysql -uroot -p abtest < db/admin.sql
mysql -uroot -p abtest < db/engine.sql
```

配置文件示例：

`admin/config.yaml`

```yaml
db: "abtest:abtest@tcp(127.0.0.1:3306)/abtest?parseTime=true&charset=utf8mb4"
redis:
  address: "127.0.0.1:6379"
  password: ""
  pool_size: 10
  idle_size: 2
redis_prefix: "sab-"
secret: ""
engine: "http://127.0.0.1:8080"
test: false
```

说明：

- `db`：MySQL连接串，`admin`和`engine`都会使用。
- `redis.address`：Redis 地址，管理端用于会话和权限缓存。
- `redis_prefix`：建议为当前环境设置独立前缀，避免和其他环境混用。
- `secret`：可选的预置签名密钥。留空时由系统按现有逻辑处理。
- `engine`：`admin` 代理在线验证等请求时访问的引擎地址；留空时默认使用 `http://127.0.0.1:8080`。
- `test`：设为 `true` 时会打开更详细的调试能力，不建议生产环境开启。

`engine/config.yaml`

```yaml
db: "abtest:abtest@tcp(127.0.0.1:3306)/abtest?parseTime=true&charset=utf8mb4"
interval_s: 300
```

说明：

- `interval_s`：引擎从数据库拉取最新实验快照的周期，默认建议保留为 `300` 秒。

构建产物：

```bash
./build.sh
```

脚本会检查本机是否具备Go和Node.js/npm构建环境，然后构建：

- `bin/admin`
- `bin/engine`
- `ui/dist`

启动服务时建议直接使用构建后的二进制：

```bash
./bin/admin -config admin/config.yaml -port 8001 -ui-resource ./ui/dist
./bin/engine -config engine/config.yaml -port 8080
```
