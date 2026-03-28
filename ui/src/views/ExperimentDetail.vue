<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getExp, updateExp, shuffleExp, deleteExp, getApp, getAppPrivileges, grantAppPrivilege } from '@/api'
import type { Application, Experiment } from '@/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import LayerList from '@/components/traffic/LayerList.vue'
import FilterEditor from '@/components/FilterEditor.vue'
import { serializeExprNodes, validateExprNodes } from '@/utils/filter'
import { useI18n } from '@/i18n'
import { getNameMaxLength, validateName } from '@/utils/name'

type AppSnapshot = {
  id: number
  name: string
  description?: string
  version: number
  experiment: {
    id: number
    status: number
    version: number
    name: string
    description?: string
  }[]
}

const route = useRoute()
const router = useRouter()
const expId = Number(route.params.id)
const experiment = ref<Experiment | null>(null)
const loading = ref(false)
const appVersion = ref<number | null>(null)
const layerListRef = ref<InstanceType<typeof LayerList> | null>(null)
const expSnapshot = ref<{ name: string; description?: string; filter: string } | null>(null)
const { t } = useI18n()
const privilegeDialogVisible = ref(false)
const privilegeLoading = ref(false)
const privileges = ref<{ name: string; privilege: number; grantor: string }[]>([])
const privilegeForm = ref({ name: '', privilege: 1 })
const experimentNameMaxLength = getNameMaxLength('experiment')
const userNameMaxLength = getNameMaxLength('user')

const getFilterText = (filter?: Experiment['filter']) => serializeExprNodes(filter)
const normalizeText = (text?: string) => text || ''

const syncSnapshot = (exp: Experiment) => {
  expSnapshot.value = {
    name: exp.name,
    description: normalizeText(exp.description),
    filter: getFilterText(exp.filter)
  }
}

const isExperimentNameDirty = computed(() => {
  if (!experiment.value || !expSnapshot.value) return false
  return expSnapshot.value.name !== experiment.value.name
})

const isExperimentDescriptionDirty = computed(() => {
  if (!experiment.value || !expSnapshot.value) return false
  return expSnapshot.value.description !== normalizeText(experiment.value.description)
})

const isExperimentFilterDirty = computed(() => {
  if (!experiment.value || !expSnapshot.value) return false
  return expSnapshot.value.filter !== getFilterText(experiment.value.filter)
})

const isExperimentDirty = computed(
  () => isExperimentNameDirty.value || isExperimentDescriptionDirty.value || isExperimentFilterDirty.value
)

const appId = computed(() => {
  const id = experiment.value?.app_id
  return typeof id === 'number' && id > 0 ? id : null
})

const loadExp = async () => {
  loading.value = true
  try {
    const res = await getExp(expId)
    experiment.value = res.data
    appVersion.value = null
    syncSnapshot(res.data)
  } catch (e) {
    ElMessage.error(t('message.failedLoadExperiments'))
  } finally {
    loading.value = false
  }
}

const handleUpdate = async () => {
    if (!experiment.value) return
    if (!isExperimentDirty.value) return
    const nameValidation = validateName(experiment.value.name, 'experiment')
    if (!nameValidation.valid) {
      ElMessage.error(t(nameValidation.messageKey, { max: nameValidation.max }))
      return
    }
    const validation = validateExprNodes(experiment.value.filter || [])
    if (!validation.valid) {
      ElMessage.error(validation.messageKey ? t(validation.messageKey) : t('message.invalidFilter'))
      return
    }
    try {
        await updateExp(experiment.value.id, {
            name: nameValidation.normalized,
            description: experiment.value.description,
            version: experiment.value.version,
            filter: experiment.value.filter
        })
        ElMessage.success(t('message.experimentUpdated'))
        experiment.value.name = nameValidation.normalized
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

const handleLayersUpdate = (nextLayers: Experiment['layer']) => {
  if (!experiment.value) return
  experiment.value.layer = nextLayers
}

const bumpExperimentVersion = () => {
  if (!experiment.value) return
  experiment.value.version += 1
}

const resolveAppDetail = async (): Promise<Application | null> => {
  if (!appId.value) return null
  try {
    const appRes = await getApp(appId.value)
    if (typeof appRes.data.version === 'number') {
      appVersion.value = appRes.data.version
      return appRes.data
    }
  } catch (e) {
    return null
  }
  return null
}

const handleDelete = async () => {
  if (!experiment.value) return
  if (!appId.value) {
    ElMessage.error(t('message.appInfoMissing'))
    return
  }
  const resolvedApp = await resolveAppDetail()
  if (!resolvedApp || typeof resolvedApp.version !== 'number') {
    ElMessage.error(t('message.appInfoMissing'))
    return
  }
  try {
    await ElMessageBox.confirm(t('confirm.deleteExperiment'), t('common.warning'), { type: 'warning' })
    await deleteExp(experiment.value.id, {
      app_id: appId.value,
      app_ver: resolvedApp.version,
      version: experiment.value.version
    })
    const nextExperiments = (resolvedApp.experiment || [])
      .filter(item => item.id !== experiment.value?.id)
      .map(item => ({
        id: item.id,
        status: item.status,
        version: item.version,
        name: item.name,
        description: item.description
      }))
    const appSnapshot: AppSnapshot = {
      id: resolvedApp.id,
      name: resolvedApp.name,
      description: resolvedApp.description,
      version: resolvedApp.version + 1,
      experiment: nextExperiments
    }
    ElMessage.success(t('message.experimentDeleted'))
    router.push({
      path: '/',
      query: {
        app_id: String(appId.value),
        refresh: String(Date.now())
      },
      state: {
        appSnapshot
      }
    })
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(t('message.deleteFailed'))
  }
}

const privilegeLabel = (privilege: number) => {
  if (privilege === 1) return t('privilege.read')
  if (privilege === 2) return t('privilege.write')
  if (privilege === 3) return t('privilege.admin')
  return t('privilege.none')
}

const loadPrivileges = async () => {
  if (!appId.value) {
    ElMessage.error(t('message.appInfoMissing'))
    return
  }
  privilegeLoading.value = true
  try {
    const res = await getAppPrivileges(appId.value)
    privileges.value = res.data || []
  } catch (e) {
    ElMessage.error(t('message.failedLoadPrivileges'))
  } finally {
    privilegeLoading.value = false
  }
}

const showPrivilegeDialog = async () => {
  privilegeDialogVisible.value = true
  await loadPrivileges()
}

const submitPrivilege = async () => {
  if (!appId.value) {
    ElMessage.error(t('message.appInfoMissing'))
    return
  }
  const nameValidation = validateName(privilegeForm.value.name, 'user')
  if (!nameValidation.valid) {
    ElMessage.error(t(nameValidation.messageKey, { max: nameValidation.max }))
    return
  }
  try {
    await grantAppPrivilege(appId.value, {
      name: nameValidation.normalized,
      privilege: privilegeForm.value.privilege
    })
    ElMessage.success(t('message.privilegeUpdated'))
    privilegeForm.value = { name: '', privilege: 1 }
    await loadPrivileges()
  } catch (e) {
    ElMessage.error(t('message.operationFailed'))
  }
}

const revokePrivilege = async (name: string) => {
  if (!appId.value) {
    ElMessage.error(t('message.appInfoMissing'))
    return
  }
  try {
    await grantAppPrivilege(appId.value, { name, privilege: 0 })
    ElMessage.success(t('message.privilegeUpdated'))
    await loadPrivileges()
  } catch (e) {
    ElMessage.error(t('message.operationFailed'))
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
        <el-input
          v-model="experiment.name"
          :maxlength="experimentNameMaxLength"
          :placeholder="t('detail.expName')"
          class="exp-name-input"
          :class="{ 'dirty-input': isExperimentNameDirty }"
        />
        <el-input
          v-model="experiment.description"
          :placeholder="t('detail.expDesc')"
          class="exp-desc-input"
          :class="{ 'dirty-input': isExperimentDescriptionDirty }"
        />
        <el-button type="primary" :disabled="!isExperimentDirty" @click="handleUpdate">{{ t('common.update') }}</el-button>
        <el-button type="danger" @click="handleDelete">{{ t('common.delete') }}</el-button>
        <div class="exp-row-right">
          <el-button @click="showPrivilegeDialog">{{ t('detail.appPrivilege') }}</el-button>
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
      <LayerList
        ref="layerListRef"
        :experiment="experiment"
        @update:layers="handleLayersUpdate"
        @bump-experiment-version="bumpExperimentVersion"
      />
    </div>

    <el-dialog v-model="privilegeDialogVisible" :title="t('detail.privilegeTitle')" width="680px">
      <el-form :inline="true" :model="privilegeForm" class="privilege-form">
        <el-form-item :label="t('detail.targetUser')">
          <el-input v-model="privilegeForm.name" :maxlength="userNameMaxLength" />
        </el-form-item>
        <el-form-item :label="t('detail.privilegeLevel')">
          <el-select v-model="privilegeForm.privilege" style="width: 120px">
            <el-option :label="t('privilege.read')" :value="1" />
            <el-option :label="t('privilege.write')" :value="2" />
            <el-option :label="t('privilege.admin')" :value="3" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="submitPrivilege">{{ t('common.confirm') }}</el-button>
        </el-form-item>
      </el-form>

      <el-table :data="privileges" v-loading="privilegeLoading" style="width: 100%">
        <el-table-column prop="name" :label="t('common.name')" />
        <el-table-column :label="t('detail.privilegeLevel')">
          <template #default="{ row }">
            {{ privilegeLabel(row.privilege) }}
          </template>
        </el-table-column>
        <el-table-column prop="grantor" :label="t('detail.grantor')" />
        <el-table-column :label="t('common.operation')" width="120">
          <template #default="{ row }">
            <el-button link type="danger" @click="revokePrivilege(row.name)">{{ t('detail.revoke') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
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
  flex: 0 0 auto;
}
.exp-row :deep(.exp-name-input.el-input) {
  width: 160px;
}
.exp-row :deep(.exp-desc-input.el-input) {
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
.privilege-form {
  margin-bottom: 12px;
}
</style>
