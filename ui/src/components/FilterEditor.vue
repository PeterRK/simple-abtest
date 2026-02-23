<template>
  <div class="filter-editor">
    <div class="header">
      <el-checkbox v-model="enabled">Enable Filter</el-checkbox>
    </div>
    <div v-if="enabled && root" class="tree-container">
      <FilterNode :node="root" :indent="0" @remove="handleRootRemove" />
    </div>
    <div v-else-if="enabled" class="empty-state">
      <el-button @click="createRoot">Add Root Rule</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import FilterNode from './FilterNode.vue'
import { flatToTree, treeToFlat, type ExprNode, type TreeNode } from '@/utils/filter'

const props = defineProps<{
  modelValue: ExprNode[] | undefined
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', val: ExprNode[]): void
}>()

const enabled = ref(false)
const root = ref<TreeNode | null>(null)

watch(() => props.modelValue, (val) => {
  // Sync from prop only if we don't have local changes or initial load
  // Actually, better to parse always if it differs significantly?
  // For simplicity: convert on mount or if external change implies reset.
  // Here assuming one-way data flow for init, then internal state emits up.
  if (val && val.length > 0) {
    enabled.value = true
    // Only rebuild if root is null or we want to force sync?
    // Let's rebuild for now to be safe on load.
    // Ideally we should check deep equality.
    if (!root.value) {
        root.value = flatToTree(val)
    }
  } else {
    if (!root.value) enabled.value = false
  }
}, { immediate: true })

watch([enabled, root], () => {
  if (!enabled.value) {
    emit('update:modelValue', [])
    return
  }
  if (root.value) {
    const flat = treeToFlat(root.value)
    emit('update:modelValue', flat)
  } else {
    emit('update:modelValue', [])
  }
}, { deep: true })

const createRoot = () => {
  root.value = {
    id: 'root',
    op: 1, // AND
    children: []
  }
}

const handleRootRemove = () => {
  root.value = null
  enabled.value = false
}
</script>

<style scoped>
.filter-editor {
  border: 1px solid #eee;
  padding: 10px;
}
</style>
