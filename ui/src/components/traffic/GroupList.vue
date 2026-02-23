<template>
  <div class="group-list">
    <el-table :data="groups" size="small">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="Name" />
        <el-table-column prop="share" label="Share (0-1000)" width="120" />
        <el-table-column label="Default" width="80">
            <template #default="{ row }">
                <el-tag v-if="row.is_default" type="success" size="small">Default</el-tag>
            </template>
        </el-table-column>
        <el-table-column label="Config">
             <template #default="{ row }">
                 <el-button type="text" @click="openConfig(row)">Config</el-button>
             </template>
        </el-table-column>
        <el-table-column label="Actions" width="150">
            <template #default="{ row }">
                <el-button type="text" @click="openGroupDialog('edit', row)">Edit</el-button>
                <el-button type="text" @click="openRebalance(row)" v-if="!row.is_default">Rebalance</el-button>
                <el-button type="text" class="danger" v-if="!row.is_default && row.share === 0" @click="handleDelete(row)">Delete</el-button>
            </template>
        </el-table-column>
    </el-table>
    <div class="footer">
        <el-button size="small" @click="openGroupDialog('create')">Add Group</el-button>
    </div>

    <!-- Group Dialog -->
    <el-dialog v-model="dialogVisible" :title="dialogType === 'create' ? 'Add Group' : 'Edit Group'">
        <el-form :model="form" label-width="100px">
            <el-form-item label="Name">
                <el-input v-model="form.name" />
            </el-form-item>
            <el-form-item label="Description">
                <el-input v-model="form.description" />
            </el-form-item>
            <el-form-item label="Force Hit (Keys)">
                <el-select v-model="form.force_hit" multiple filterable allow-create default-first-option placeholder="Enter keys">
                </el-select>
            </el-form-item>
            <el-form-item label="Config ID" v-if="dialogType === 'edit'">
                <el-input v-model="form.cfg_id" disabled />
            </el-form-item>
        </el-form>
        <template #footer>
            <el-button @click="dialogVisible = false">Cancel</el-button>
            <el-button type="primary" @click="handleSave">Save</el-button>
        </template>
    </el-dialog>

    <!-- Rebalance Dialog (Group Share) -->
    <el-dialog v-model="rebalanceVisible" title="Adjust Group Share" width="400px">
        <el-form>
            <el-form-item label="New Share">
                <el-input-number v-model="rebalanceShare" :min="0" :max="1000" />
            </el-form-item>
            <p>Remaining share will be assigned to Default group.</p>
        </el-form>
        <template #footer>
            <el-button @click="rebalanceVisible = false">Cancel</el-button>
            <el-button type="primary" @click="handleRebalance">Confirm</el-button>
        </template>
    </el-dialog>

    <!-- Config Dialog -->
    <el-dialog v-model="configVisible" title="Group Configuration" width="60%">
        <div class="config-actions">
            <el-button type="primary" @click="showCreateConfig = true">New Config</el-button>
        </div>
        
        <div v-if="showCreateConfig" class="create-config">
            <el-input v-model="newConfigContent" type="textarea" :rows="10" placeholder="JSON config content" />
            <div class="mt-2">
                <el-button @click="showCreateConfig = false">Cancel</el-button>
                <el-button type="primary" @click="handleCreateConfig">Save & Apply</el-button>
            </div>
        </div>
        
        <el-table :data="configHistory" height="300px" style="margin-top: 10px">
            <el-table-column prop="id" label="ID" width="80" />
            <el-table-column prop="create_time" label="Time">
                 <template #default="{ row }">
                     {{ new Date(row.create_time * 1000).toLocaleString() }}
                 </template>
            </el-table-column>
            <el-table-column prop="content" label="Content" show-overflow-tooltip />
            <el-table-column label="Action" width="100">
                <template #default="{ row }">
                     <el-button type="text" @click="applyConfig(row)">Apply</el-button>
                </template>
            </el-table-column>
        </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { createGrp, updateGrp, deleteGrp, rebalanceSeg, getGrpCfg, createGrpCfg } from '@/api'
import type { Segment, Group, Config } from '@/api/types'
import { ElMessage, ElMessageBox } from 'element-plus'

const props = defineProps<{
  segment: Segment
}>()

const emit = defineEmits(['refresh'])

const groups = computed(() => props.segment.group || [])

// Group Dialog
const dialogVisible = ref(false)
const dialogType = ref<'create' | 'edit'>('create')
const form = ref({ name: '', description: '', force_hit: [] as string[], cfg_id: 0 })
const currentGroup = ref<Group | null>(null)

const openGroupDialog = (type: 'create' | 'edit', row?: Group) => {
    dialogType.value = type
    if (type === 'edit' && row) {
        currentGroup.value = row
        form.value = {
            name: row.name,
            description: row.description || '',
            force_hit: row.force_hit || [],
            cfg_id: row.cfg_id
        }
    } else {
        currentGroup.value = null
        form.value = { name: '', description: '', force_hit: [], cfg_id: 0 }
    }
    dialogVisible.value = true
}

const handleSave = async () => {
    try {
        if (dialogType.value === 'create') {
            await createGrp({
                seg_id: props.segment.id,
                seg_ver: props.segment.version!,
                name: form.value.name,
                description: form.value.description,
                share: 0, // Initial share 0
                force_hit: form.value.force_hit
            })
            ElMessage.success('Group created')
        } else if (currentGroup.value) {
            await updateGrp(currentGroup.value.id, {
                name: form.value.name,
                description: form.value.description,
                share: currentGroup.value.share,
                is_default: currentGroup.value.is_default,
                version: currentGroup.value.version,
                cfg_id: currentGroup.value.cfg_id,
                force_hit: form.value.force_hit
            })
            ElMessage.success('Group updated')
        }
        dialogVisible.value = false
        emit('refresh')
    } catch (e) {
        // ignore
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
        ElMessage.success('Group deleted')
        emit('refresh')
    } catch (e) {
        // ignore
    }
}

// Rebalance
const rebalanceVisible = ref(false)
const rebalanceShare = ref(0)
const rebalanceGroup = ref<Group | null>(null)

const openRebalance = (grp: Group) => {
    rebalanceGroup.value = grp
    rebalanceShare.value = grp.share
    rebalanceVisible.value = true
}

const handleRebalance = async () => {
    if (!rebalanceGroup.value) return
    try {
        await rebalanceSeg(props.segment.id, {
            version: props.segment.version!,
            grp_id: rebalanceGroup.value.id,
            share: rebalanceShare.value
        })
        ElMessage.success('Share updated')
        rebalanceVisible.value = false
        emit('refresh')
    } catch (e) {
        // ignore
    }
}

// Config
const configVisible = ref(false)
const activeGroup = ref<Group | null>(null)
const configHistory = ref<Config[]>([])
const showCreateConfig = ref(false)
const newConfigContent = ref('')

const openConfig = async (grp: Group) => {
    activeGroup.value = grp
    configVisible.value = true
    showCreateConfig.value = false
    await loadConfigs(grp.id)
}

const loadConfigs = async (grpId: number) => {
    try {
        const res = await getGrpCfg(grpId)
        configHistory.value = res.data || []
    } catch (e) {
        // ignore
    }
}

const handleCreateConfig = async () => {
    if (!activeGroup.value) return
    try {
        const res = await createGrpCfg(activeGroup.value.id, newConfigContent.value)
        // Bind to group immediately? 
        // Create config just creates it. We need to update group to point to it.
        // Wait, API doc says: "Create a new config record for a group."
        // And Group has cfg_id.
        // Usually we want to apply it.
        // Let's apply it by updating group.
        const newCfgId = res.data.id
        await updateGrp(activeGroup.value.id, {
             ...activeGroup.value,
             cfg_id: newCfgId,
             force_hit: activeGroup.value.force_hit || [] // Ensure force_hit is passed
        })
        ElMessage.success('Config created and applied')
        showCreateConfig.value = false
        newConfigContent.value = ''
        emit('refresh') // Refresh parent to see new cfg_id
        configVisible.value = false
    } catch (e) {
        ElMessage.error('Failed to create config')
    }
}

const applyConfig = async (cfg: Config) => {
    if (!activeGroup.value) return
    try {
        await updateGrp(activeGroup.value.id, {
             ...activeGroup.value,
             cfg_id: cfg.id,
             force_hit: activeGroup.value.force_hit || []
        })
        ElMessage.success('Config applied')
        emit('refresh')
        configVisible.value = false
    } catch (e) {
         ElMessage.error('Failed to apply config')
    }
}
</script>

<style scoped>
.footer {
    margin-top: 10px;
}
.danger {
    color: #f56c6c;
}
.config-actions {
    margin-bottom: 10px;
}
.mt-2 {
    margin-top: 10px;
}
</style>
