<template>
  <div class="filter-node" :style="{ marginLeft: indent + 'px' }">
    <div class="node-row">
      <el-select v-model="node.op" placeholder="算子类型" style="width: 120px" @change="handleOpChange">
        <el-option v-for="opt in OpOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
      </el-select>

      <template v-if="!isLogicOp(node.op)">
        <el-input v-model="node.key" placeholder="Key" style="width: 140px" />
        <el-select v-model="node.dtype" placeholder="参数类型" style="width: 120px" :disabled="isInOp(node.op)">
          <el-option v-for="opt in dtypeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>

        <template v-if="node.op === 4 || node.op === 5">
           <el-input v-model="ssInput" placeholder="参数" style="width: 180px" @change="updateSS" />
        </template>
        <template v-else>
           <el-input v-if="node.dtype === 1" v-model="node.s" placeholder="参数" style="width: 180px" />
           <el-input-number v-else-if="node.dtype === 2" v-model="node.i" placeholder="参数" style="width: 180px" :controls="false" />
           <el-input-number v-else-if="node.dtype === 3" v-model="node.f" placeholder="参数" style="width: 180px" :controls="false" />
        </template>
      </template>

      <el-button size="small" @click="$emit('remove')">删除</el-button>
      <el-button v-if="canHaveChildren(node.op)" size="small" @click="addChild" :disabled="node.op === 3 && node.children.length > 0">增加子节点</el-button>
    </div>

    <div class="children" v-if="node.children && node.children.length > 0">
      <FilterNode
        v-for="(child, index) in node.children"
        :key="child.id"
        :node="child"
        :indent="20"
        @remove="removeChild(index)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { OpOptions, DTypeOptions, type TreeNode } from '@/utils/filter'

const props = defineProps<{
  node: TreeNode
  indent: number
}>()

const emit = defineEmits<{
  (e: 'remove'): void
}>()

// SS input helper
const ssInput = ref(props.node.ss?.join(',') || '')
watch(() => props.node.ss, (val) => {
  if (val) ssInput.value = val.join(',')
})

const updateSS = (val: string) => {
  props.node.ss = val.split(',').map(s => s.trim()).filter(s => s)
}

const isLogicOp = (op: number) => [1, 2, 3].includes(op)
const canHaveChildren = (op: number) => isLogicOp(op)
const isInOp = (op: number) => [4, 5].includes(op)
const dtypeOptions = computed(() => {
  if (isInOp(props.node.op)) {
    return DTypeOptions.filter(opt => opt.value === 1)
  }
  return DTypeOptions
})

watch(
  () => props.node.op,
  (val) => {
    if (!isLogicOp(val)) {
      if (isInOp(val)) {
        props.node.dtype = 1
      } else if (!props.node.dtype) {
        props.node.dtype = 1
      }
    }
  },
  { immediate: true }
)

const handleOpChange = (val: number) => {
  if (isLogicOp(val)) {
    props.node.key = undefined
    props.node.dtype = undefined
    props.node.s = undefined
    props.node.i = undefined
    props.node.f = undefined
    props.node.ss = undefined
  } else {
    props.node.children = []
    if (isInOp(val)) {
      props.node.dtype = 1
      props.node.s = undefined
      props.node.i = undefined
      props.node.f = undefined
    } else if (!props.node.dtype) {
      props.node.dtype = 1
    }
  }
}

let idCounter = 0
const generateId = () => `sub_${Date.now()}_${idCounter++}`

const addChild = () => {
  props.node.children.push({
    id: generateId(),
    op: 6,
    dtype: 1,
    children: []
  })
}

const removeChild = (index: number) => {
  props.node.children.splice(index, 1)
}
</script>

<style scoped>
.filter-node {
  margin-top: 10px;
}
.node-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
</style>
