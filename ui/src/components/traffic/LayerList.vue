<template>
  <div class="layer-list">
    <el-collapse v-model="activeLayers">
      <el-collapse-item v-for="(layer, index) in layers" :key="layer.id" :name="layer.id">
        <template #title>
          <span>{{ layer.name || t('layer.fallbackName', { index: index + 1 }) }}</span>
        </template>
        <div class="layer-body">
          <div class="layer-meta">
            <el-input
              v-model="layer.name"
              :placeholder="t('layer.namePlaceholder')"
              :class="{ 'dirty-input': isLayerNameDirty(layer) }"
            />
            <el-button size="small" type="primary" :disabled="!isLayerNameDirty(layer)" @click="handleUpdateLayer(layer)">{{ t('layer.rename') }}</el-button>
            <el-button size="small" type="danger" @click="handleDeleteLayer(layer)">{{ t('common.delete') }}</el-button>
            <div class="layer-meta-right">
              <el-button size="small" type="primary" @click="handleAddSegment(layer)">{{ t('layer.addSegment') }}</el-button>
              <el-button size="small" @click="openRebalanceDialog(layer)">{{ t('layer.rebalanceSegment') }}</el-button>
            </div>
          </div>
          <SegmentList :layer="layer" @update:layer="handleLayerUpdate" />
        </div>
      </el-collapse-item>
    </el-collapse>

    <el-dialog v-model="dialogVisible" :title="t('layer.createTitle')" width="360px">
      <el-form :model="form" label-width="40px">
        <el-form-item :label="t('common.name')">
          <el-input v-model="form.name" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleCreate">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="rebalanceVisible" :title="t('layer.rebalanceTitle')" width="60%">
      <el-table :data="rebalanceSegments" size="small">
        <el-table-column prop="id" :label="t('common.id')" width="80" />
        <el-table-column :label="t('layer.percent')">
          <template #default="{ row, $index }">
            <el-input-number v-model="row.percent" :min="0" :max="100" size="small" @change="updateRanges($index)" />
          </template>
        </el-table-column>
        <el-table-column :label="t('layer.begin')">
          <template #default="{ row }">
            <span>{{ row.begin }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('layer.end')">
          <template #default="{ row }">
            <span>{{ row.end }}</span>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="rebalanceVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleRebalance">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { createLayer, updateLayer, deleteLayer, getLayer, createSegment, rebalanceLayer } from '@/api'
import type { Experiment, Layer } from '@/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import SegmentList from './SegmentList.vue'
import { useI18n } from '@/i18n'

const props = defineProps<{
  experiment: Experiment | null
}>()

const emit = defineEmits<{
  (e: 'update:layers', value: Layer[]): void
  (e: 'bump-experiment-version'): void
}>()

const activeLayers = ref<number[]>([])
const layers = ref<Layer[]>([])
const loadedLayerIds = ref(new Set<number>())
const layerNameMap = ref(new Map<number, string>())
const localLayerSyncing = ref(false)
const { t } = useI18n()

const dialogVisible = ref(false)
const form = ref({ name: '' })
const isCancelAction = (error: unknown) => error === 'cancel' || error === 'close'

const openLayerDialog = () => {
  form.value = { name: '' }
  dialogVisible.value = true
}

const normalizeLayer = (layer: Layer): Layer => ({
  ...layer,
  segment: (layer.segment || []).map(seg => ({
    ...seg,
    version: seg.version ?? 0
  }))
})

const emitLayersUpdate = (nextLayers: Layer[]) => {
  const normalizedLayers = nextLayers.map(normalizeLayer)
  layers.value = normalizedLayers
  localLayerSyncing.value = true
  emit('update:layers', normalizedLayers.map(layer => ({ ...layer })))
}

defineExpose({
  openLayerDialog
})

const loadLayers = () => {
  if (!props.experiment?.layer || props.experiment.layer.length === 0) {
    layers.value = []
    loadedLayerIds.value = new Set()
    layerNameMap.value = new Map()
    activeLayers.value = []
    return
  }
  const currentActiveSet = new Set(activeLayers.value)
  layers.value = props.experiment.layer.map(normalizeLayer)
  const validLayerIds = new Set(layers.value.map(layer => layer.id))
  const nextActive = activeLayers.value.filter(layerId => validLayerIds.has(layerId))
  if (layers.value.length === 1 && nextActive.length === 0) {
    const onlyLayer = layers.value[0]
    if (onlyLayer) nextActive.push(onlyLayer.id)
  }
  activeLayers.value = nextActive
  const map = new Map<number, string>()
  for (const layer of layers.value) {
    map.set(layer.id, layer.name)
  }
  layerNameMap.value = map
  loadedLayerIds.value = new Set(
    Array.from(currentActiveSet).filter(layerId => validLayerIds.has(layerId))
  )
}

const updateLayerDetail = (detail: Layer) => {
  const normalizedDetail = normalizeLayer(detail)
  emitLayersUpdate(layers.value.map(layer => (layer.id === normalizedDetail.id ? normalizedDetail : layer)))
  layerNameMap.value.set(normalizedDetail.id, normalizedDetail.name)
}

const isLayerNameDirty = (layer: Layer) => layerNameMap.value.get(layer.id) !== layer.name

const fetchLayerDetail = async (layerId: number, force = false) => {
  if (!force && loadedLayerIds.value.has(layerId)) return
  try {
    const res = await getLayer(layerId)
    updateLayerDetail(res.data)
    loadedLayerIds.value.add(layerId)
  } catch (e) {
    ElMessage.error(t('message.failedLoadLayer'))
  }
}

const handleCreate = async () => {
  if (!props.experiment) return
  try {
    const res = await createLayer({
      exp_id: props.experiment.id,
      exp_ver: props.experiment.version!,
      name: form.value.name
    })
    const createdLayer: Layer = {
      ...res.data,
      segment: res.data.segment || []
    }
    emitLayersUpdate([...layers.value, createdLayer])
    layerNameMap.value.set(createdLayer.id, createdLayer.name)
    emit('bump-experiment-version')
    ElMessage.success(t('message.layerCreated'))
    dialogVisible.value = false
  } catch (e) {
    ElMessage.error(t('message.createFailed'))
  }
}

const handleUpdateLayer = async (layer: Layer) => {
  const originalName = layerNameMap.value.get(layer.id)
  if (originalName === layer.name) return
  try {
    await updateLayer(layer.id, {
      name: layer.name,
      version: layer.version
    })
    ElMessage.success(t('message.layerUpdated'))
    const nextLayer = { ...layer, version: layer.version + 1 }
    emitLayersUpdate(layers.value.map(item => (item.id === nextLayer.id ? nextLayer : item)))
    layerNameMap.value.set(nextLayer.id, nextLayer.name)
  } catch (e) {
    ElMessage.error(t('message.updateFailedRefresh'))
  }
}

const handleDeleteLayer = async (layer: Layer) => {
    if (!props.experiment) return
    try {
        await ElMessageBox.confirm(t('confirm.deleteLayer'), t('common.warning'), { type: 'warning' })
        await deleteLayer(layer.id, {
            exp_id: props.experiment.id,
            exp_ver: props.experiment.version!,
            version: layer.version!
        })
        emitLayersUpdate(layers.value.filter(item => item.id !== layer.id))
        activeLayers.value = activeLayers.value.filter(id => id !== layer.id)
        loadedLayerIds.value.delete(layer.id)
        layerNameMap.value.delete(layer.id)
        emit('bump-experiment-version')
        ElMessage.success(t('message.layerDeleted'))
    } catch (e) {
        if (!isCancelAction(e)) ElMessage.error(t('message.deleteFailed'))
    }
}

const handleAddSegment = async (layer: Layer) => {
  try {
    const res = await createSegment({
      lyr_id: layer.id,
      lyr_ver: layer.version!
    })
    const nextSegments = [...(layer.segment || []), { ...res.data, version: res.data.version ?? 0 }]
    const nextLayer = {
      ...layer,
      segment: nextSegments,
      version: layer.version + 1
    }
    emitLayersUpdate(layers.value.map(item => (item.id === layer.id ? nextLayer : item)))
    layerNameMap.value.set(nextLayer.id, nextLayer.name)
    ElMessage.success(t('message.segmentCreated'))
  } catch (e) {
    ElMessage.error(t('message.createFailed'))
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
const rebalanceTargetLayer = ref<Layer | null>(null)

const openRebalanceDialog = (layer: Layer) => {
  rebalanceTargetLayer.value = layer
  rebalanceSegments.value = (layer.segment || []).map(s => ({
    id: s.id,
    begin: s.begin,
    end: s.end,
    percent: s.end - s.begin
  }))
  updateRanges(0)
  rebalanceVisible.value = true
}

const updateRanges = (_index: number) => {
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
  if (!rebalanceTargetLayer.value) return
  try {
    const total = rebalanceSegments.value.reduce((sum, item) => sum + (item?.percent || 0), 0)
    if (total !== 100) {
      ElMessage.error(t('message.sumShareMust100'))
      return
    }
    await rebalanceLayer(rebalanceTargetLayer.value.id, {
      version: rebalanceTargetLayer.value.version!,
      segment: rebalanceSegments.value
    })
    ElMessage.success(t('message.segmentsRebalanced'))
    const currentSegments = rebalanceTargetLayer.value.segment || []
    const versionMap = new Map(currentSegments.map(seg => [seg.id, seg.version]))
    const nextSegments = rebalanceSegments.value.map(item => ({
      id: item.id,
      begin: item.begin,
      end: item.end,
      version: versionMap.get(item.id) || 0
    }))
    const nextLayer: Layer = {
      ...rebalanceTargetLayer.value,
      segment: nextSegments,
      version: rebalanceTargetLayer.value.version! + 1
    }
    rebalanceTargetLayer.value = nextLayer
    emitLayersUpdate(layers.value.map(layer => (layer.id === nextLayer.id ? nextLayer : layer)))
    rebalanceVisible.value = false
  } catch (e) {
    ElMessage.error(t('message.rebalanceFailedRefresh'))
  }
}

const handleLayerUpdate = (nextLayer: Layer) => {
  emitLayersUpdate(layers.value.map(layer => (layer.id === nextLayer.id ? normalizeLayer(nextLayer) : layer)))
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
  }
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
