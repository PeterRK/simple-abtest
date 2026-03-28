# 流量模型说明

## 总览

`simple-abtest`的流量模型分成两套桶位：

- 百分桶：`[0,100)`，用于跨Layer对齐Segment。
- 千分桶：`[0,1000)`，用于在同一个Segment内平衡各Group的份额。

这里的重点不是“粗粒度”和“细粒度”，而是两者承担不同职责：

- 百分桶负责跨层对齐。
- 千分桶负责组间均衡。

同时要注意一个前提：只有富实验才会真正使用百分桶。简单实验只有一个Layer，此时`Segment`和百分桶都不起作用，实际只使用千分桶。

## 百分桶的作用

百分桶由实验级`seed`和请求`key`共同计算得到：

- 输出范围是`0`到`99`。
- 结果用于确定用户在实验级入口上命中哪个Segment区间。
- 同一个实验下，各Layer都使用这套百分桶结果来保持Segment对齐。

它解决的问题是：

- 多个Layer如何共享同一批流量段。
- 富实验里如何让不同配置维度在同一批用户上同时生效。
- 调整Segment区间时，如何保持跨Layer的命中一致性。

示例：

```text
Segment A: [0, 20)
Segment B: [20, 100)
```

这表示：

- 百分桶落在`[0,20)`的用户，会在所有相关Layer上进入同一段Segment。
- 百分桶落在`[20,100)`的用户，会在所有相关Layer上进入另一段Segment。

这里的关键不是“大盘切片更粗”，而是“跨Layer用同一套分段入口”。

## 千分桶的作用

千分桶由Segment级`seed`和请求`key`共同计算得到：

- 输出范围是`0`到`999`。
- 每个Group用一个1000位的位图表示自己持有的桶位。
- `share`表示该Group当前占有多少个千分桶。

它解决的问题是：

- 同一个决策段内控制组和实验组如何分配份额。
- 多个Group之间如何保持总量守恒的份额均衡。
- 调整某个Group的占比时，如何只影响当前分组结果。

示例：

```text
DEFAULT: 700 / 1000
variantA: 300 / 1000
```

这表示：

- 命中当前分组决策段的用户里，70% 命中默认组。
- 30% 命中`variantA`。

这里的关键不是“更细”，而是“在固定决策范围内做份额均衡”。

## 为什么要分成百分桶和千分桶

如果只用一层桶位，会把两类问题混在一起：

- 跨Layer对齐会和组间份额调整耦合。
- 一次组内配比修改可能破坏原有的跨层一致性。
- 一次跨层Segment调整又可能误伤组内均衡。

拆成两层之后：

- 百分桶负责跨层对齐。
- 千分桶负责组间均衡。
- Segment区间和Group份额可以分开维护。

## 代码中的对应关系

### 1. 百分桶

在富实验场景下，代码会先使用实验级`seed`计算百分桶：

- 位置：`engine/core/dispatch.go`
- 逻辑：`Hash(experiment.seed, key) % 100`

这一步决定的是跨Layer共用的Segment入口。

### 2. 千分桶

进入具体分组决策后，再使用Segment级`seed`计算千分桶：

- 位置：`engine/core/dispatch.go`
- 逻辑：`Hash(segment.seed, key) % 1000`

这一步决定的是当前分组范围内各Group的份额归属。

## 简单实验和富实验

代码里有两种执行路径：

### 简单实验

条件：

- 只有一个Layer
- 这个Layer只有一个Segment

此时：

- `Segment`只是数据结构上的默认层级，不参与路由。
- 百分桶不参与决策。
- 代码直接在这个唯一Layer的分组范围内按千分桶选择Group。

适合：

- 普通控制组 / 实验组分流
- 不需要跨层对齐的实验

### 富实验

条件：

- 有多个Layer，或者
- 某个Layer下有多个Segment

此时先走百分桶完成跨层对齐，再走千分桶完成组间均衡。

适合：

- 多层配置联动
- 需要多个Layer命中同一段流量的实验
- 需要在固定Segment内继续分配组份额的实验

## 调整流量时该改哪里

遵循这条规则最安全：

- 想调整跨Layer对齐关系，改`Segment range`。
- 想调整Group之间的份额均衡，改`Group share/bitmap`。

不要混用：

- 不要用新增很多Group的方式替代Segment对齐。
- 不要用频繁改Segment边界的方式替代组内份额调整。
- 简单实验里不要把`Segment`当成有效调度手段，它不会进入决策逻辑。

## 默认组的作用

每个Segment至少有一个默认组：

- 默认组初始占满全部1000个千分桶。
- 新建实验组时，初始`share=0`。
- 通过重平衡，把默认组持有的一部分桶位转给目标组。

这保证了：

- 当前分组范围内总是能完整覆盖全部份额。
- 组删除时不会留下未归属桶位。

## 强制命中

在正常哈希分流之前，Layer还支持`force_hit`：

- 指定某些key直接命中某个Group。
- 命中后跳过正常的百分桶和千分桶决策。

适合：

- 联调验收
- 定向灰度
- 问题复现

## 建模建议

- 单纯A/B对照实验：保留默认结构即可，实际只用千分桶调份额。
- 需要多个Layer对齐命中：使用百分桶维护Segment区间，再在每个Segment内用千分桶分配Group。
- 多个配置维度彼此独立输出：用多个Layer。

## 相关文档

- [README.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/README.md)
- [doc/data-model.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/doc/data-model.md)
- [doc/engine-api.md](/d:/GoSpace/projects/src/github.com/peterrk/simple-abtest/doc/engine-api.md)

