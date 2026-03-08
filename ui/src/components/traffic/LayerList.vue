<template>
  <div class="layer-list">
    <el-collapse v-model="activeLayers">
      <el-collapse-item v-for="(layer, index) in layers" :key="layer.id" :name="layer.id">
        <template #title>
          <span>{{ layer.name || `Layer${index + 1}` }}</span>
        </template>
        <div class="layer-body">
          <div class="layer-meta">
            <el-input v-model="layer.name" placeholder="层名" />
            <el-button size="small" type="primary" @click="handleUpdateLayer(layer)">改名</el-button>
            <el-button size="small" type="danger" @click="handleDeleteLayer(layer)">删除</el-button>
            <div class="layer-meta-right">
              <el-button size="small" type="primary" @click="handleAddSegment(layer)">新增Segment</el-button>
              <el-button size="small" @click="openRebalanceDialog(layer)">调整Segment流量</el-button>
            </div>
          </div>
          <SegmentList :layer="layer" />
        </div>
      </el-collapse-item>
    </el-collapse>

    <el-dialog v-model="dialogVisible" title="新增Layer" width="360px">
      <el-form :model="form" label-width="40px">
        <el-form-item label="名称">
          <el-input v-model="form.name" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleCreate">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="rebalanceVisible" title="调整Segment流量" width="60%">
      <el-table :data="rebalanceSegments" size="small">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column label="Percent">
          <template #default="{ row, $index }">
            <el-input-number v-model="row.percent" :min="0" :max="100" size="small" @change="updateRanges($index)" />
          </template>
        </el-table-column>
        <el-table-column label="Begin">
          <template #default="{ row }">
            <span>{{ row.begin }}</span>
          </template>
        </el-table-column>
        <el-table-column label="End">
          <template #default="{ row }">
            <span>{{ row.end }}</span>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="rebalanceVisible = false">取消</el-button>
        <el-button type="primary" @click="handleRebalance">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { createLyr, updateLyr, deleteLyr, getLayer, createSeg, rebalanceLyr } from '@/api'
import type { Experiment, Layer } from '@/api/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import SegmentList from './SegmentList.vue'

const props = defineProps<{
  experiment: Experiment | null
}>()

const activeLayers = ref<number[]>([])
const layers = ref<Layer[]>([])
const loadedLayerIds = ref(new Set<number>())
const layerNameMap = ref(new Map<number, string>())
const localLayerSyncing = ref(false)

const dialogVisible = ref(false)
const form = ref({ name: '' })

const openLayerDialog = () => {
  form.value = { name: '' }
  dialogVisible.value = true
}

const bumpExperimentVersion = () => {
  if (props.experiment && typeof props.experiment.version === 'number') {
    props.experiment.version += 1
  }
}

const syncLayersToExperiment = () => {
  if (!props.experiment) return
  localLayerSyncing.value = true
  props.experiment.layer = layers.value.map(layer => ({ ...layer }))
}

defineExpose({
  openLayerDialog
})

const loadLayers = () => {
  if (!props.experiment?.layer || props.experiment.layer.length === 0) {
    layers.value = []
    loadedLayerIds.value = new Set()
    layerNameMap.value = new Map()
    return
  }
  layers.value = props.experiment.layer.map(layer => ({
    ...layer,
    segment: (layer.segment || []).map(seg => ({ ...seg }))
  }))
  const map = new Map<number, string>()
  for (const layer of layers.value) {
    map.set(layer.id, layer.name)
  }
  layerNameMap.value = map
}

const updateLayerDetail = (detail: Layer) => {
  layers.value = layers.value.map(layer => (layer.id === detail.id ? detail : layer))
  layerNameMap.value.set(detail.id, detail.name)
}

const fetchLayerDetail = async (layerId: number, force = false) => {
  if (!force && loadedLayerIds.value.has(layerId)) return
  try {
    const res = await getLayer(layerId)
    updateLayerDetail(res.data)
    loadedLayerIds.value.add(layerId)
  } catch (e) {
    // ignore
  }
}

const handleCreate = async () => {
  if (!props.experiment) return
  try {
    const res = await createLyr({
      exp_id: props.experiment.id,
      exp_ver: props.experiment.version!,
      name: form.value.name
    })
    const createdLayer: Layer = {
      ...res.data,
      segment: res.data.segment || []
    }
    layers.value = [...layers.value, createdLayer]
    layerNameMap.value.set(createdLayer.id, createdLayer.name)
    syncLayersToExperiment()
    bumpExperimentVersion()
    ElMessage.success('Layer created')
    dialogVisible.value = false
  } catch (e) {
    // ignore
  }
}

const handleUpdateLayer = async (layer: Layer) => {
  const originalName = layerNameMap.value.get(layer.id)
  if (originalName === layer.name) return
  try {
    await updateLyr(layer.id, {
      name: layer.name,
      version: layer.version
    })
    ElMessage.success('Layer updated')
    layer.version = layer.version + 1
    layerNameMap.value.set(layer.id, layer.name)
  } catch (e) {
    ElMessage.error('更新失败，请手动刷新后重试')
  }
}

const handleDeleteLayer = async (layer: Layer) => {
    if (!props.experiment) return
    try {
        await ElMessageBox.confirm('Delete this layer?', 'Warning', { type: 'warning' })
        await deleteLyr(layer.id, {
            exp_id: props.experiment.id,
            exp_ver: props.experiment.version!,
            version: layer.version!
        })
        layers.value = layers.value.filter(item => item.id !== layer.id)
        activeLayers.value = activeLayers.value.filter(id => id !== layer.id)
        loadedLayerIds.value.delete(layer.id)
        layerNameMap.value.delete(layer.id)
        syncLayersToExperiment()
        bumpExperimentVersion()
        ElMessage.success('Layer deleted')
    } catch (e) {
        // ignore
    }
}

const handleAddSegment = async (layer: Layer) => {
  try {
    const res = await createSeg({
      lyr_id: layer.id,
      lyr_ver: layer.version!
    })
    const nextSegments = [...(layer.segment || []), res.data]
    const nextLayer = {
      ...layer,
      segment: nextSegments,
      version: layer.version + 1
    }
    layers.value = layers.value.map(item => (item.id === layer.id ? nextLayer : item))
    layerNameMap.value.set(nextLayer.id, nextLayer.name)
    syncLayersToExperiment()
    bumpExperimentVersion()
    ElMessage.success('Segment created')
  } catch (e) {
    // ignore
  }
}

interface RebalanceItem {
  id: number
  begin: number
  end: number
  percent: number
}

const rebalanceVisible = ref(false)
const rebalanceSegments = ref<RebalanceItem[]>([])
const rebalanceLayer = ref<Layer | null>(null)

const openRebalanceDialog = (layer: Layer) => {
  rebalanceLayer.value = layer
  rebalanceSegments.value = (layer.segment || []).map(s => ({
    id: s.id,
    begin: s.begin,
    end: s.end,
    percent: s.end - s.begin
  }))
  updateRanges(0)
  rebalanceVisible.value = true
}

const updateRanges = (index: number) => {
  const list = rebalanceSegments.value
  if (list.length === 0) return
  let currentEnd = 0
  for (let i = 0; i < list.length; i++) {
    const item = list[i]
    if (!item) continue
    item.begin = currentEnd
    const safePercent = Math.max(0, Math.min(100 - currentEnd, item.percent || 0))
    item.percent = safePercent
    item.end = item.begin + safePercent
    currentEnd = item.end
  }
}

const handleRebalance = async () => {
  if (!rebalanceLayer.value) return
  try {
    const total = rebalanceSegments.value.reduce((sum, item) => sum + (item?.percent || 0), 0)
    if (total !== 100) {
      ElMessage.error('流量百分比总和需为100')
      return
    }
    await rebalanceLyr(rebalanceLayer.value.id, {
      version: rebalanceLayer.value.version!,
      segment: rebalanceSegments.value
    })
    ElMessage.success('Segments rebalanced')
    const currentSegments = rebalanceLayer.value.segment || []
    const versionMap = new Map(currentSegments.map(seg => [seg.id, seg.version]))
    const nextSegments = rebalanceSegments.value.map(item => ({
      id: item.id,
      begin: item.begin,
      end: item.end,
      version: versionMap.get(item.id) || 0
    }))
    rebalanceLayer.value.segment = nextSegments
    rebalanceLayer.value.version = rebalanceLayer.value.version! + 1
    layers.value = layers.value.map(layer =>
      layer.id === rebalanceLayer.value?.id
        ? { ...layer, segment: nextSegments, version: rebalanceLayer.value.version! }
        : layer
    )
    rebalanceVisible.value = false
  } catch (e) {
    ElMessage.error('调整失败，请手动刷新后重试')
  }
}

watch(
  () => props.experiment?.layer,
  () => {
    if (localLayerSyncing.value) {
      localLayerSyncing.value = false
      return
    }
    loadLayers()
    loadedLayerIds.value = new Set()
    for (const layerId of activeLayers.value) {
      fetchLayerDetail(layerId, true)
    }
  },
  { deep: true, immediate: true }
)

watch(
  () => activeLayers.value,
  (val) => {
    for (const layerId of val) {
      fetchLayerDetail(layerId)
    }
  },
  { deep: true }
)
</script>

<style scoped>
.layer-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.layer-meta {
  display: flex;
  align-items: center;
  gap: 10px;
}
.layer-meta :deep(.el-input) {
  width: 220px;
}
.layer-meta-right {
  margin-left: auto;
  display: flex;
  gap: 10px;
}
.danger {
    color: #f56c6c;
}
</style>
