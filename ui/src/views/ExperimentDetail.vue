<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getExp, updateExp, shuffleExp, deleteExp, getApps, getApp } from '@/api'
import type { Experiment } from '@/api/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import LayerList from '@/components/traffic/LayerList.vue'
import FilterEditor from '@/components/FilterEditor.vue'
import { validateExprNodes } from '@/utils/filter'

const route = useRoute()
const router = useRouter()
const expId = Number(route.params.id)
const experiment = ref<Experiment | null>(null)
const loading = ref(false)
const appInfo = ref<{ id: number; version: number } | null>(null)
const layerListRef = ref<InstanceType<typeof LayerList> | null>(null)

const loadExp = async () => {
  loading.value = true
  try {
    const res = await getExp(expId)
    experiment.value = res.data
  } catch (e) {
    ElMessage.error('Failed to load experiment')
  } finally {
    loading.value = false
  }
}

const handleUpdate = async () => {
    if (!experiment.value) return
    const validation = validateExprNodes(experiment.value.filter || [])
    if (!validation.valid) {
      ElMessage.error(validation.message || '过滤条件不合法')
      return
    }
    try {
        await updateExp(experiment.value.id, {
            name: experiment.value.name,
            description: experiment.value.description,
            version: experiment.value.version,
            filter: experiment.value.filter
        })
        ElMessage.success('Experiment updated')
        loadExp()
    } catch(e) {
        ElMessage.error('Update failed')
    }
}

const handleShuffle = async () => {
    if (!experiment.value) return
    try {
        await shuffleExp(experiment.value.id)
        ElMessage.success('Shuffled')
    } catch(e) {
        ElMessage.error('Shuffle failed')
    }
}

const handleCreateLayer = () => {
  layerListRef.value?.openLayerDialog()
}

const resolveAppInfo = async () => {
  if (experiment.value?.app_id && experiment.value?.app_ver) {
    appInfo.value = { id: experiment.value.app_id, version: experiment.value.app_ver }
    return
  }
  try {
    const appsRes = await getApps()
    for (const app of appsRes.data || []) {
      const appRes = await getApp(app.id)
      const hasExp = (appRes.data.experiment || []).some(exp => exp.id === expId)
      if (hasExp && appRes.data.version != null) {
        appInfo.value = { id: appRes.data.id, version: appRes.data.version }
        return
      }
    }
  } catch (e) {
    appInfo.value = null
  }
}

const handleDelete = async () => {
  if (!experiment.value) return
  await resolveAppInfo()
  if (!appInfo.value) {
    ElMessage.error('无法获取应用信息')
    return
  }
  try {
    await ElMessageBox.confirm('确认删除该实验？', '提示', { type: 'warning' })
    await deleteExp(experiment.value.id, {
      app_id: appInfo.value.id,
      app_ver: appInfo.value.version,
      version: experiment.value.version
    })
    ElMessage.success('实验已删除')
    router.push('/')
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('删除失败')
  }
}

onMounted(() => {
  loadExp()
})
</script>

<template>
  <div class="exp-detail-page" v-if="experiment" v-loading="loading">
    <div class="exp-body">
      <div class="exp-row">
        <el-input v-model="experiment.name" placeholder="实验名" />
        <el-input v-model="experiment.description" placeholder="实验描述" />
        <el-button type="primary" @click="handleUpdate">更新</el-button>
        <el-button type="danger" @click="handleDelete">删除</el-button>
        <div class="exp-row-right">
          <el-button @click="handleShuffle">流量打散</el-button>
          <el-button type="primary" @click="handleCreateLayer">新增Layer</el-button>
        </div>
      </div>
      <div class="filter-section">
        <div class="filter-title">过滤条件</div>
        <FilterEditor v-model="experiment.filter" />
      </div>
    </div>

    <div class="section">
      <LayerList ref="layerListRef" :experiment="experiment" @refresh="loadExp" />
    </div>
  </div>
</template>

<style scoped>
.section {
    margin-top: 30px;
}
.exp-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.exp-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.exp-row :deep(.el-input) {
  width: 240px;
}
.exp-row-right {
  margin-left: auto;
  display: flex;
  gap: 10px;
}
.filter-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.filter-title {
  font-weight: 600;
}
</style>
