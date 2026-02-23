<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { getExp, updateExp, shuffleExp } from '@/api'
import type { Experiment } from '@/api/types'
import { ElMessage } from 'element-plus'
import LayerList from '@/components/traffic/LayerList.vue'
import FilterEditor from '@/components/FilterEditor.vue'

const route = useRoute()
const expId = Number(route.params.id)
const experiment = ref<Experiment | null>(null)
const loading = ref(false)

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

onMounted(() => {
  loadExp()
})
</script>

<template>
  <div class="exp-detail-page" v-if="experiment" v-loading="loading">
    <div class="header">
        <h2>{{ experiment.name }}</h2>
        <div class="actions">
            <el-button @click="handleShuffle">Shuffle Seed</el-button>
            <el-button type="primary" @click="handleUpdate">Save Changes</el-button>
        </div>
    </div>
    
    <el-descriptions border column="2">
        <el-descriptions-item label="ID">{{ experiment.id }}</el-descriptions-item>
        <el-descriptions-item label="Status">
            <el-tag :type="experiment.status ? 'success' : 'info'">{{ experiment.status ? 'Active' : 'Stopped' }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="Description" span="2">
            <el-input v-model="experiment.description" />
        </el-descriptions-item>
    </el-descriptions>

    <div class="section">
        <h3>Filter Rules</h3>
        <FilterEditor v-model="experiment.filter" />
    </div>

    <div class="section">
        <h3>Traffic Allocation (Layers)</h3>
        <LayerList :experiment="experiment" @refresh="loadExp" />
    </div>
  </div>
</template>

<style scoped>
.header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
}
.section {
    margin-top: 30px;
}
</style>
