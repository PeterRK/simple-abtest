<template>
  <div class="segment-list">
    <div class="header">
      <h4>Segments</h4>
      <div class="actions">
        <el-button size="small" type="primary" @click="handleAddSegment">Add Segment</el-button>
        <el-button size="small" @click="openRebalanceDialog">Rebalance Segments</el-button>
      </div>
    </div>

    <!-- Visual Bar -->
    <div class="segment-bar">
        <div v-for="seg in segments" :key="seg.id" 
             class="segment-block"
             :style="{ width: (seg.end - seg.begin) + '%', backgroundColor: getColor(seg.id) }"
             :title="`ID: ${seg.id}, Range: [${seg.begin}, ${seg.end})`">
             {{ seg.begin }}-{{ seg.end }}
        </div>
    </div>

    <el-collapse accordion>
      <el-collapse-item v-for="seg in segments" :key="seg.id" :name="seg.id">
        <template #title>
           <div class="segment-title">
             <span>Segment {{ seg.id }} [{{ seg.begin }}, {{ seg.end }})</span>
             <div class="segment-actions" @click.stop>
               <el-button type="text" @click="handleShuffle(seg)">Shuffle</el-button>
               <el-button type="text" class="danger" v-if="seg.begin === seg.end" @click="handleDelete(seg)">Delete</el-button>
             </div>
           </div>
        </template>
        <GroupList :segment="seg" @refresh="$emit('refresh')" />
      </el-collapse-item>
    </el-collapse>

    <!-- Rebalance Dialog -->
    <el-dialog v-model="rebalanceVisible" title="Rebalance Segments" width="60%">
        <p>Adjust segment ranges. Must be contiguous and cover [0, 100).</p>
        <el-table :data="rebalanceSegments" size="small">
            <el-table-column prop="id" label="ID" width="80" />
            <el-table-column label="Begin">
                <template #default="{ row }">
                    <!-- Begin is dependent on prev End, except first is 0 -->
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
            <el-button @click="rebalanceVisible = false">Cancel</el-button>
            <el-button type="primary" @click="handleRebalance">Confirm</el-button>
        </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { createSeg, deleteSeg, shuffleSeg, rebalanceLyr, getSegment } from '@/api'
import type { Layer, Segment } from '@/api/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import GroupList from './GroupList.vue'

const props = defineProps<{
  layer: Layer
}>()

const emit = defineEmits(['refresh'])

const segmentDetails = ref<Segment[]>([])
const segments = computed(() => segmentDetails.value)

// Random colors for visualization
const colors = ['#409EFF', '#67C23A', '#E6A23C', '#F56C6C', '#909399', '#36cfc9', '#9254de', '#f759ab']
const getColor = (id: number) => {
    return colors[id % colors.length]
}

const handleAddSegment = async () => {
    try {
        await createSeg({
            lyr_id: props.layer.id,
            lyr_ver: props.layer.version!
        })
        ElMessage.success('Segment created (Range [100, 100))')
        emit('refresh')
    } catch (e) {
        // ignore
    }
}

const loadSegments = async () => {
  if (!props.layer.segment || props.layer.segment.length === 0) {
    segmentDetails.value = []
    return
  }
  const results = await Promise.all(
    props.layer.segment.map(async seg => {
      try {
        const res = await getSegment(seg.id)
        return res.data
      } catch (e) {
        return seg
      }
    })
  )
  segmentDetails.value = results as Segment[]
}

const handleDelete = async (seg: Segment) => {
    try {
        await ElMessageBox.confirm('Delete this segment? Range must be empty.', 'Warning', { type: 'warning' })
        await deleteSeg(seg.id, {
            lyr_id: props.layer.id,
            lyr_ver: props.layer.version!,
            version: seg.version!
        })
        ElMessage.success('Segment deleted')
        emit('refresh')
    } catch (e) {
        // ignore
    }
}

const handleShuffle = async (seg: Segment) => {
    try {
        await shuffleSeg(seg.id)
        ElMessage.success('Segment seed shuffled')
    } catch (e) {
        // ignore
    }
}

watch(
  () => props.layer.segment,
  () => {
    loadSegments()
  },
  { deep: true, immediate: true }
)

// Rebalance
interface RebalanceItem {
    id: number
    begin: number
    end: number
}

const rebalanceVisible = ref(false)
const rebalanceSegments = ref<RebalanceItem[]>([])

const openRebalanceDialog = () => {
    // Clone segments for editing
    rebalanceSegments.value = segments.value.map(s => ({ id: s.id, begin: s.begin, end: s.end }))
    rebalanceVisible.value = true
}

const updateRanges = (index: number) => {
    // Propagate end to next begin
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
    try {
        await rebalanceLyr(props.layer.id, {
            version: props.layer.version!,
            segment: rebalanceSegments.value
        })
        ElMessage.success('Segments rebalanced')
        rebalanceVisible.value = false
        emit('refresh')
    } catch (e) {
        // ignore
    }
}
</script>

<style scoped>
.header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 10px;
}
.segment-bar {
    display: flex;
    height: 24px;
    background-color: #eee;
    border-radius: 4px;
    overflow: hidden;
    margin-bottom: 10px;
}
.segment-block {
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 10px;
    color: #fff;
    cursor: default;
    overflow: hidden;
    white-space: nowrap;
}
.segment-title {
    display: flex;
    justify-content: space-between;
    width: 100%;
    padding-right: 10px;
}
.danger {
    color: #f56c6c;
}
</style>
