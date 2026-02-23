<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getApps, createApp, updateApp, deleteApp, getApp, createExp, switchExp } from '@/api'
import type { Application, Experiment } from '@/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'

const router = useRouter()
const apps = ref<Application[]>([])
const selectedAppId = ref<number | null>(null)
const experiments = ref<Experiment[]>([])
const loading = ref(false)

// App Dialog
const appDialogVisible = ref(false)
const appForm = ref({ name: '', description: '' })
const isEditApp = ref(false)
const currentApp = ref<Application | null>(null)

// Exp Dialog
const expDialogVisible = ref(false)
const expForm = ref({ name: '', description: '', filter: '[]' })

const loadApps = async () => {
  try {
    const res = await getApps()
    apps.value = res.data
    if (apps.value.length > 0 && !selectedAppId.value) {
      const firstApp = apps.value[0]
      if (firstApp) {
          selectedAppId.value = firstApp.id
          await loadExperiments()
      }
    }
  } catch (e) {
    console.error(e)
    ElMessage.error('Failed to load apps')
  }
}

const loadExperiments = async () => {
  if (!selectedAppId.value) return
  loading.value = true
  try {
    const res = await getApp(selectedAppId.value)
    if (res.data.experiment) {
      experiments.value = res.data.experiment
    } else {
      experiments.value = []
    }
    // Update current app version
    const app = apps.value.find(a => a.id === selectedAppId.value)
    if (app) {
        app.version = res.data.version
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
  isEditApp.value = false
  appForm.value = { name: '', description: '' }
  appDialogVisible.value = true
}

const showEditApp = () => {
  const app = apps.value.find(a => a.id === selectedAppId.value)
  if (!app) return
  isEditApp.value = true
  currentApp.value = app
  appForm.value = { name: app.name, description: app.description || '' }
  appDialogVisible.value = true
}

const handleAppSubmit = async () => {
  try {
    if (isEditApp.value && currentApp.value) {
      await updateApp(currentApp.value.id, {
        name: appForm.value.name,
        description: appForm.value.description,
        version: currentApp.value.version
      })
      ElMessage.success('App updated')
    } else {
      await createApp(appForm.value)
      ElMessage.success('App created')
    }
    appDialogVisible.value = false
    loadApps()
  } catch (e) {
    ElMessage.error('Operation failed')
  }
}

const handleDeleteApp = async () => {
  const app = apps.value.find(a => a.id === selectedAppId.value)
  if (!app) return
  try {
    await ElMessageBox.confirm('Delete this app?', 'Warning', { type: 'warning' })
    await deleteApp(app.id, { version: app.version })
    ElMessage.success('App deleted')
    selectedAppId.value = null
    loadApps()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('Delete failed')
  }
}

const showCreateExp = () => {
  expForm.value = { name: '', description: '', filter: '[]' }
  expDialogVisible.value = true
}

const handleExpSubmit = async () => {
  const app = apps.value.find(a => a.id === selectedAppId.value)
  if (!app) return
  try {
    let filter = []
    try {
        filter = JSON.parse(expForm.value.filter)
    } catch(e) {
        ElMessage.error('Invalid filter JSON')
        return
    }

    await createExp({
      app_id: app.id,
      app_ver: app.version,
      name: expForm.value.name,
      description: expForm.value.description,
      filter: filter
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
  try {
    await switchExp(row.id, { status: newStatus, version: row.version })
    ElMessage.success('Status updated')
    loadExperiments()
  } catch (e) {
    ElMessage.error('Update failed')
    row.status = row.status === 1 ? 0 : 1
  }
}

onMounted(() => {
  loadApps()
})
</script>

<template>
  <div class="exp-list-page">
    <div class="toolbar">
      <div class="left">
        <el-select v-model="selectedAppId" placeholder="Select App" @change="handleAppChange" style="width: 200px">
          <el-option v-for="app in apps" :key="app.id" :label="`${app.name} (${app.id})`" :value="app.id" />
        </el-select>
        <el-button-group class="ml-2">
            <el-button @click="showCreateApp">New App</el-button>
            <el-button :disabled="!selectedAppId" @click="showEditApp">Edit App</el-button>
            <el-button :disabled="!selectedAppId" type="danger" @click="handleDeleteApp">Delete App</el-button>
        </el-button-group>
      </div>
      <div class="right">
        <el-button type="primary" :disabled="!selectedAppId" @click="showCreateExp">New Experiment</el-button>
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
    <el-dialog v-model="appDialogVisible" :title="isEditApp ? 'Edit App' : 'New App'">
      <el-form :model="appForm">
        <el-form-item label="Name">
          <el-input v-model="appForm.name" />
        </el-form-item>
        <el-form-item label="Description">
          <el-input v-model="appForm.description" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="appDialogVisible = false">Cancel</el-button>
        <el-button type="primary" @click="handleAppSubmit">Confirm</el-button>
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
        <el-form-item label="Filter (JSON)">
          <el-input v-model="expForm.filter" type="textarea" :rows="3" />
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
