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
      <GroupList :segment="selectedSegmentDetail" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { deleteSeg, getSegment } from '@/api'
import type { Layer, Segment } from '@/api/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import GroupList from './GroupList.vue'
import { useI18n } from '@/i18n'

const props = defineProps<{
  layer: Layer
}>()

const segmentDetails = ref<Record<number, Segment>>({})
const segments = computed(() => props.layer.segment || [])
const selectedSegmentId = ref<number | null>(null)
const { t } = useI18n()
const selectedSegmentDetail = computed(() =>
  selectedSegmentId.value != null ? segmentDetails.value[selectedSegmentId.value] || null : null
)

const selectSegment = (seg: Segment) => {
  selectedSegmentId.value = seg.id
  if (!segmentDetails.value[seg.id]) {
    getSegment(seg.id)
      .then(res => {
        segmentDetails.value = { ...segmentDetails.value, [seg.id]: res.data }
      })
      .catch(() => {
        // ignore
      })
  }
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
        await deleteSeg(seg.id, {
            lyr_id: props.layer.id,
            lyr_ver: props.layer.version!,
            version: seg.version!
        })
        props.layer.segment = (props.layer.segment || []).filter(item => item.id !== seg.id)
        if (typeof props.layer.version === 'number') {
          props.layer.version += 1
        }
        if (selectedSegmentId.value === seg.id) {
          selectedSegmentId.value = null
        }
        const nextDetails = { ...segmentDetails.value }
        delete nextDetails[seg.id]
        segmentDetails.value = nextDetails
        ElMessage.success(t('message.segmentDeleted'))
    } catch (e) {
        // ignore
    }
}

watch(
  () => props.layer.segment,
  () => {
    if (!segments.value.some(seg => seg.id === selectedSegmentId.value)) {
      selectedSegmentId.value = null
    }
    segmentDetails.value = {}
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
}
.segment-card.active {
  border-color: #409eff;
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
