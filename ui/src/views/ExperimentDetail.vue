<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getExp, updateExp, shuffleExp, deleteExp, getApps, getApp } from '@/api'
import type { Experiment } from '@/api/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import LayerList from '@/components/traffic/LayerList.vue'
import FilterEditor from '@/components/FilterEditor.vue'
import { validateExprNodes } from '@/utils/filter'
import { useI18n } from '@/i18n'

const route = useRoute()
const router = useRouter()
const expId = Number(route.params.id)
const experiment = ref<Experiment | null>(null)
const loading = ref(false)
const appInfo = ref<{ id: number; version: number } | null>(null)
const layerListRef = ref<InstanceType<typeof LayerList> | null>(null)
const expSnapshot = ref<{ name: string; description?: string; filter: string } | null>(null)
const { t } = useI18n()

const getFilterText = (filter?: Experiment['filter']) => JSON.stringify(filter || [])

const syncSnapshot = (exp: Experiment) => {
  expSnapshot.value = {
    name: exp.name,
    description: exp.description,
    filter: getFilterText(exp.filter)
  }
}

const loadExp = async () => {
  loading.value = true
  try {
    const res = await getExp(expId)
    experiment.value = res.data
    syncSnapshot(res.data)
  } catch (e) {
    ElMessage.error(t('message.failedLoadExperiments'))
  } finally {
    loading.value = false
  }
}

const handleUpdate = async () => {
    if (!experiment.value) return
    if (
      expSnapshot.value &&
      expSnapshot.value.name === experiment.value.name &&
      expSnapshot.value.description === experiment.value.description &&
      expSnapshot.value.filter === getFilterText(experiment.value.filter)
    ) {
      return
    }
    const validation = validateExprNodes(experiment.value.filter || [])
    if (!validation.valid) {
      ElMessage.error(validation.messageKey ? t(validation.messageKey) : t('message.invalidFilter'))
      return
    }
    try {
        await updateExp(experiment.value.id, {
            name: experiment.value.name,
            description: experiment.value.description,
            version: experiment.value.version,
            filter: experiment.value.filter
        })
        ElMessage.success(t('message.experimentUpdated'))
        experiment.value.version = experiment.value.version + 1
        syncSnapshot(experiment.value)
    } catch(e) {
        ElMessage.error(t('message.updateFailedRefresh'))
    }
}

const handleShuffle = async () => {
    if (!experiment.value) return
    try {
        await shuffleExp(experiment.value.id)
        ElMessage.success(t('message.shuffled'))
    } catch(e) {
        ElMessage.error(t('message.operationFailed'))
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
    ElMessage.error(t('message.appInfoMissing'))
    return
  }
  try {
    await ElMessageBox.confirm(t('confirm.deleteExperiment'), t('common.warning'), { type: 'warning' })
    await deleteExp(experiment.value.id, {
      app_id: appInfo.value.id,
      app_ver: appInfo.value.version,
      version: experiment.value.version
    })
    ElMessage.success(t('message.experimentDeleted'))
    router.push({
      path: '/',
      query: {
        app_id: String(appInfo.value.id),
        refresh: String(Date.now())
      }
    })
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(t('message.deleteFailed'))
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
        <el-input v-model="experiment.name" :placeholder="t('detail.expName')" />
        <el-input v-model="experiment.description" :placeholder="t('detail.expDesc')" />
        <el-button type="primary" @click="handleUpdate">{{ t('common.update') }}</el-button>
        <el-button type="danger" @click="handleDelete">{{ t('common.delete') }}</el-button>
        <div class="exp-row-right">
          <el-button @click="handleShuffle">{{ t('group.shuffle') }}</el-button>
          <el-button type="primary" @click="handleCreateLayer">{{ t('detail.createLayer') }}</el-button>
        </div>
      </div>
      <div class="filter-section">
        <div class="filter-title">{{ t('detail.filter') }}</div>
        <FilterEditor v-model="experiment.filter" />
      </div>
    </div>

    <div class="section">
      <LayerList ref="layerListRef" :experiment="experiment" />
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
