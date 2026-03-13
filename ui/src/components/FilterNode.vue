<template>
  <div class="filter-node" :style="{ marginLeft: indent + 'px' }">
    <div class="node-row">
      <el-select v-model="node.op" :placeholder="t('filter.opType')" style="width: 120px" @change="handleOpChange">
        <el-option v-for="opt in OpOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
      </el-select>

      <template v-if="!isLogicOp(node.op)">
        <el-input v-model="node.key" :placeholder="t('common.key')" style="width: 140px" @update:model-value="emitChange" />
        <el-select
          v-model="node.dtype"
          :placeholder="t('filter.paramType')"
          style="width: 120px"
          :disabled="isInOp(node.op)"
          @change="emitChange"
        >
          <el-option v-for="opt in dtypeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>

        <template v-if="node.op === 4 || node.op === 5">
           <el-input v-model="ssInput" :placeholder="t('filter.param')" style="width: 180px" @change="updateSS" />
        </template>
        <template v-else>
           <el-input v-if="node.dtype === 1" v-model="node.s" :placeholder="t('filter.param')" style="width: 180px" @update:model-value="emitChange" />
           <el-input-number v-else-if="node.dtype === 2" v-model="node.i" :placeholder="t('filter.param')" style="width: 180px" :controls="false" @update:model-value="emitChange" />
           <el-input-number v-else-if="node.dtype === 3" v-model="node.f" :placeholder="t('filter.param')" style="width: 180px" :controls="false" @update:model-value="emitChange" />
        </template>
      </template>

      <el-button size="small" @click="$emit('remove')">{{ t('common.delete') }}</el-button>
      <el-button v-if="canHaveChildren(node.op)" size="small" @click="addChild" :disabled="node.op === 3 && node.children.length > 0">{{ t('filter.addChild') }}</el-button>
    </div>

    <div class="children" v-if="node.children && node.children.length > 0">
      <FilterNode
        v-for="(child, index) in node.children"
        :key="child.id"
        :node="child"
        :indent="20"
        @remove="removeChild(index)"
        @change="emitChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { getOpOptions, getDTypeOptions, type TreeNode } from '@/utils/filter'
import { useI18n } from '@/i18n'

const props = defineProps<{
  node: TreeNode
  indent: number
}>()

const emit = defineEmits<{
  (e: 'remove'): void
  (e: 'change'): void
}>()

const { t } = useI18n()

// SS input helper
const ssInput = ref(props.node.ss?.join(',') || '')
watch(() => props.node.ss, (val) => {
  if (val) ssInput.value = val.join(',')
})

const updateSS = (val: string) => {
  props.node.ss = val.split(',').map(s => s.trim()).filter(s => s)
  emitChange()
}

const isLogicOp = (op: number) => [1, 2, 3].includes(op)
const canHaveChildren = (op: number) => isLogicOp(op)
const isInOp = (op: number) => [4, 5].includes(op)
const OpOptions = computed(() => getOpOptions(t))
const dtypeOptions = computed(() => {
  const allTypes = getDTypeOptions(t)
  if (isInOp(props.node.op)) {
    return allTypes.filter(opt => opt.value === 1)
  }
  return allTypes
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
  emitChange()
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
  emitChange()
}

const removeChild = (index: number) => {
  props.node.children.splice(index, 1)
  emitChange()
}

const emitChange = () => {
  emit('change')
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
