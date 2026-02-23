<template>
  <div class="filter-node" :style="{ marginLeft: indent + 'px' }">
    <div class="node-row">
      <el-select v-model="node.op" placeholder="Op" style="width: 100px" @change="handleOpChange">
        <el-option v-for="opt in OpOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
      </el-select>

      <!-- Key/DType/Params only for non-logic Ops (not AND/OR/NOT) -->
      <!-- Wait, doc says: 
           AND/OR/NOT: logic ops, have children.
           Others: comparison ops, have key/dtype/params.
      -->
      <template v-if="!isLogicOp(node.op)">
        <el-input v-model="node.key" placeholder="Key" style="width: 120px" />
        <el-select v-model="node.dtype" placeholder="Type" style="width: 100px">
          <el-option v-for="opt in DTypeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        
        <!-- Params based on DType -->
        <template v-if="node.op === 4 || node.op === 5"> <!-- IN / NOT IN -->
           <!-- For SS, we can use simple input comma separated or tags -->
           <el-input v-model="ssInput" placeholder="v1,v2" style="width: 150px" @change="updateSS" />
        </template>
        <template v-else>
           <el-input v-if="node.dtype === 1" v-model="node.s" placeholder="Value" style="width: 150px" />
           <el-input-number v-else-if="node.dtype === 2" v-model="node.i" placeholder="Value" style="width: 150px" controls-position="right" />
           <el-input-number v-else-if="node.dtype === 3" v-model="node.f" placeholder="Value" style="width: 150px" controls-position="right" />
        </template>
      </template>

      <el-button type="danger" circle size="small" @click="$emit('remove')">
        <el-icon><Delete /></el-icon>
      </el-button>

      <el-button v-if="canHaveChildren(node.op)" type="primary" size="small" circle @click="addChild">
        <el-icon><Plus /></el-icon>
      </el-button>
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
import { ref, computed, watch } from 'vue'
import { OpOptions, DTypeOptions, type TreeNode } from '@/utils/filter'
import { Delete, Plus } from '@element-plus/icons-vue'

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

const handleOpChange = (val: number) => {
  if (isLogicOp(val)) {
    // Clear data fields if switching to logic
    props.node.key = undefined
    props.node.dtype = undefined
    props.node.s = undefined
    props.node.i = undefined
    props.node.f = undefined
    props.node.ss = undefined
  } else {
    // Clear children if switching to comparison (though UI might hide them, better clear)
    props.node.children = []
    if (!props.node.dtype) props.node.dtype = 1 // default string
  }
}

let idCounter = 0
const generateId = () => `sub_${Date.now()}_${idCounter++}`

const addChild = () => {
  props.node.children.push({
    id: generateId(),
    op: 6, // Default to Equal
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
