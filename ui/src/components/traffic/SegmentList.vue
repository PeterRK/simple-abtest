<template>
  <div class="segment-list">
    <div class="segment-grid">
      <div
        v-for="(seg, index) in segments"
        :key="seg.id"
        class="segment-card"
        :class="{ active: selectedSegmentId === seg.id }"
        @click="selectSegment(seg)"
      >
        <div class="segment-main">
          <div class="segment-percent">{{ seg.end - seg.begin }}%</div>
          <el-button
            size="small"
            type="danger"
            :disabled="seg.begin !== seg.end"
            @click.stop="handleDelete(seg)"
          >
            {{ t('common.delete') }}
          </el-button>
        </div>
      </div>
    </div>

    <div v-if="selectedSegmentDetail" class="segment-detail">
      <GroupList :segment="selectedSegmentDetail" @update:segment="handleSegmentUpdate" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { deleteSegment, getSegment } from '@/api'
import type { Layer, Segment } from '@/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import GroupList from './GroupList.vue'
import { useI18n } from '@/i18n'

const props = defineProps<{
  layer: Layer
}>()

const emit = defineEmits<{
  (e: 'update:layer', value: Layer): void
}>()

const segmentDetails = ref<Record<number, Segment>>({})
const segments = computed(() => props.layer.segment || [])
const selectedSegmentId = ref<number | null>(null)
const { t } = useI18n()
const isCancelAction = (error: unknown) => error === 'cancel' || error === 'close'
const selectedSegmentDetail = computed(() =>
  selectedSegmentId.value != null ? segmentDetails.value[selectedSegmentId.value] || null : null
)

const emitLayerUpdate = (nextLayer: Layer) => {
  emit('update:layer', {
    ...nextLayer,
    segment: (nextLayer.segment || []).map(segment => ({ ...segment }))
  })
}

const normalizeSegmentDetail = (segment: Segment, fallback?: Segment): Segment => ({
  ...segment,
  begin: segment.begin ?? fallback?.begin ?? 0,
  end: segment.end ?? fallback?.end ?? 0,
  version: segment.version ?? fallback?.version ?? 0,
  group: (segment.group || []).map(grp => ({
    ...grp,
    version: grp.version ?? 0
  }))
})

const syncSegmentSummaryVersion = (segmentId: number, version: number) => {
  const list = props.layer.segment || []
  const idx = list.findIndex(item => item.id === segmentId)
  if (idx < 0) return
  const target = list[idx]
  if (!target) return
  if (target.version === version) return
  const next = [...list]
  next[idx] = { ...target, version }
  emitLayerUpdate({
    ...props.layer,
    segment: next
  })
}

const selectSegment = (seg: Segment) => {
  selectedSegmentId.value = seg.id
  if (segmentDetails.value[seg.id]) return
  const cachedSummary = (props.layer.segment || []).find(item => item.id === seg.id)
  if (cachedSummary) {
    segmentDetails.value = {
      ...segmentDetails.value,
      [seg.id]: normalizeSegmentDetail(cachedSummary)
    }
  }
  getSegment(seg.id)
    .then(res => {
      const detail = normalizeSegmentDetail(res.data, cachedSummary)
      segmentDetails.value = { ...segmentDetails.value, [seg.id]: detail }
      syncSegmentSummaryVersion(seg.id, detail.version)
    })
    .catch(() => {
      ElMessage.error(t('message.failedLoadSegment'))
    })
}

const autoSelectSingleSegment = () => {
  if (segments.value.length !== 1) return
  const onlySegment = segments.value[0]
  if (!onlySegment) return
  selectSegment(onlySegment)
}

const handleDelete = async (seg: Segment) => {
    try {
        await ElMessageBox.confirm(t('confirm.deleteSegment'), t('common.warning'), { type: 'warning' })
        await deleteSegment(seg.id, {
            lyr_id: props.layer.id,
            lyr_ver: props.layer.version!,
            version: seg.version!
        })
        emitLayerUpdate({
          ...props.layer,
          version: typeof props.layer.version === 'number' ? props.layer.version + 1 : 1,
          segment: (props.layer.segment || []).filter(item => item.id !== seg.id)
        })
        if (selectedSegmentId.value === seg.id) {
          selectedSegmentId.value = null
        }
        const nextDetails = { ...segmentDetails.value }
        delete nextDetails[seg.id]
        segmentDetails.value = nextDetails
        ElMessage.success(t('message.segmentDeleted'))
    } catch (e) {
        if (!isCancelAction(e)) ElMessage.error(t('message.deleteFailed'))
    }
}

watch(
  () => props.layer.segment,
  () => {
    if (!segments.value.some(seg => seg.id === selectedSegmentId.value)) {
      selectedSegmentId.value = null
    }
    const nextSegmentDetails: Record<number, Segment> = {}
    for (const seg of segments.value) {
      const cached = segmentDetails.value[seg.id]
      if (cached) {
        nextSegmentDetails[seg.id] = normalizeSegmentDetail(cached, seg)
      }
    }
    segmentDetails.value = nextSegmentDetails
    if (selectedSegmentId.value != null && !segmentDetails.value[selectedSegmentId.value]) {
      const selectedSummary = segments.value.find(seg => seg.id === selectedSegmentId.value)
      if (selectedSummary) {
        selectSegment(selectedSummary)
      }
    }
    autoSelectSingleSegment()
  },
  { deep: true, immediate: true }
)
watch(
  () => props.layer.id,
  () => {
    selectedSegmentId.value = null
    segmentDetails.value = {}
    autoSelectSingleSegment()
  }
)

watch(
  () => [selectedSegmentId.value, selectedSegmentDetail.value?.version] as const,
  ([segmentId, version]) => {
    if (segmentId == null || typeof version !== 'number') return
    syncSegmentSummaryVersion(segmentId, version)
  }
)

const handleSegmentUpdate = (nextSegment: Segment) => {
  segmentDetails.value = {
    ...segmentDetails.value,
    [nextSegment.id]: nextSegment
  }
  emitLayerUpdate({
    ...props.layer,
    segment: (props.layer.segment || []).map(segment =>
      segment.id === nextSegment.id ? { ...nextSegment } : { ...segment }
    )
  })
}
</script>

<style scoped>
.segment-grid {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.segment-card {
  flex: 1 1 180px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  cursor: pointer;
  transition: border-color 0.15s, background-color 0.15s;
}
.segment-card.active {
  border-color: #409eff;
  background-color: #ecf5ff;
  box-shadow: 0 0 0 1px #409eff inset;
}
.segment-main {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}
.segment-percent {
  font-weight: 600;
}
.segment-detail {
  margin-top: 16px;
}
</style>
