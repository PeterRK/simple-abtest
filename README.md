# simple-abtest

A simple A/B test platform.

## 项目整体功能

仓库包含三个子项目，形成一套完整的 A/B 实验平台：

- `admin`（后端管理服务）：
  - 提供实验管理 API（应用、实验、Layer、Segment、Group、配置版本、用户与权限）。
  - 负责将实验配置写入 MySQL，并使用 Redis 维护会话与权限缓存。
- `engine`（后端分流服务）：
  - 周期性从 MySQL 拉取实验配置，构建内存模型。
  - 对业务请求执行过滤与分流，返回命中的各层配置结果。
- `ui`（前端管理台）：
  - 调用 `admin` API 管理实验结构与配置。
  - 调用 `engine` API 做在线验证（给定 `appid/key/context` 查看命中配置）。

整体链路是：在 `ui` 上配置实验 -> `admin` 写入数据库 -> `engine` 周期拉取并生效 -> 业务侧调用 `engine` 获取分流结果。

## 部署方式

以下为单机部署（开发/测试环境）示例。

### 1. 准备依赖

- Go（建议 1.26+）
- Node.js（建议 22+）
- MySQL 8+
- Redis 6+

### 2. 初始化数据库

创建数据库后执行：

```bash
mysql -uroot -p < db/admin.sql
mysql -uroot -p < db/engine.sql
```

### 3. 准备服务配置

`admin` 配置示例（如 `admin/config.yaml`）：

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

`engine` 配置示例（如 `engine/config.yaml`）：

```yaml
log:
  max_backups: 7
  max_days: 7
test: true
db: "user:password@tcp(127.0.0.1:3306)/abtest?parseTime=true&charset=utf8mb4"
interval_s: 300
```

### 4. 启动后端服务

```bash
go run ./admin -config admin/config.yaml -port 8001
go run ./engine -config engine/config.yaml -port 8080
```

健康检查：

- `admin`: `GET http://127.0.0.1:8001/health`
- `engine`: `GET http://127.0.0.1:8080/health`

### 5. 启动前端

`ui/vite.config.ts` 已内置代理：

- `/api` -> `http://localhost:8001`
- `/engine` -> `http://localhost:8080`

开发模式：

```bash
cd ui
npm install
npm run dev
```

生产构建：

```bash
cd ui
npm install
npm run build
```

构建产物在 `ui/dist`，部署到静态服务器后，需保证：

- `/api/*` 转发到 `admin` 服务
- `/engine/*` 转发到 `engine` 服务（并去掉前缀 `/engine`）
