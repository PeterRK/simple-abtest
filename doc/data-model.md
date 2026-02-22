## 概览

`simple-abtest` 包含两个服务：

- **admin 服务**：管理实验配置，直接读写 MySQL 中的业务表；
- **engine 服务**：周期性从 MySQL 拉取配置，构建内存模型并对线上请求进行分流。

两者共享同一套数据模型，底层主要表如下：

- `application`：应用（App）维度
- `experiment`：实验
- `exp_layer`：实验的层（Layer）
- `exp_segment`：层下的分段（Segment）
- `exp_group`：分组（Group），承载流量与配置引用
- `exp_config`：具体配置内容（版本历史）

数据库层面不使用外键约束，而是依赖应用层逻辑并配合定期清理脚本维持一致性。  
整体设计面向低竞争环境，通过乐观锁（`version` 字段）来避免更新冲突。

---

## 实体与字段说明

### 1. application 表

定义应用的基本信息，是所有实验的顶级归属。

- `app_id`：应用 ID，主键，自增；
- `name` / `description`：应用名称与描述；
- `version` / `update_time`：乐观锁与更新时间。

---

### 2. experiment 表

定义某个应用下的一套 A/B 实验。

- `exp_id`：实验 ID，主键，自增；
- `app_id`：所属应用 ID（逻辑外键到 `application.app_id`）；
- `name` / `description`：实验名称与描述；
- `seed`：用于“富实验”层面Segment分配的随机种子；
- `filter`：JSON 表达式（表达式树节点数组），用于按请求上下文过滤实验；
- `status`：实验状态，0 表示停用，1 表示启用；
- `version` / `update_time`：乐观锁与更新时间。

---

### 3. exp_layer 表

实验下的“层（Layer）”，用于在一个实验中承载多个独立的配置维度。

- `lyr_id`：层 ID，主键，自增；
- `exp_id`：所属实验 ID（逻辑外键）；
- `name` / `description`：层名称与描述；
- `version` / `update_time`：乐观锁与更新时间。

---

### 4. exp_segment 表

层下的分段（Segment）。针对 **富实验**（多层实验），它将用户映射到 \[0,100) 的“粗粒度槽位段”，再在段内使用更细粒度的 0–999 槽位和 group bitmap 实现分组。这样设计可以实现跨层的Segment对齐。

- `seg_id`：分段 ID，主键，自增；
- `lyr_id`：所属层 ID（逻辑外键）；
- `range_begin` / `range_end`：该段覆盖的粗粒度区间 \[0,100)，左闭右开；
- `seed`：该段内进一步 hash 到 0–999 槽位的种子；
- `version` / `update_time`：乐观锁与更新时间。

典型约定：

- 某个 layer 下所有 segment 的 \[begin, end) 区间覆盖 \[0,100)，且两两不重叠、连续；
- 初始情况下会创建一个覆盖 \[0,100) 的默认 segment。

---

### 5. exp_group 表

Segment 下的具体分组（Group），负责持有实际“流量份额与配置引用”。

- `grp_id`：分组 ID，主键，自增；
- `seg_id`：所属 segment ID（逻辑外键）；
- `name` / `description`：分组名称与描述；
- `share`：该 group 拥有的 **槽位数量**（0–1000），用于快速查阅流量占比；
- `bitmap`：长度 125 字节的 bitset，对应 0–999 的 1000 个细粒度槽位；
- `is_default`：是否为默认组。每个 segment 至少有一个默认组，且其 bitmap 覆盖剩余所有槽位；
- `force_hit`：逗号分隔的 key 列表，用于强制命中的 key（绕过正常 hash 分配逻辑）；
- `cfg_id`：当前生效的配置 ID（逻辑外键到 `exp_config.cfg_id`）。0 表示未绑定配置；
- `version` / `update_time`：乐观锁与更新时间。

---

### 6. exp_config 表

存储 Group 级别的**具体配置内容**，支持版本历史。

- `cfg_id`：配置 ID，主键，自增；
- `grp_id`：所属 group ID（逻辑外键）；
- `content`：配置内容，可能是 JSON 字符串，但系统不强制；
- `create_time`：配置创建时间，用于查询历史版本。

---

## 实体关系与层级结构

整体层级自上而下为：

```text
application
  └── experiment
        └── exp_layer
              └── exp_segment
                    └── exp_group
                          └── exp_config
```

- 一个 `application` 可以有多个 `experiment`；
- 一个 `experiment` 可以有多个 `exp_layer`，每个 layer 输出一个独立的配置；
- 一个 `exp_layer` 可以有多个 `exp_segment`，以 \[0,100) 的区间划分流量段；
- 一个 `exp_segment` 可以有多个 `exp_group`，每个 group 对应一套配置；
- 一个 `exp_group` 在任意时刻只绑定一个 `cfg_id`，但可以拥有多个 `exp_config` 历史记录。

跨表一致性通过 admin 逻辑保证，例如：

- 创建 experiment 时会自动创建默认 layer 与默认 segment、默认 group；
- 删除应用前会校验是否存在实验；
- 删除 layer/segment/group 时会检查是否有无法满足的约束（如 share 非 0、是 default 等）。

---

## 清理脚本与一致性

`db/clean.sql` 用于清理悬置数据：

这体现了上面提到的实体层级关系，同时弥补了数据库层面没有外键约束的不足。

---

## 运行时内存模型与 engine 行为

engine 服务通过 `engine/db/mysql.go` 中的 `Fetch` 方法，从上述表构建内存结构：

- `map[uint32][]core.Experiment`：key 为 `app_id`；
- `core.Experiment`：
  - `Seed`：来自 `experiment.seed`；
  - `Filter`：由 `experiment.filter` 解析得到的 `[]core.ExprNode`；
  - `Layers`：由 `exp_layer`、`exp_segment`、`exp_group` 联合构建。
- `core.Layer`：
  - `Name`：`exp_layer.name`；
  - `Segments`：对应该 layer 下的所有 segment；
  - `ForceHit`：从 `exp_group.force_hit` 解析出的强制命中映射，key → group。
- `core.Segment`：
  - `Range.Begin` / `Range.End`：取自 `exp_segment.range_begin` / `range_end`；
  - `Seed`：`exp_segment.seed`；
  - `Groups`：该 segment 下的所有 group。
- `core.Group`：
  - `Name`：`exp_group.name`；
  - `Bitmap`：`exp_group.bitmap`；
  - `Config`：实际使用的配置内容，来自与 `exp_group.cfg_id` 关联的 `exp_config.content`。

engine 在处理请求时的大致流程为：

1. 根据请求中的 `appid` 取出对应的 `[]core.Experiment`；
2. 对每个 experiment：
   - 用 `EvalExpr(Filter, context)` 判断是否命中过滤条件；
   - 若命中，进入 `GetExpConfig` 做分流：
     - 简单实验（单 layer 单 segment）：直接用 segment 的 seed/bitmap 做 0–999 槽位 hash，并考虑 layer 级别的强制命中映射 `ForceHit`；
     - 富实验：先用 `experiment.seed` 将用户 hash 到 \[0,100) 的 slot，选择对应 segment，然后在 segment 内部做 0–999 槽位 hash，再结合 `ForceHit` 得到最终 group。
3. 将所有 layer 的 `Group.Config` 聚合成 `map[layerName]config`，并返回给调用方。

通过这种设计，**admin** 负责维护结构化的实验/分层/分段/分组/配置数据，**engine** 则将其转化为高效的内存模型与位图运算，实现在线稳定分流。

---

## 强制命中机制说明

在实际业务中，往往需要“指定某个用户/设备必然命中某个分组”，比如：

- 产品同学需要验证某个配置是否正确，希望把自己的账号固定到某个实验分组；
- 线上问题排查时，需要让某些 key 保持稳定的实验行为，避免 hash 变动带来的噪声。

为此，系统提供了基于 `exp_group.force_hit` 字段的“强制命中”能力：

- `exp_group.force_hit` 是一个逗号分隔的 key 列表；
- 在加载配置时，engine 会在每个 layer 内部把所有 group 的 `force_hit` 合并到一个 map：
  - key：强制命中的 key（例如某个用户 ID）；
  - value：指向该 layer 下某个具体 `Group` 的指针。
- 这个 map 最终存放在 `core.Layer.ForceHit` 中。

请求进入 engine 时，分流过程会优先检查强制命中映射：

1. 根据 appid 找到对应的 `[]core.Experiment`；
2. 对命中过滤条件的 experiment，在每个 layer 上：
   - 先在 `Layer.ForceHit` 中查询当前请求的 key；
   - 如果命中，则**直接返回对应 group**（及其配置），不再走 hash + bitmap 的正常分流逻辑；
   - 如果未命中，再按照“富实验/简单实验”流程，用种子和 bitmap 进行常规分流。

这样设计的好处：

- 强制命中与正常分流逻辑完全解耦，业务方只需要维护 `force_hit` 字段即可；
- 一旦需要撤销强制命中，只需清空相应 group 的 `force_hit` 内容，engine 在下一次拉取配置后就会恢复为纯 hash 分流。
