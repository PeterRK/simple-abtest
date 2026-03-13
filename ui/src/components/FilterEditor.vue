<template>
  <div class="filter-editor">
    <div v-if="root" class="tree-container">
      <FilterNode :node="root" :indent="0" @remove="handleRootRemove" @change="commitTreeChange" />
    </div>
    <div v-else class="empty-state">
      <span class="empty-text">{{ t('filter.empty') }}</span>
      <el-button size="small" @click="createRoot">{{ t('filter.addRoot') }}</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import FilterNode from './FilterNode.vue'
import { flatToTree, treeToFlat, serializeExprNodes, type ExprNode, type TreeNode } from '@/utils/filter'
import { useI18n } from '@/i18n'

const props = defineProps<{
  modelValue: ExprNode[] | undefined
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', val: ExprNode[]): void
}>()

const { t } = useI18n()

const root = ref<TreeNode | null>(null)
const lastCommittedSignature = ref('')

watch(
  () => props.modelValue,
  (val) => {
    const nextSignature = serializeExprNodes(val)
    if (nextSignature === lastCommittedSignature.value) return
    lastCommittedSignature.value = nextSignature
    root.value = val && val.length > 0 ? flatToTree(val) : null
  },
  { immediate: true }
)

const commitTreeChange = () => {
  const flat = treeToFlat(root.value)
  const nextSignature = serializeExprNodes(flat)
  if (nextSignature === lastCommittedSignature.value) return
  lastCommittedSignature.value = nextSignature
  emit('update:modelValue', flat)
}

const createRoot = () => {
  root.value = {
    id: 'root',
    op: 6,
    dtype: 1,
    children: []
  }
  commitTreeChange()
}

const handleRootRemove = () => {
  root.value = null
  commitTreeChange()
}
</script>

<style scoped>
.filter-editor {
  border: 1px solid #eee;
  padding: 10px;
}
.empty-state {
  display: flex;
  align-items: center;
  gap: 10px;
}
.empty-text {
  color: #909399;
}
</style>
