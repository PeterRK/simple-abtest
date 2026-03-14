# 数据模型

## 总览

`simple-abtest` 由 `admin` 维护结构化实验数据，由 `engine` 周期性拉取并构建内存模型。核心实体如下：

- `application`：应用。
- `experiment`：实验。
- `exp_layer`：实验中的配置层。
- `exp_segment`：Layer 下的流量段。
- `exp_group`：Segment 下的分组。
- `exp_config`：Group 关联的配置历史。

整体关系：

```text
application
  -> experiment
    -> exp_layer
      -> exp_segment
        -> exp_group
          -> exp_config
```

## application

应用是最顶层容器，用于承载实验和权限。

关键字段：

- `app_id`：应用 ID。
- `name`、`description`：应用名称和描述。
- `access_token`：调用 `engine` 时使用的访问令牌。
- `version`：乐观锁版本号。

说明：

- 权限也是按应用粒度授予。
- 一个应用下可以包含多个实验。

## experiment

实验定义“谁能进入实验”和“进入实验后有哪些 Layer”。

关键字段：

- `exp_id`：实验 ID。
- `app_id`：所属应用。
- `seed`：实验级哈希种子；只在富实验中用于生成百分桶，支撑跨 Layer 的 Segment 对齐。
- `filter`：请求上下文过滤表达式。
- `status`：`0` 为停用，`1` 为启用。
- `version`：乐观锁版本号。

说明：

- `filter` 命中后，实验才会参与后续分流。
- 简单实验中不会使用实验级百分桶。
- 在富实验里，`seed` 先把用户映射到同一套百分桶入口，再进入各 Layer。

## exp_layer

Layer 表示实验中的一个独立输出维度。

关键字段：

- `lyr_id`：Layer ID。
- `exp_id`：所属实验。
- `name`：Layer 名称，同时也是返回结果里 `config` 的键名。
- `version`：乐观锁版本号。

说明：

- 一个实验可有多个 Layer。
- 每个 Layer 最终返回一个命中的 Group 配置。
- 多个 Layer 共享实验级百分桶，用来保持 Segment 对齐。
- 如果实验只有一个 Layer，则不会发生跨层对齐。

## exp_segment

Segment 是 Layer 下的结构节点。在富实验里，它用于承载跨层对齐关系；在简单实验里，它只是默认结构，不参与决策。

关键字段：

- `seg_id`：Segment ID。
- `lyr_id`：所属 Layer。
- `range_begin`、`range_end`：百分桶区间，左闭右开，范围是 `[0,100)`。
- `seed`：Segment 级哈希种子，用于生成千分桶，在当前分组范围内做 Group 份额均衡。
- `version`：乐观锁版本号。

约束：

- 一个 Layer 下的全部 Segment 必须连续覆盖 `[0,100)`。
- 新建实验或 Layer 时会自动创建一个默认 Segment：`[0,100)`。

说明：

- 富实验中，Segment 的核心职责是承载百分桶区间。
- 简单实验中，虽然仍然存在一个默认 Segment，但它不参与路由选择。
- 它表达的是跨 Layer 对齐后的流量段，而不是 Group 之间的份额本身。

## exp_group

Group 是实际命中单元，承载份额、配置和强制命中规则。

关键字段：

- `grp_id`：Group ID。
- `seg_id`：所属 Segment。
- `name`：Group 名称。
- `share`：当前占有的千分桶数量，范围 `0` 到 `1000`。
- `bitmap`：长度为 125 字节的位图，对应 1000 个千分桶。
- `is_default`：是否默认组。
- `force_hit`：强制命中的 key 列表。
- `cfg_id`：当前生效配置 ID。
- `version`：乐观锁版本号。

说明：

- `share` 是 `bitmap` 的摘要信息，表示当前 Group 在分组决策中持有多少份额。
- 默认组负责兜底，保证全部 1000 个桶位都被覆盖。
- 非默认组从默认组手里拿走部分桶位，形成组间均衡结果。

## exp_config

配置表保存 Group 的历史配置内容。

关键字段：

- `cfg_id`：配置 ID。
- `grp_id`：所属 Group。
- `content`：配置正文，不限制格式。
- `stamp`：时间戳，用于配置历史查询。

说明：

- 一个 Group 可以有多条配置历史。
- `exp_group.cfg_id` 指向当前生效版本。

## 默认初始化行为

为了降低建模成本，系统在创建上层对象时会自动补齐默认结构：

- 创建实验时，会自动创建一个默认 Layer。
- 创建 Layer 时，会自动创建一个默认 Segment。
- 创建 Segment 时，会自动创建一个默认 Group。

默认 Group 的初始状态：

- 名称为 `DEFAULT`
- `share = 1000`
- `bitmap` 覆盖全部 1000 个千分桶

因此，新实验在没有额外配置之前也能保持完整流量闭环。

## engine 内存模型

`engine` 从 MySQL 读取启用中的实验，构建内存结构：

- `Application`
- `Experiment`
- `Layer`
- `Segment`
- `Group`

映射关系与数据库基本一致，但有两个运行时增强：

- `experiment.filter` 会被解析成表达式树，直接用于请求过滤。
- `group.force_hit` 会被合并到 `layer.force_hit` 映射，加速强制命中查询。

运行时要点：

- 简单实验：直接在唯一 Layer 的 Group 上按千分桶决策。
- 富实验：先用实验级百分桶选 Segment，再在 Segment 内按千分桶选 Group。

## 设计意图

这个数据模型把两件事拆开了：

- 富实验里，Segment 负责百分桶带来的跨层对齐。
- Group 负责千分桶带来的组间均衡。

同时，简单实验保持最短路径，只保留默认结构，不让 Segment 和百分桶进入决策逻辑。

补充说明见 [doc/traffic-model.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/doc/traffic-model.md)。
