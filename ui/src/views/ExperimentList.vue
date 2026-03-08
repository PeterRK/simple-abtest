<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { getApps, createApp, updateApp, deleteApp, getApp, createExp, switchExp } from '@/api'
import type { Application, Experiment } from '@/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'

const router = useRouter()
const route = useRoute()
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
  } catch (e) {
    console.error(e)
    ElMessage.error('Failed to load apps')
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
      version: res.data.version,
      description: res.data.description,
      experiment: res.data.experiment
    }

    const index = apps.value.findIndex(a => a.id === appId)
    const app = index !== -1 ? apps.value[index] : undefined
    if (app) {
      app.name = res.data.name
      app.description = res.data.description
      app.version = res.data.version
      app.experiment = res.data.experiment
    }
  } catch (e) {
    console.error(e)
    ElMessage.error('Failed to load experiments')
  } finally {
    loading.value = false
  }
}

const handleAppChange = () => {
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
      version: created.version,
      description: created.description,
      experiment: created.experiment
    }
    ElMessage.success('App created')
    appDialogVisible.value = false
    await loadApps()
    await loadExperiments()
  } catch (e) {
    ElMessage.error('Operation failed')
  }
}

const handleUpdateApp = async () => {
  if (!currentApp.value) return
  if (currentApp.value.version == null) {
    ElMessage.error('App version is missing')
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
    ElMessage.success('App updated')
    currentApp.value.name = appForm.value.name
    currentApp.value.description = appForm.value.description
    currentApp.value.version = currentApp.value.version + 1
    appDialogVisible.value = false
  } catch (e) {
    ElMessage.error('Update failed')
  }
}

const handleDeleteAppInDialog = async () => {
  const app = currentApp.value
  if (!app) return
  if (app.version == null) {
    ElMessage.error('App version is missing')
    return
  }
  try {
    await ElMessageBox.confirm('确定删除该应用？', '提示', { type: 'warning' })
    await deleteApp(app.id, { version: app.version })
    ElMessage.success('App deleted')
    appDialogVisible.value = false
    selectedAppId.value = null
    loadApps()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('Delete failed')
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
    ElMessage.error('App version is missing')
    return
  }
  try {
    await createExp({
      app_id: app.id,
      app_ver: app.version,
      name: expForm.value.name,
      description: expForm.value.description
    })
    ElMessage.success('Experiment created')
    expDialogVisible.value = false
    loadExperiments()
  } catch (e) {
    ElMessage.error('Create failed')
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
    ElMessage.success('Status updated')
    row.version = row.version + 1
    experimentStatusMap.value.set(row.id, newStatus)
  } catch (e) {
    ElMessage.error('更新失败，请手动刷新后重试')
    row.status = row.status === 1 ? 0 : 1
  }
}

onMounted(() => {
  loadApps().then(() => {
    const appId = Number(route.query.app_id)
    if (Number.isFinite(appId) && appId > 0) {
      selectedAppId.value = appId
      loadExperiments()
    }
  })
})

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
        <el-select v-model="selectedAppId" placeholder="Select App" @change="handleAppChange" style="width: 200px">
          <el-option v-for="app in apps" :key="app.id" :label="`${app.name} (${app.id})`" :value="app.id" />
        </el-select>
        <el-button-group class="ml-2">
            <el-button @click="showCreateApp">新增</el-button>
            <el-button :disabled="!selectedAppId" @click="showAppDetail">详情</el-button>
        </el-button-group>
      </div>
      <div class="right">
        <el-button type="primary" :disabled="!selectedAppId" @click="showCreateExp">新增实验</el-button>
      </div>
    </div>

    <el-table :data="experiments" style="width: 100%" v-loading="loading" @row-click="handleExpClick" row-class-name="clickable-row">
      <el-table-column prop="id" label="ID" width="100" />
      <el-table-column prop="name" label="Name" />
      <el-table-column prop="description" label="Description" />
      <el-table-column label="Status" width="100">
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
    <el-dialog v-model="appDialogVisible" :title="appDialogMode === 'create' ? '新增应用' : '应用详情'">
      <el-form :model="appForm">
        <el-form-item v-if="appDialogMode === 'detail' && currentApp" label="ID">
          <span>{{ currentApp.id }}</span>
        </el-form-item>
        <el-form-item label="名称">
          <el-input v-model="appForm.name" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="appForm.description" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="appDialogVisible = false">取消</el-button>
        <template v-if="appDialogMode === 'detail'">
          <el-button type="primary" @click="handleUpdateApp">更新</el-button>
          <el-button type="danger" @click="handleDeleteAppInDialog">删除</el-button>
        </template>
        <template v-else>
          <el-button type="primary" @click="handleCreateApp">确定</el-button>
        </template>
      </template>
    </el-dialog>

    <!-- Exp Dialog -->
    <el-dialog v-model="expDialogVisible" title="New Experiment">
      <el-form :model="expForm">
        <el-form-item label="Name">
          <el-input v-model="expForm.name" />
        </el-form-item>
        <el-form-item label="Description">
          <el-input v-model="expForm.description" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="expDialogVisible = false">Cancel</el-button>
        <el-button type="primary" @click="handleExpSubmit">Confirm</el-button>
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
