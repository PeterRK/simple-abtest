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
          <SegmentList :layer="layer" @refresh="$emit('refresh')" />
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
        <el-table-column label="Begin">
          <template #default="{ row }">
            <span>{{ row.begin }}</span>
          </template>
        </el-table-column>
        <el-table-column label="End">
          <template #default="{ row, $index }">
            <el-input-number v-model="row.end" :min="row.begin" :max="100" size="small" @change="updateRanges($index)" />
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

const emit = defineEmits(['refresh'])

const activeLayers = ref<number[]>([])
const layers = ref<Layer[]>([])
const loadedLayerIds = ref(new Set<number>())

const dialogVisible = ref(false)
const form = ref({ name: '' })

const openLayerDialog = () => {
  form.value = { name: '' }
  dialogVisible.value = true
}

defineExpose({
  openLayerDialog
})

const loadLayers = () => {
  if (!props.experiment?.layer || props.experiment.layer.length === 0) {
    layers.value = []
    loadedLayerIds.value = new Set()
    return
  }
  layers.value = props.experiment.layer.map(layer => ({
    id: layer.id,
    name: layer.name,
    version: layer.version,
    exp_id: layer.exp_id,
    exp_ver: layer.exp_ver
  }))
}

const updateLayerDetail = (detail: Layer) => {
  layers.value = layers.value.map(layer => (layer.id === detail.id ? detail : layer))
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
    await createLyr({
      exp_id: props.experiment.id,
      exp_ver: props.experiment.version!,
      name: form.value.name
    })
    ElMessage.success('Layer created')
    dialogVisible.value = false
    emit('refresh')
  } catch (e) {
    // ignore
  }
}

const handleUpdateLayer = async (layer: Layer) => {
  try {
    await updateLyr(layer.id, {
      name: layer.name,
      version: layer.version
    })
    ElMessage.success('Layer updated')
    emit('refresh')
  } catch (e) {
    // ignore
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
        ElMessage.success('Layer deleted')
        emit('refresh')
    } catch (e) {
        // ignore
    }
}

const handleAddSegment = async (layer: Layer) => {
  try {
    await createSeg({
      lyr_id: layer.id,
      lyr_ver: layer.version!
    })
    ElMessage.success('Segment created')
    await fetchLayerDetail(layer.id, true)
  } catch (e) {
    // ignore
  }
}

interface RebalanceItem {
  id: number
  begin: number
  end: number
}

const rebalanceVisible = ref(false)
const rebalanceSegments = ref<RebalanceItem[]>([])
const rebalanceLayer = ref<Layer | null>(null)

const openRebalanceDialog = (layer: Layer) => {
  rebalanceLayer.value = layer
  rebalanceSegments.value = (layer.segment || []).map(s => ({ id: s.id, begin: s.begin, end: s.end }))
  rebalanceVisible.value = true
}

const updateRanges = (index: number) => {
  const list = rebalanceSegments.value
  if (!list[index]) return
  let currentEnd = list[index].end
  for (let i = index + 1; i < list.length; i++) {
    const item = list[i]
    if (!item) continue
    item.begin = currentEnd
    if (item.end < currentEnd) {
      item.end = currentEnd
    }
    currentEnd = item.end
  }
}

const handleRebalance = async () => {
  if (!rebalanceLayer.value) return
  try {
    await rebalanceLyr(rebalanceLayer.value.id, {
      version: rebalanceLayer.value.version!,
      segment: rebalanceSegments.value
    })
    ElMessage.success('Segments rebalanced')
    rebalanceVisible.value = false
    emit('refresh')
  } catch (e) {
    // ignore
  }
}

watch(
  () => props.experiment?.layer,
  () => {
    loadLayers()
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
