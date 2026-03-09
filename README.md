# simple-abtest

`simple-abtest` 是一套可自部署的 A/B 实验平台，覆盖「实验配置管理 -> 在线分流 -> 结果验证」全链路，适合中小团队快速落地灰度和策略实验。

## 业务功能

### 1. 应用与实验管理

- 以应用（App）为顶层管理多个实验。
- 支持实验创建、编辑、删除、启停。
- 支持实验描述与过滤条件配置，按业务上下文精准命中目标用户。

### 2. 分层分流能力

- 支持 `Layer -> Segment -> Group` 分层流量建模。
- 支持按阶段调整 Segment 流量区间，实现灰度扩量与回滚。
- 支持 Group 流量份额重平衡，满足对照组/实验组配比调整。

### 3. 配置管理与版本回溯

- Group 绑定业务配置，支持配置版本历史保留。
- 可按版本切换当前生效配置，便于回看与回退。

### 4. 在线验证与联调

- 提供在线分流接口，按 `appid + key + context` 返回命中结果。
- 返回命中标签，便于日志分析和效果归因。
- 支持指定 key 强制命中分组，用于验收测试和问题复现。

### 5. 账号与权限协作

- 支持用户注册/登录与会话管理。
- 支持应用级权限分配（只读/读写/管理员）。
- 支持多人协作管理同一应用下实验。

## 服务组成

- `admin`：管理后台服务，负责应用/实验/流量/权限管理 API。
- `engine`：在线分流服务，负责实时返回命中配置。
- `ui`：Web 管理台，提供实验配置、权限管理和在线验证界面。

## 快速部署（单机开发/测试）

### 1. 环境依赖

- Go `1.26+`
- Node.js `22+`
- MySQL `8+`
- Redis `6+`

### 2. 初始化数据库

先创建数据库（示例：`abtest`），再执行：

```bash
mysql -uroot -p abtest < db/admin.sql
mysql -uroot -p abtest < db/engine.sql
```

### 3. 准备服务配置

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

### 4. 启动后端服务

```bash
go run ./admin -config admin/config.yaml -port 8001
go run ./engine -config engine/config.yaml -port 8080
```

### 5. 启动前端管理台

```bash
cd ui
npm install
npm run dev
```

默认开发代理（`ui/vite.config.ts`）：

- `/api/* -> http://localhost:8001`
- `/engine/* -> http://localhost:8080`
