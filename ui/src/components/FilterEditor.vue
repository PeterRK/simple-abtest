<template>
  <div class="filter-editor">
    <div v-if="root" class="tree-container">
      <FilterNode :node="root" :indent="0" @remove="handleRootRemove" />
    </div>
    <div v-else class="empty-state">
      <span class="empty-text">无过滤条件</span>
      <el-button size="small" @click="createRoot">新增根算子</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import FilterNode from './FilterNode.vue'
import { flatToTree, treeToFlat, type ExprNode, type TreeNode } from '@/utils/filter'

const props = defineProps<{
  modelValue: ExprNode[] | undefined
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', val: ExprNode[]): void
}>()

const root = ref<TreeNode | null>(null)

watch(() => props.modelValue, (val) => {
  // Sync from prop only if we don't have local changes or initial load
  // Actually, better to parse always if it differs significantly?
  // For simplicity: convert on mount or if external change implies reset.
  // Here assuming one-way data flow for init, then internal state emits up.
  if (val && val.length > 0) {
    if (!root.value) {
        root.value = flatToTree(val)
    }
  }
}, { immediate: true })

watch(root, () => {
  const flat = treeToFlat(root.value)
  emit('update:modelValue', flat)
}, { deep: true })

const createRoot = () => {
  root.value = {
    id: 'root',
    op: 6,
    dtype: 1,
    children: []
  }
}

const handleRootRemove = () => {
  root.value = null
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
