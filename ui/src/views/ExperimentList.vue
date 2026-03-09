<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { getApps, createApp, updateApp, deleteApp, getApp, createExp, switchExp } from '@/api'
import type { Application, Experiment } from '@/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from '@/i18n'

const router = useRouter()
const route = useRoute()
const RECENT_APP_ID_KEY = 'simple-abtest:recent-app-id'
const apps = ref<Application[]>([])
const selectedAppId = ref<number | null>(null)
const experiments = ref<Experiment[]>([])
const loading = ref(false)

// App Dialog
const appDialogVisible = ref(false)
const appForm = ref({ name: '', description: '' })
const appDialogMode = ref<'create' | 'detail'>('create')
const currentApp = ref<Application | null>(null)

// Exp Dialog
const expDialogVisible = ref(false)
const expForm = ref({ name: '', description: '' })
const experimentStatusMap = ref(new Map<number, number>())
const { t } = useI18n()

const getRememberedAppId = () => {
  if (typeof window === 'undefined') return null
  const raw = window.localStorage.getItem(RECENT_APP_ID_KEY)
  if (!raw) return null
  const parsed = Number(raw)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null
}

const rememberAppId = (appId: number | null) => {
  if (typeof window === 'undefined') return
  if (appId == null || appId <= 0) return
  window.localStorage.setItem(RECENT_APP_ID_KEY, String(appId))
}

const loadApps = async () => {
  try {
    const res = await getApps()
    apps.value = res.data
    if (selectedAppId.value) {
      const exists = apps.value.find(a => a.id === selectedAppId.value)
      if (!exists) {
        selectedAppId.value = null
        currentApp.value = null
        experiments.value = []
      }
    }
    if (!selectedAppId.value) {
      const rememberedAppId = getRememberedAppId()
      if (rememberedAppId && apps.value.some(app => app.id === rememberedAppId)) {
        selectedAppId.value = rememberedAppId
      }
    }
  } catch (e) {
    console.error(e)
    if ((e as any)?.response?.status === 401) {
      apps.value = []
      selectedAppId.value = null
      currentApp.value = null
      experiments.value = []
      return
    }
    ElMessage.error(t('message.failedLoadApps'))
  }
}

const loadExperiments = async () => {
  if (!selectedAppId.value) {
    experiments.value = []
    currentApp.value = null
    return
  }
  loading.value = true
  try {
    const appId = selectedAppId.value
    const res = await getApp(appId)
    experiments.value = res.data.experiment ? res.data.experiment : []
    const statusMap = new Map<number, number>()
    for (const exp of experiments.value) {
      statusMap.set(exp.id, exp.status)
    }
    experimentStatusMap.value = statusMap

    currentApp.value = {
      id: res.data.id,
      name: res.data.name,
      access_token: res.data.access_token,
      version: res.data.version,
      description: res.data.description,
      experiment: res.data.experiment
    }

    const index = apps.value.findIndex(a => a.id === appId)
    const app = index !== -1 ? apps.value[index] : undefined
    if (app) {
      app.name = res.data.name
      app.access_token = res.data.access_token
      app.description = res.data.description
      app.version = res.data.version
      app.experiment = res.data.experiment
    }
  } catch (e) {
    console.error(e)
    if ((e as any)?.response?.status === 401) {
      experiments.value = []
      currentApp.value = null
      return
    }
    ElMessage.error(t('message.failedLoadExperiments'))
  } finally {
    loading.value = false
  }
}

const handleAppChange = () => {
  rememberAppId(selectedAppId.value)
  loadExperiments()
}

const showCreateApp = () => {
  appDialogMode.value = 'create'
  currentApp.value = null
  appForm.value = { name: '', description: '' }
  appDialogVisible.value = true
}

const showAppDetail = () => {
  const app = apps.value.find(a => a.id === selectedAppId.value)
  if (!app) return
  appDialogMode.value = 'detail'
  currentApp.value = app
  appForm.value = { name: app.name, description: app.description || '' }
  appDialogVisible.value = true
}

const handleCreateApp = async () => {
  try {
    const res = await createApp(appForm.value)
    const created = res.data
    selectedAppId.value = created.id
    currentApp.value = {
      id: created.id,
      name: created.name,
      access_token: created.access_token,
      version: created.version,
      description: created.description,
      experiment: created.experiment
    }
    ElMessage.success(t('message.appCreated'))
    appDialogVisible.value = false
    await loadApps()
    await loadExperiments()
  } catch (e) {
    ElMessage.error(t('message.operationFailed'))
  }
}

const handleUpdateApp = async () => {
  if (!currentApp.value) return
  if (currentApp.value.version == null) {
    ElMessage.error(t('message.appVersionMissing'))
    return
  }
  if (
    currentApp.value.name === appForm.value.name &&
    (currentApp.value.description || '') === appForm.value.description
  ) {
    return
  }
  try {
    await updateApp(currentApp.value.id, {
      name: appForm.value.name,
      description: appForm.value.description,
      version: currentApp.value.version
    })
    ElMessage.success(t('message.appUpdated'))
    currentApp.value.name = appForm.value.name
    currentApp.value.description = appForm.value.description
    currentApp.value.version = currentApp.value.version + 1
    appDialogVisible.value = false
  } catch (e) {
    ElMessage.error(t('message.updateFailed'))
  }
}

const handleDeleteAppInDialog = async () => {
  const app = currentApp.value
  if (!app) return
  if (app.version == null) {
    ElMessage.error(t('message.appVersionMissing'))
    return
  }
  try {
    await ElMessageBox.confirm(t('confirm.deleteApp'), t('common.warning'), { type: 'warning' })
    await deleteApp(app.id, { version: app.version })
    ElMessage.success(t('message.appDeleted'))
    appDialogVisible.value = false
    selectedAppId.value = null
    loadApps()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(t('message.deleteFailed'))
  }
}

const showCreateExp = () => {
  expForm.value = { name: '', description: '' }
  expDialogVisible.value = true
}

const handleExpSubmit = async () => {
  const app = apps.value.find(a => a.id === selectedAppId.value)
  if (!app) return
  if (app.version == null) {
    ElMessage.error(t('message.appVersionMissing'))
    return
  }
  try {
    await createExp({
      app_id: app.id,
      app_ver: app.version,
      name: expForm.value.name,
      description: expForm.value.description
    })
    ElMessage.success(t('message.experimentCreated'))
    expDialogVisible.value = false
    loadExperiments()
  } catch (e) {
    ElMessage.error(t('message.createFailed'))
  }
}

const handleExpClick = (row: Experiment) => {
  router.push(`/experiment/${row.id}`)
}

const handleSwitchChange = async (val: number | boolean | string, row: Experiment) => {
  const newStatus = val ? 1 : 0
  const previousStatus = experimentStatusMap.value.get(row.id)
  if (previousStatus === newStatus) return
  try {
    await switchExp(row.id, { status: newStatus, version: row.version })
    ElMessage.success(t('message.statusUpdated'))
    row.version = row.version + 1
    experimentStatusMap.value.set(row.id, newStatus)
  } catch (e) {
    ElMessage.error(t('message.updateFailedRefresh'))
    row.status = row.status === 1 ? 0 : 1
  }
}

onMounted(() => {
  loadApps().then(() => {
    const appId = Number(route.query.app_id)
    if (Number.isFinite(appId) && appId > 0) {
      selectedAppId.value = appId
      rememberAppId(appId)
      loadExperiments()
      return
    }
    if (selectedAppId.value) {
      loadExperiments()
    }
  })
})

watch(
  () => selectedAppId.value,
  (val) => {
    rememberAppId(val)
  }
)

watch(
  () => route.query.refresh,
  () => {
    const appId = Number(route.query.app_id)
    if (!Number.isFinite(appId) || appId <= 0) return
    selectedAppId.value = appId
    loadApps().then(() => {
      loadExperiments()
    })
  }
)
</script>

<template>
  <div class="exp-list-page">
    <div class="toolbar">
      <div class="left">
        <el-select v-model="selectedAppId" :placeholder="t('list.selectApp')" @change="handleAppChange" style="width: 200px">
          <el-option v-for="app in apps" :key="app.id" :label="`${app.name} (${app.id})`" :value="app.id" />
        </el-select>
        <el-button-group class="ml-2">
            <el-button @click="showCreateApp">{{ t('common.create') }}</el-button>
            <el-button :disabled="!selectedAppId" @click="showAppDetail">{{ t('common.detail') }}</el-button>
        </el-button-group>
      </div>
      <div class="right">
        <el-button type="primary" :disabled="!selectedAppId" @click="showCreateExp">{{ t('list.createExperiment') }}</el-button>
      </div>
    </div>

    <el-table :data="experiments" style="width: 100%" v-loading="loading" @row-click="handleExpClick" row-class-name="clickable-row">
      <el-table-column prop="id" :label="t('common.id')" width="100" />
      <el-table-column prop="name" :label="t('common.name')" />
      <el-table-column prop="description" :label="t('common.description')" />
      <el-table-column :label="t('common.status')" width="100">
        <template #default="{ row }">
          <el-switch
            v-model="row.status"
            :active-value="1"
            :inactive-value="0"
            @click.stop
            @change="(val: number | string | boolean) => handleSwitchChange(val, row)"
          />
        </template>
      </el-table-column>
    </el-table>

    <!-- App Dialog -->
    <el-dialog v-model="appDialogVisible" :title="appDialogMode === 'create' ? t('list.appCreateTitle') : t('list.appDetailTitle')">
      <el-form :model="appForm">
        <el-form-item v-if="appDialogMode === 'detail' && currentApp" :label="t('common.id')">
          <span>{{ currentApp.id }}</span>
        </el-form-item>
        <el-form-item v-if="appDialogMode === 'detail' && currentApp" :label="t('common.accessToken')">
          <el-input :model-value="currentApp.access_token || ''" readonly />
        </el-form-item>
        <el-form-item :label="t('common.name')">
          <el-input v-model="appForm.name" />
        </el-form-item>
        <el-form-item :label="t('common.description')">
          <el-input v-model="appForm.description" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="appDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <template v-if="appDialogMode === 'detail'">
          <el-button type="primary" @click="handleUpdateApp">{{ t('common.update') }}</el-button>
          <el-button type="danger" @click="handleDeleteAppInDialog">{{ t('common.delete') }}</el-button>
        </template>
        <template v-else>
          <el-button type="primary" @click="handleCreateApp">{{ t('common.confirm') }}</el-button>
        </template>
      </template>
    </el-dialog>

    <!-- Exp Dialog -->
    <el-dialog v-model="expDialogVisible" :title="t('list.experimentCreateTitle')">
      <el-form :model="expForm">
        <el-form-item :label="t('common.name')">
          <el-input v-model="expForm.name" />
        </el-form-item>
        <el-form-item :label="t('common.description')">
          <el-input v-model="expForm.description" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="expDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleExpSubmit">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  margin-bottom: 20px;
}
.ml-2 {
    margin-left: 10px;
}
:deep(.clickable-row) {
    cursor: pointer;
}
</style>
