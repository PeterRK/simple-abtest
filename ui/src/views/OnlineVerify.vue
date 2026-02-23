<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getApps, verify } from '@/api'
import type { Application } from '@/types'
import { ElMessage } from 'element-plus'

const apps = ref<Application[]>([])
const form = ref({
  appid: null as number | null,
  key: '',
  contextStr: '{}'
})
const result = ref<any>(null)

onMounted(async () => {
  try {
    const res = await getApps()
    apps.value = res.data
  } catch (e) {
    ElMessage.error('Failed to load apps')
  }
})

const handleVerify = async () => {
  if (!form.value.appid || !form.value.key) {
    ElMessage.warning('App ID and Key are required')
    return
  }
  try {
    const context = JSON.parse(form.value.contextStr)
    const res = await verify({
      appid: form.value.appid,
      key: form.value.key,
      context
    })
    result.value = res.data
  } catch (e) {
    ElMessage.error('Verification failed or invalid JSON')
  }
}
</script>

<template>
  <div class="verify-page">
    <h2>Online Verify</h2>
    <el-form :model="form" label-width="100px">
      <el-form-item label="Application">
        <el-select v-model="form.appid" placeholder="Select App">
          <el-option v-for="app in apps" :key="app.id" :label="app.name" :value="app.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="Key">
        <el-input v-model="form.key" placeholder="User ID or Device ID" />
      </el-form-item>
      <el-form-item label="Context">
        <el-input v-model="form.contextStr" type="textarea" :rows="4" placeholder='JSON format, e.g. {"country": "US"}' />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="handleVerify">Verify</el-button>
      </el-form-item>
    </el-form>

    <div v-if="result" class="result-area">
      <h3>Result</h3>
      <pre>{{ JSON.stringify(result, null, 2) }}</pre>
    </div>
  </div>
</template>

<style scoped>
.verify-page {
  max-width: 800px;
  margin: 0 auto;
}
.result-area {
  margin-top: 20px;
  background: #f5f7fa;
  padding: 15px;
  border-radius: 4px;
}
</style>
