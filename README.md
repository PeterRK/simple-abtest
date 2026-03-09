# simple-abtest

`simple-abtest` 是一套面向业务迭代场景的 A/B 实验平台，覆盖「实验配置管理 -> 分流决策生效 -> 线上验证」全链路。  
平台目标是帮助业务团队在不改动线上主流程代码的前提下，快速进行功能灰度、策略对比、参数调优与效果验证。

## 业务功能

平台围绕实验生命周期提供以下核心能力：

- **应用与实验管理**
  - 以应用（App）为顶层组织实验，支持在同一应用下维护多组实验。
  - 支持实验启停、实验描述维护、实验过滤规则配置，便于按业务条件精准命中目标用户。
- **分层实验与流量建模**
  - 支持 `Layer -> Segment -> Group` 的层级化建模，一个实验可拆分多个独立配置层并行输出。
  - 支持 Segment 区间调整与流量重平衡，实现分阶段扩量、缩量和灰度推进。
  - 支持 Group 级别流量份额调整与种子打散，降低历史分桶干扰，便于重新抽样。
- **配置版本与回溯能力**
  - Group 绑定配置内容，支持配置版本历史留存，满足回看、比对与回滚诉求。
  - 引入乐观锁版本控制，减少多人协作下的并发覆盖问题。
- **在线分流与命中解释**
  - `engine` 服务按 `appid + key + context` 做实时决策，返回每个 Layer 的命中配置。
  - 返回命中标签（如 `layer:group`），便于业务侧日志对齐和效果分析。
- **强制命中与联调排障**
  - 支持将指定 key 强制命中某个实验分组，用于产品验收、问题复现和定向回归测试。
  - 管理台提供在线验证入口，可直接输入上下文查看当前命中结果。
- **账号体系与权限控制**
  - 提供用户注册/登录、会话校验、应用级授权（只读/读写/管理员）能力。
  - 支持按应用分配协作权限，保障跨团队实验操作边界。

## 典型业务流程

1. 业务方在管理台创建应用与实验，并设置过滤条件、Layer/Segment/Group 结构与配置内容。  
2. 管理服务将配置持久化到 MySQL，维护会话与权限缓存。  
3. 分流服务定期拉取最新配置并构建内存模型。  
4. 线上业务请求携带 `appid/key/context` 调用分流服务，获取实时命中配置并落日志分析效果。  
5. 当需要灰度扩量、回滚或定向排查时，直接在管理台调整流量、切换配置版本或设置强制命中。

## 子项目功能

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
