<template>
  <div class="group-list">
    <div class="group-row">
      <div
        v-for="grp in groups"
        :key="grp.id"
        class="group-card"
        :class="{ active: selectedGroupId === grp.id }"
        @click="selectGroup(grp)"
      >
        <div class="group-title">{{ grp.name }} ({{ formatSharePercent(grp.share) }})</div>
        <div class="group-actions">
          <el-button size="small" @click.stop="openRebalance(grp)" v-if="!grp.is_default">{{ t('group.rebalance') }}</el-button>
          <el-button size="small" type="danger" v-if="!grp.is_default && grp.share === 0" @click.stop="handleDelete(grp)">{{ t('common.delete') }}</el-button>
        </div>
      </div>
    </div>

    <div class="group-footer">
      <el-button size="small" type="primary" @click="openGroupDialog('create')">{{ t('group.createGroup') }}</el-button>
      <el-button size="small" @click="handleShuffle">{{ t('group.shuffle') }}</el-button>
    </div>

    <div v-if="selectedGroupDetail" class="group-detail">
      <div class="group-detail-header">
        <el-input
          v-model="groupForm.name"
          :placeholder="t('group.groupName')"
          :class="{ 'dirty-input': isGroupNameDirty }"
        />
        <el-button type="primary" :disabled="!isGroupDirty" @click="handleUpdate">{{ t('common.update') }}</el-button>
        <div class="group-detail-actions">
          <el-button @click="handleSearchConfigs">{{ t('group.searchConfig') }}</el-button>
          <div class="config-days-input">
            <el-input-number v-model="configDays" :min="0" :max="3650" size="small" />
            <span>{{ t('group.dayAgo') }}</span>
          </div>
        </div>
      </div>
      <div class="group-config-area">
        <el-input
          v-model="forceHitText"
          type="textarea"
          :rows="8"
          :placeholder="t('group.forceHitPlaceholder')"
          :class="{ 'dirty-input': isForceHitDirty }"
        />
        <el-input
          v-model="newConfigContent"
          type="textarea"
          :rows="8"
          :placeholder="t('group.configPlaceholder')"
          :class="{ 'dirty-input': isConfigContentDirty, 'config-content-dirty': isConfigContentDirty }"
        />
        <div class="config-history">
          <el-table
            :data="displayConfigs"
            size="small"
            border
            highlight-current-row
            row-key="id"
            :current-row-key="selectedConfigId"
            :row-class-name="configRowClassName"
            @current-change="handleSelectConfig"
          >
            <el-table-column prop="id" :label="t('group.configId')" width="120" />
            <el-table-column prop="stamp" :label="t('group.updateTime')" />
          </el-table>
        </div>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" :title="t('group.createTitle')" width="360px">
      <el-form :model="form" label-width="40px">
        <el-form-item :label="t('common.name')">
          <el-input v-model="form.name" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleCreate">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="rebalanceVisible" :title="t('group.rebalanceTitle')" width="400px">
      <el-form>
        <el-form-item :label="t('layer.sharePercent')">
          <el-input-number v-model="rebalancePercent" :min="0" :max="100" :step="0.1" :precision="1" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rebalanceVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleRebalance">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { createGroup, updateGroup, deleteGroup, rebalanceSegment, getConfigs, createConfig, shuffleSegment, getGroup, getConfig } from '@/api'
import type { Segment, Group, Config } from '@/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from '@/i18n'

const props = defineProps<{
  segment: Segment
}>()

const groups = computed(() => props.segment.group || [])
const selectedGroupId = ref<number | null>(null)
const selectedGroupDetail = ref<Group | null>(null)
const groupForm = ref({ name: '' })
const forceHitText = ref('')
const newConfigContent = ref('')
const selectedConfigContent = ref('')
const configHistory = ref<Config[]>([])
const selectedConfigId = ref<number | null>(null)
const configDays = ref(7)
const configContentCache = new Map<number, string>()
const configLoadInFlight = new Map<number, Promise<string>>()
const { t } = useI18n()
const buildForceHitList = (text: string) =>
  text
    .split('\n')
    .map(item => item.trim())
    .filter(item => item.length > 0)

const isSameList = (left: string[], right: string[]) => {
  if (left.length !== right.length) return false
  for (let i = 0; i < left.length; i++) {
    if (left[i] !== right[i]) return false
  }
  return true
}

const isConfigContentDirty = computed(() => (newConfigContent.value || '') !== (selectedConfigContent.value || ''))
const isGroupNameDirty = computed(() => {
  if (!selectedGroupDetail.value) return false
  return selectedGroupDetail.value.name !== groupForm.value.name
})

const isForceHitDirty = computed(() => {
  if (!selectedGroupDetail.value) return false
  return !isSameList(selectedGroupDetail.value.force_hit || [], buildForceHitList(forceHitText.value))
})

const isConfigSelectionDirty = computed(() => {
  if (!selectedGroupDetail.value) return false
  const originalConfigId = selectedGroupDetail.value.cfg_id ?? 0
  const activeConfigId = selectedConfigId.value ?? originalConfigId
  return activeConfigId !== originalConfigId
})

const isGroupDirty = computed(
  () => isGroupNameDirty.value || isForceHitDirty.value || isConfigSelectionDirty.value || isConfigContentDirty.value
)

const resetGroupState = () => {
  selectedGroupDetail.value = null
  groupForm.value = { name: '' }
  forceHitText.value = ''
  newConfigContent.value = ''
  selectedConfigContent.value = ''
  configHistory.value = []
  selectedConfigId.value = null
  configContentCache.clear()
  configLoadInFlight.clear()
}

const loadGroupDetail = async (grpId: number) => {
  try {
    const res = await getGroup(grpId)
    selectedGroupDetail.value = res.data
    groupForm.value = { name: res.data.name }
    forceHitText.value = (res.data.force_hit || []).join('\n')
    newConfigContent.value = res.data.config || ''
    selectedConfigContent.value = res.data.config || ''
    selectedConfigId.value = res.data.cfg_id ?? null
    configHistory.value = res.data.cfg_id ? [{ id: res.data.cfg_id, stamp: res.data.cfg_stamp }] : []
    configContentCache.clear()
    configLoadInFlight.clear()
    if (res.data.cfg_id != null && res.data.cfg_id > 0) {
      configContentCache.set(res.data.cfg_id, res.data.config || '')
    }
  } catch (e) {
    selectedGroupDetail.value = null
  }
}

watch(
  () => props.segment.id,
  () => {
    selectedGroupId.value = null
    resetGroupState()
  }
)

const selectGroup = (grp: Group) => {
  selectedGroupId.value = grp.id
}

watch(
  () => selectedGroupId.value,
  (val) => {
    if (!val) {
      resetGroupState()
      return
    }
    configHistory.value = []
    newConfigContent.value = ''
    loadGroupDetail(val)
  }
)

const dialogVisible = ref(false)
const form = ref({ name: '' })

const openGroupDialog = (type: 'create') => {
  if (type === 'create') {
    form.value = { name: '' }
    dialogVisible.value = true
  }
}

const bumpSegmentVersion = () => {
  if (typeof props.segment.version === 'number') {
    props.segment.version += 1
    return
  }
  props.segment.version = 1
}

const handleCreate = async () => {
  try {
    const segmentVersion = props.segment.version ?? 0
    const res = await createGroup({
      seg_id: props.segment.id,
      seg_ver: segmentVersion,
      name: form.value.name
    })
    if (!props.segment.group) props.segment.group = []
    props.segment.group.push({
      ...res.data,
      version: res.data.version ?? 0
    })
    bumpSegmentVersion()
    ElMessage.success(t('message.groupCreated'))
    dialogVisible.value = false
  } catch (e) {
    // ignore
  }
}

const handleUpdate = async () => {
  if (!selectedGroupDetail.value) return
  try {
    if (!isGroupDirty.value) return
    const forceHit = buildForceHitList(forceHitText.value)
    const originalConfigId = selectedGroupDetail.value.cfg_id ?? 0
    const activeConfigId = selectedConfigId.value ?? originalConfigId
    const hasNameChange = isGroupNameDirty.value
    const hasForceHitChange = isForceHitDirty.value
    const nextContent = newConfigContent.value || ''
    const activeContent = selectedConfigContent.value || ''
    const hasContentChange = isConfigContentDirty.value
    const hasConfigChange = isConfigSelectionDirty.value
    let nextConfigId = activeConfigId
    let nextConfigContent = activeContent
    let createdConfigId: number | null = null
    let createdConfigStamp = ''
    if (hasContentChange) {
      const res = await createConfig(selectedGroupDetail.value.id, nextContent)
      createdConfigId = res.data.id
      createdConfigStamp = res.data.stamp || ''
      nextConfigId = res.data.id
      nextConfigContent = nextContent
      await updateGroup(selectedGroupDetail.value.id, {
        name: groupForm.value.name,
        version: selectedGroupDetail.value.version,
        cfg_id: res.data.id,
        force_hit: forceHit
      })
    } else {
      await updateGroup(selectedGroupDetail.value.id, {
        name: groupForm.value.name,
        version: selectedGroupDetail.value.version,
        cfg_id: activeConfigId,
        force_hit: forceHit
      })
      nextConfigId = activeConfigId
      nextConfigContent = activeContent
    }
    ElMessage.success(t('message.groupUpdated'))
    const nextVersion = selectedGroupDetail.value.version + 1
    selectedGroupDetail.value = {
      ...selectedGroupDetail.value,
      name: groupForm.value.name,
      cfg_id: nextConfigId,
      cfg_stamp: createdConfigId != null ? createdConfigStamp : selectedGroupDetail.value.cfg_stamp,
      force_hit: forceHit,
      version: nextVersion,
      config: nextConfigContent
    }
    selectedConfigContent.value = nextConfigContent
    selectedConfigId.value = nextConfigId
    configContentCache.set(nextConfigId, nextConfigContent)
    if (createdConfigId != null) {
      configHistory.value = [{ id: createdConfigId, stamp: createdConfigStamp }]
    }
    if (props.segment.group) {
      const target = props.segment.group.find(item => item.id === selectedGroupDetail.value?.id)
      if (target) {
        target.name = groupForm.value.name
        target.cfg_id = nextConfigId
        target.force_hit = forceHit
        target.version = nextVersion
      }
    }
  } catch (e) {
    ElMessage.error(t('message.updateFailedRefresh'))
  }
}

const handleDelete = async (grp: Group) => {
    try {
        const segmentVersion = props.segment.version ?? 0
        const groupVersion = grp.version ?? 0
        await ElMessageBox.confirm(t('confirm.deleteGroup'), t('common.warning'), { type: 'warning' })
        await deleteGroup(grp.id, {
            seg_id: props.segment.id,
            seg_ver: segmentVersion,
            version: groupVersion
        })
        if (props.segment.group) {
          props.segment.group = props.segment.group.filter(item => item.id !== grp.id)
        }
        if (selectedGroupId.value === grp.id) {
          selectedGroupId.value = null
          resetGroupState()
        }
        bumpSegmentVersion()
        ElMessage.success(t('message.groupDeleted'))
    } catch (e) {
        // ignore
    }
}

const handleShuffle = async () => {
  try {
    await shuffleSegment(props.segment.id)
    ElMessage.success(t('message.segmentSeedShuffled'))
  } catch (e) {
    // ignore
  }
}

// Rebalance
const rebalanceVisible = ref(false)
const rebalancePercent = ref(0)
const rebalanceGroup = ref<Group | null>(null)
const formatSharePercent = (share: number) => `${(share / 10).toFixed(1)}%`

const validateRebalance = (targetGroupId: number, nextShare: number) => {
  const list = props.segment.group || []
  const target = list.find(item => item.id === targetGroupId)
  const defaultGroup = list.find(item => item.is_default)
  if (!target || !defaultGroup) {
    return { valid: false, message: t('message.missingTargetGroup') }
  }
  const minShare = Math.max(0, target.share + defaultGroup.share - 1000)
  const maxShare = Math.min(1000, target.share + defaultGroup.share)
  if (!Number.isFinite(nextShare) || nextShare < minShare || nextShare > maxShare) {
    return {
      valid: false,
      message: t('message.invalidShareRange', { min: formatSharePercent(minShare), max: formatSharePercent(maxShare) })
    }
  }
  return { valid: true }
}

const applyLocalRebalance = (targetGroupId: number, nextShare: number) => {
  const list = props.segment.group || []
  const target = list.find(item => item.id === targetGroupId)
  if (!target) return
  const prevShare = target.share
  const delta = nextShare - prevShare
  const defaultGroup = list.find(item => item.is_default)
  target.share = nextShare
  if (defaultGroup && defaultGroup.id !== targetGroupId) {
    defaultGroup.share = Math.max(0, Math.min(1000, defaultGroup.share - delta))
  }
  if (selectedGroupDetail.value?.id === targetGroupId) {
    selectedGroupDetail.value.share = nextShare
  } else if (defaultGroup && selectedGroupDetail.value?.id === defaultGroup.id) {
    selectedGroupDetail.value.share = defaultGroup.share
  }
  bumpSegmentVersion()
}

const openRebalance = (grp: Group) => {
    rebalanceGroup.value = grp
    rebalancePercent.value = grp.share / 10
    rebalanceVisible.value = true
}

const handleRebalance = async () => {
    if (!rebalanceGroup.value) return
    const nextShare = Math.round(rebalancePercent.value * 10)
    const validation = validateRebalance(rebalanceGroup.value.id, nextShare)
    if (!validation.valid) {
      ElMessage.error(validation.message)
      return
    }
    try {
        const segmentVersion = props.segment.version ?? 0
        await rebalanceSegment(props.segment.id, {
            version: segmentVersion,
            grp_id: rebalanceGroup.value.id,
            share: nextShare
        })
        applyLocalRebalance(rebalanceGroup.value.id, nextShare)
        ElMessage.success(t('message.shareUpdated'))
        rebalanceVisible.value = false
    } catch (e) {
        // ignore
    }
}

const displayConfigs = computed(() => {
  if (!selectedGroupDetail.value) return []
  const list = configHistory.value || []
  const currentId = selectedGroupDetail.value.cfg_id
  if (!currentId) return list
  const stamp = selectedGroupDetail.value.cfg_stamp
  const merged = list.map(item => (item.id === currentId ? { ...item, stamp: item.stamp ?? stamp } : item))
  if (merged.some(item => item.id === currentId)) return merged
  return [{ id: currentId, stamp }, ...merged]
})

const toBeginTimestamp = () => {
  const days = Number(configDays.value) || 0
  if (days <= 0) return undefined
  return Math.floor(Date.now() / 1000) - days * 86400
}

const loadConfigs = async (grpId: number, begin?: number) => {
  try {
    const res = await getConfigs(grpId, begin)
    configHistory.value = res.data || []
  } catch (e) {
    // ignore
  }
}

const handleSearchConfigs = async () => {
  if (!selectedGroupDetail.value) return
  const begin = toBeginTimestamp()
  await loadConfigs(selectedGroupDetail.value.id, begin)
}

const normalizeConfigContent = (content: unknown) => {
  if (typeof content === 'string') return content
  try {
    return JSON.stringify(content, null, 2)
  } catch (e) {
    return String(content)
  }
}

const handleSelectConfig = async (cfg: Config | null) => {
  if (!cfg) return
  const groupDetail = selectedGroupDetail.value
  if (!groupDetail) return
  const cfgId = cfg.id
  selectedConfigId.value = cfgId
  const cached = configContentCache.get(cfgId)
  if (cached !== undefined) {
    selectedConfigContent.value = cached
    newConfigContent.value = cached
    return
  }
  let pending = configLoadInFlight.get(cfgId)
  if (!pending) {
    pending = getConfig(groupDetail.id, cfgId).then(res => normalizeConfigContent(res.data))
    configLoadInFlight.set(cfgId, pending)
  }
  try {
    const content = await pending
    configContentCache.set(cfgId, content)
    if (selectedConfigId.value === cfgId) {
      selectedConfigContent.value = content
      newConfigContent.value = content
    }
  } catch (e) {
    // ignore
  } finally {
    if (configLoadInFlight.get(cfgId) === pending) {
      configLoadInFlight.delete(cfgId)
    }
  }
}

const configRowClassName = ({ row }: { row: Config }) => {
  if (row.id === selectedGroupDetail.value?.cfg_id) return 'config-row-current'
  return ''
}
</script>

<style scoped>
.group-row {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.group-card {
  flex: 1 1 200px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  padding: 10px;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: center;
  justify-content: center;
  text-align: center;
}
.group-card.active {
  border-color: #409eff;
  box-shadow: 0 0 0 1px #409eff inset;
}
.group-title {
  font-weight: 600;
}
.group-actions {
  display: flex;
  gap: 6px;
  justify-content: center;
}
.group-footer {
  display: flex;
  justify-content: space-between;
  margin-top: 10px;
}
.group-detail {
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.group-detail-header {
  display: flex;
  align-items: center;
  gap: 10px;
}
.group-detail-header :deep(.el-input) {
  width: 220px;
}
.group-detail-actions {
  margin-left: auto;
  display: flex;
  gap: 10px;
}
.config-days-input {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.group-config-area {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}
.group-config-area :deep(.el-textarea__inner) {
  min-height: 220px;
}
.group-config-area :deep(.config-content-dirty .el-textarea__inner) {
  color: #e6a23c;
}
.config-history {
  min-width: 260px;
  flex: 0 0 320px;
}
.config-history :deep(.el-table) {
  width: 100%;
}
.config-row-current td {
  background-color: #f0f9eb;
}
.config-history {
  display: flex;
  justify-content: flex-end;
}
.config-history-list {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
</style>
