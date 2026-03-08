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
          <el-button size="small" @click.stop="openRebalance(grp)" v-if="!grp.is_default">扩缩容</el-button>
          <el-button size="small" type="danger" v-if="!grp.is_default && grp.share === 0" @click.stop="handleDelete(grp)">删除</el-button>
        </div>
      </div>
    </div>

    <div class="group-footer">
      <el-button size="small" type="primary" @click="openGroupDialog('create')">新增Group</el-button>
      <el-button size="small" @click="handleShuffle">流量打散</el-button>
    </div>

    <div v-if="selectedGroupDetail" class="group-detail">
      <div class="group-detail-header">
        <el-input v-model="groupForm.name" placeholder="组名" />
        <el-button type="primary" @click="handleUpdate">更新</el-button>
        <div class="group-detail-actions">
          <el-button @click="handleSearchConfigs">配置查找</el-button>
          <div class="config-days-input">
            <el-input-number v-model="configDays" :min="0" :max="3650" size="small" />
            <span>天前</span>
          </div>
        </div>
      </div>
      <div class="group-config-area">
        <el-input v-model="forceHitText" type="textarea" :rows="8" placeholder="强制命中 key，每行一个" />
        <el-input v-model="newConfigContent" type="textarea" :rows="8" placeholder="配置内容" />
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
            <el-table-column prop="id" label="配置ID" width="120" />
            <el-table-column prop="stamp" label="更新时间" />
          </el-table>
        </div>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" title="新增Group" width="360px">
      <el-form :model="form" label-width="40px">
        <el-form-item label="名称">
          <el-input v-model="form.name" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleCreate">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="rebalanceVisible" title="扩缩容" width="400px">
      <el-form>
        <el-form-item label="流量百分比">
          <el-input-number v-model="rebalancePercent" :min="0" :max="100" :step="0.1" :precision="1" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rebalanceVisible = false">取消</el-button>
        <el-button type="primary" @click="handleRebalance">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { createGrp, updateGrp, deleteGrp, rebalanceSeg, getGrpCfg, createGrpCfg, shuffleSeg, getGroup, getConfig } from '@/api'
import type { Segment, Group, Config } from '@/api/types'
import { ElMessage, ElMessageBox } from 'element-plus'

const props = defineProps<{
  segment: Segment
}>()

const groups = computed(() => props.segment.group || [])
const selectedGroupId = ref<number | null>(null)
const selectedGroupDetail = ref<Group | null>(null)
const groupForm = ref({ name: '' })
const forceHitText = ref('')
const newConfigContent = ref('')
const currentConfigContent = ref('')
const configHistory = ref<Config[]>([])
const selectedConfigId = ref<number | null>(null)
const configDays = ref(7)
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

const resetGroupState = () => {
  selectedGroupDetail.value = null
  groupForm.value = { name: '' }
  forceHitText.value = ''
  newConfigContent.value = ''
  currentConfigContent.value = ''
  configHistory.value = []
  selectedConfigId.value = null
}

const loadGroupDetail = async (grpId: number) => {
  try {
    const res = await getGroup(grpId)
    selectedGroupDetail.value = res.data
    groupForm.value = { name: res.data.name }
    forceHitText.value = (res.data.force_hit || []).join('\n')
    newConfigContent.value = res.data.config || ''
    currentConfigContent.value = res.data.config || ''
    selectedConfigId.value = res.data.cfg_id ?? null
    configHistory.value = res.data.cfg_id ? [{ id: res.data.cfg_id, stamp: res.data.cfg_stamp }] : []
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
  }
}

const handleCreate = async () => {
  try {
    const res = await createGrp({
      seg_id: props.segment.id,
      seg_ver: props.segment.version!,
      name: form.value.name
    })
    if (!props.segment.group) props.segment.group = []
    props.segment.group.push(res.data)
    bumpSegmentVersion()
    ElMessage.success('Group created')
    dialogVisible.value = false
  } catch (e) {
    // ignore
  }
}

const handleUpdate = async () => {
  if (!selectedGroupDetail.value) return
  try {
    const forceHit = buildForceHitList(forceHitText.value)
    const currentConfigId = selectedGroupDetail.value.cfg_id
    const activeConfigId = selectedConfigId.value ?? currentConfigId
    if (activeConfigId == null) return
    const hasNameChange = selectedGroupDetail.value.name !== groupForm.value.name
    const hasForceHitChange = !isSameList(selectedGroupDetail.value.force_hit || [], forceHit)
    const hasConfigChange = activeConfigId !== currentConfigId
    const hasContentChange = newConfigContent.value !== currentConfigContent.value
    if (!hasNameChange && !hasForceHitChange && !hasConfigChange && !hasContentChange) return
    let nextConfigId = activeConfigId
    let nextConfigContent = currentConfigContent.value
    let createdConfigId: number | null = null
    if (activeConfigId !== currentConfigId) {
      await updateGrp(selectedGroupDetail.value.id, {
        name: groupForm.value.name,
        version: selectedGroupDetail.value.version,
        cfg_id: activeConfigId,
        force_hit: forceHit
      })
    } else {
      if (hasContentChange) {
        const res = await createGrpCfg(selectedGroupDetail.value.id, newConfigContent.value)
        createdConfigId = res.data.id
        nextConfigId = res.data.id
        nextConfigContent = newConfigContent.value
        await updateGrp(selectedGroupDetail.value.id, {
          name: groupForm.value.name,
          version: selectedGroupDetail.value.version,
          cfg_id: res.data.id,
          force_hit: forceHit
        })
      } else {
        await updateGrp(selectedGroupDetail.value.id, {
          name: groupForm.value.name,
          version: selectedGroupDetail.value.version,
          cfg_id: currentConfigId ?? 0,
          force_hit: forceHit
        })
        nextConfigId = currentConfigId ?? 0
      }
    }
    ElMessage.success('Group updated')
    const nextVersion = selectedGroupDetail.value.version + 1
    selectedGroupDetail.value = {
      ...selectedGroupDetail.value,
      name: groupForm.value.name,
      cfg_id: nextConfigId,
      force_hit: forceHit,
      version: nextVersion,
      config: nextConfigContent
    }
    currentConfigContent.value = nextConfigContent
    selectedConfigId.value = nextConfigId
    if (createdConfigId != null && !configHistory.value.some(item => item.id === createdConfigId)) {
      configHistory.value = [{ id: createdConfigId }, ...configHistory.value]
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
    ElMessage.error('更新失败，请手动刷新后重试')
  }
}

const handleDelete = async (grp: Group) => {
    try {
        await ElMessageBox.confirm('Delete this group?', 'Warning', { type: 'warning' })
        await deleteGrp(grp.id, {
            seg_id: props.segment.id,
            seg_ver: props.segment.version!,
            version: grp.version
        })
        if (props.segment.group) {
          props.segment.group = props.segment.group.filter(item => item.id !== grp.id)
        }
        if (selectedGroupId.value === grp.id) {
          selectedGroupId.value = null
          resetGroupState()
        }
        bumpSegmentVersion()
        ElMessage.success('Group deleted')
    } catch (e) {
        // ignore
    }
}

const handleShuffle = async () => {
  try {
    await shuffleSeg(props.segment.id)
    ElMessage.success('Segment seed shuffled')
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
    return { valid: false, message: '未找到目标组或默认组，请刷新后重试' }
  }
  const minShare = Math.max(0, target.share + defaultGroup.share - 1000)
  const maxShare = Math.min(1000, target.share + defaultGroup.share)
  if (!Number.isFinite(nextShare) || nextShare < minShare || nextShare > maxShare) {
    return {
      valid: false,
      message: `流量不合法，仅可在 ${formatSharePercent(minShare)} ~ ${formatSharePercent(maxShare)} 范围内调整`
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
        await rebalanceSeg(props.segment.id, {
            version: props.segment.version!,
            grp_id: rebalanceGroup.value.id,
            share: nextShare
        })
        applyLocalRebalance(rebalanceGroup.value.id, nextShare)
        ElMessage.success('Share updated')
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
    const res = await getGrpCfg(grpId, begin)
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
  selectedConfigId.value = cfg.id
  try {
    const res = await getConfig(cfg.id)
    newConfigContent.value = normalizeConfigContent(res.data)
  } catch (e) {
    // ignore
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
