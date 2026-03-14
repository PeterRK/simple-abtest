# 流量模型说明

## 总览

`simple-abtest` 的流量模型分成两套桶位：

- 百分桶：`[0,100)`，用于跨 Layer 对齐 Segment。
- 千分桶：`[0,1000)`，用于在同一个 Segment 内平衡各 Group 的份额。

这里的重点不是“粗粒度”和“细粒度”，而是两者承担不同职责：

- 百分桶负责跨层对齐。
- 千分桶负责组间均衡。

同时要注意一个前提：只有富实验才会真正使用百分桶。简单实验只有一个 Layer，此时 `Segment` 和百分桶都不起作用，实际只使用千分桶。

## 百分桶的作用

百分桶由实验级 `seed` 和请求 `key` 共同计算得到：

- 输出范围是 `0` 到 `99`。
- 结果用于确定用户在实验级入口上命中哪个 Segment 区间。
- 同一个实验下，各 Layer 都使用这套百分桶结果来保持 Segment 对齐。

它解决的问题是：

- 多个 Layer 如何共享同一批流量段。
- 富实验里如何让不同配置维度在同一批用户上同时生效。
- 调整 Segment 区间时，如何保持跨 Layer 的命中一致性。

示例：

```text
Segment A: [0, 20)
Segment B: [20, 100)
```

这表示：

- 百分桶落在 `[0,20)` 的用户，会在所有相关 Layer 上进入同一段 Segment。
- 百分桶落在 `[20,100)` 的用户，会在所有相关 Layer 上进入另一段 Segment。

这里的关键不是“大盘切片更粗”，而是“跨 Layer 用同一套分段入口”。

## 千分桶的作用

千分桶由 Segment 级 `seed` 和请求 `key` 共同计算得到：

- 输出范围是 `0` 到 `999`。
- 每个 Group 用一个 1000 位的位图表示自己持有的桶位。
- `share` 表示该 Group 当前占有多少个千分桶。

它解决的问题是：

- 同一个决策段内控制组和实验组如何分配份额。
- 多个 Group 之间如何保持总量守恒的份额均衡。
- 调整某个 Group 的占比时，如何只影响当前分组结果。

示例：

```text
DEFAULT: 700 / 1000
variantA: 300 / 1000
```

这表示：

- 命中当前分组决策段的用户里，70% 命中默认组。
- 30% 命中 `variantA`。

这里的关键不是“更细”，而是“在固定决策范围内做份额均衡”。

## 为什么要分成百分桶和千分桶

如果只用一层桶位，会把两类问题混在一起：

- 跨 Layer 对齐会和组间份额调整耦合。
- 一次组内配比修改可能破坏原有的跨层一致性。
- 一次跨层 Segment 调整又可能误伤组内均衡。

拆成两层之后：

- 百分桶负责跨层对齐。
- 千分桶负责组间均衡。
- Segment 区间和 Group 份额可以分开维护。

## 代码中的对应关系

### 1. 百分桶

在富实验场景下，代码会先使用实验级 `seed` 计算百分桶：

- 位置：`engine/core/dispatch.go`
- 逻辑：`Hash(experiment.seed, key) % 100`

这一步决定的是跨 Layer 共用的 Segment 入口。

### 2. 千分桶

进入具体分组决策后，再使用 Segment 级 `seed` 计算千分桶：

- 位置：`engine/core/dispatch.go`
- 逻辑：`Hash(segment.seed, key) % 1000`

这一步决定的是当前分组范围内各 Group 的份额归属。

## 简单实验和富实验

代码里有两种执行路径：

### 简单实验

条件：

- 只有一个 Layer
- 这个 Layer 只有一个 Segment

此时：

- `Segment` 只是数据结构上的默认层级，不参与路由。
- 百分桶不参与决策。
- 代码直接在这个唯一 Layer 的分组范围内按千分桶选择 Group。

适合：

- 普通控制组 / 实验组分流
- 不需要跨层对齐的实验

### 富实验

条件：

- 有多个 Layer，或者
- 某个 Layer 下有多个 Segment

此时先走百分桶完成跨层对齐，再走千分桶完成组间均衡。

适合：

- 多层配置联动
- 需要多个 Layer 命中同一段流量的实验
- 需要在固定 Segment 内继续分配组份额的实验

## 调整流量时该改哪里

遵循这条规则最安全：

- 想调整跨 Layer 对齐关系，改 `Segment range`。
- 想调整 Group 之间的份额均衡，改 `Group share/bitmap`。

不要混用：

- 不要用新增很多 Group 的方式替代 Segment 对齐。
- 不要用频繁改 Segment 边界的方式替代组内份额调整。
- 简单实验里不要把 `Segment` 当成有效调度手段，它不会进入决策逻辑。

## 默认组的作用

每个 Segment 至少有一个默认组：

- 默认组初始占满全部 1000 个千分桶。
- 新建实验组时，初始 `share=0`。
- 通过重平衡，把默认组持有的一部分桶位转给目标组。

这保证了：

- 当前分组范围内总是能完整覆盖全部份额。
- 组删除时不会留下未归属桶位。

## 强制命中

在正常哈希分流之前，Layer 还支持 `force_hit`：

- 指定某些 key 直接命中某个 Group。
- 命中后跳过正常的百分桶和千分桶决策。

适合：

- 联调验收
- 定向灰度
- 问题复现

## 建模建议

- 单纯 A/B 对照实验：保留默认结构即可，实际只用千分桶调份额。
- 需要多个 Layer 对齐命中：使用百分桶维护 Segment 区间，再在每个 Segment 内用千分桶分配 Group。
- 多个配置维度彼此独立输出：用多个 Layer。

## 相关文档

- [README.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/README.md)
- [doc/data-model.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/doc/data-model.md)
- [doc/engine-api.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/doc/engine-api.md)
