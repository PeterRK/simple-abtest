<template>
  <div class="layer-list">
    <div class="header">
      <el-button type="primary" size="small" @click="openLayerDialog('create')">Add Layer</el-button>
    </div>
    <el-collapse v-model="activeLayers">
      <el-collapse-item v-for="layer in layers" :key="layer.id" :name="layer.id">
        <template #title>
           <div class="layer-title">
             <span>{{ layer.name }} (ID: {{ layer.id }})</span>
             <div class="layer-actions" @click.stop>
               <el-button type="text" @click="openLayerDialog('edit', layer)">Edit</el-button>
               <el-button type="text" class="danger" @click="handleDeleteLayer(layer)">Delete</el-button>
             </div>
           </div>
        </template>
        <!-- Segment List -->
        <SegmentList :layer="layer" @refresh="$emit('refresh')" />
      </el-collapse-item>
    </el-collapse>
    
    <!-- Layer Dialog -->
    <el-dialog v-model="dialogVisible" :title="dialogType === 'create' ? 'Add Layer' : 'Edit Layer'">
      <el-form :model="form" label-width="100px">
        <el-form-item label="Name">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="Description">
          <el-input v-model="form.description" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">Cancel</el-button>
        <el-button type="primary" @click="handleSave">Save</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { createLyr, updateLyr, deleteLyr, getLayer } from '@/api'
import type { Experiment, Layer } from '@/api/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import SegmentList from './SegmentList.vue'

const props = defineProps<{
  experiment: Experiment | null
}>()

const emit = defineEmits(['refresh'])

const activeLayers = ref<number[]>([])
const layers = ref<Layer[]>([])

const dialogVisible = ref(false)
const dialogType = ref<'create' | 'edit'>('create')
const form = ref({ name: '', description: '' })
const currentLayer = ref<Layer | null>(null)

const openLayerDialog = (type: 'create' | 'edit', row?: Layer) => {
  dialogType.value = type
  if (type === 'edit' && row) {
    currentLayer.value = row
    form.value = {
      name: row.name,
      description: row.description || ''
    }
  } else {
    currentLayer.value = null
    form.value = { name: '', description: '' }
  }
  dialogVisible.value = true
}

const loadLayers = async () => {
  if (!props.experiment?.layer || props.experiment.layer.length === 0) {
    layers.value = []
    return
  }
  const results = await Promise.all(
    props.experiment.layer.map(async layer => {
      try {
        const res = await getLayer(layer.id)
        return res.data
      } catch (e) {
        return layer
      }
    })
  )
  layers.value = results as Layer[]
}

const handleSave = async () => {
  if (!props.experiment) return
  try {
    if (dialogType.value === 'create') {
      await createLyr({
        exp_id: props.experiment.id,
        exp_ver: props.experiment.version!,
        name: form.value.name,
        description: form.value.description
      })
      ElMessage.success('Layer created')
    } else if (currentLayer.value) {
      await updateLyr(currentLayer.value.id, {
        name: form.value.name,
        description: form.value.description,
        version: currentLayer.value.version
      })
      ElMessage.success('Layer updated')
    }
    dialogVisible.value = false
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

watch(
  () => props.experiment?.layer,
  () => {
    loadLayers()
  },
  { deep: true, immediate: true }
)
</script>

<style scoped>
.header {
    margin-bottom: 10px;
}
.layer-title {
    display: flex;
    justify-content: space-between;
    width: 100%;
    padding-right: 10px;
}
.danger {
    color: #f56c6c;
}
</style>
