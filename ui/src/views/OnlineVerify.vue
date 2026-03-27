<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getApps, verify } from '@/api'
import type { Application } from '@/types'
import { ElMessage } from 'element-plus'
import { useI18n } from '@/i18n'
import { useRecentApp } from '@/composables/useRecentApp'

const apps = ref<Application[]>([])
const form = ref({
  appid: null as number | null,
  key: '',
  contextStr: '{}'
})
const result = ref<any>(null)
const { t } = useI18n()
const { getRecentAppId, setRecentAppId } = useRecentApp()

onMounted(async () => {
  try {
    const res = await getApps()
    apps.value = res.data
    const rememberedAppId = getRecentAppId()
    if (rememberedAppId && apps.value.some(app => app.id === rememberedAppId)) {
      form.value.appid = rememberedAppId
    }
  } catch (e) {
    if ((e as any)?.response?.status === 401) {
      apps.value = []
      return
    }
    ElMessage.error(t('message.failedLoadApps'))
  }
})

const handleVerify = async () => {
  if (!form.value.appid || !form.value.key) {
    ElMessage.warning(t('message.verifyRequired'))
    return
  }
  setRecentAppId(form.value.appid)
  try {
    const context = JSON.parse(form.value.contextStr)
    const res = await verify({
      appid: form.value.appid,
      key: form.value.key,
      context
    })
    result.value = res.data
  } catch (e) {
    if (e instanceof SyntaxError) {
      ElMessage.error(t('message.verifyContextInvalidJson'))
    } else {
      ElMessage.error(t('message.verifyFailed'))
    }
  }
}
</script>

<template>
  <div class="verify-page">
    <h2>{{ t('verify.title') }}</h2>
    <el-form :model="form" label-width="100px">
      <el-form-item :label="t('verify.application')">
        <el-select v-model="form.appid" :placeholder="t('verify.selectApp')" @change="setRecentAppId">
          <el-option v-for="app in apps" :key="app.id" :label="app.name" :value="app.id" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('verify.key')">
        <el-input v-model="form.key" :placeholder="t('verify.keyPlaceholder')" />
      </el-form-item>
      <el-form-item :label="t('verify.context')">
        <el-input v-model="form.contextStr" type="textarea" :rows="4" :placeholder="t('verify.contextPlaceholder')" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="handleVerify">{{ t('verify.button') }}</el-button>
      </el-form-item>
    </el-form>

    <div v-if="result" class="result-area">
      <h3>{{ t('verify.result') }}</h3>
      <pre>{{ JSON.stringify(result, null, 2) }}</pre>
    </div>

    <!-- Access Token 明文展示属于设计决策：该 token 用于外部只读场景
         （SDK/curl 调用引擎接口时需要直接提供），不用于 admin 会话鉴权，
         此处仅供授权用户查阅以便接入调试，风险等同于 API Key 展示页。 -->
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
