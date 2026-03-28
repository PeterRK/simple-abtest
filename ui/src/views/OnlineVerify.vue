<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { getApps, verify } from '@/api'
import type { Application } from '@/types'
import { ElMessage } from 'element-plus'
import { useI18n } from '@/i18n'
import { useRecentApp } from '@/composables/useRecentApp'

const apps = ref<Application[]>([])
type ContextEntry = {
  id: number
  key: string
  value: string
}

const form = ref({
  appid: null as number | null,
  key: ''
})
const contextEntries = ref<ContextEntry[]>([
  { id: 1, key: '', value: '' }
])
const result = ref<any>(null)
const { t } = useI18n()
const { getRecentAppId, setRecentAppId } = useRecentApp()
let nextContextEntryId = 2

const normalizedContextKeys = computed(() =>
  contextEntries.value.map(entry => entry.key.trim())
)

const duplicateContextKeySet = computed(() => {
  const counts = new Map<string, number>()
  for (const key of normalizedContextKeys.value) {
    if (!key) continue
    counts.set(key, (counts.get(key) || 0) + 1)
  }
  return new Set(
    Array.from(counts.entries())
      .filter(([, count]) => count > 1)
      .map(([key]) => key)
  )
})

const hasContextKeyError = (entry: ContextEntry) => {
  const key = entry.key.trim()
  if (!entry.value.trim()) return false
  return !key || duplicateContextKeySet.value.has(key)
}

const getContextRowError = (entry: ContextEntry) => {
  const key = entry.key.trim()
  if (!entry.value.trim()) return ''
  if (!key) return t('message.verifyContextKeyRequired')
  if (duplicateContextKeySet.value.has(key)) return t('message.verifyContextKeyDuplicate')
  return ''
}

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
  if (contextEntries.value.some(entry => entry.value.trim() && !entry.key.trim())) {
    ElMessage.error(t('message.verifyContextKeyRequired'))
    return
  }
  if (duplicateContextKeySet.value.size > 0) {
    ElMessage.error(t('message.verifyContextKeyDuplicate'))
    return
  }
  setRecentAppId(form.value.appid)
  try {
    const context = Object.fromEntries(
      contextEntries.value
        .map(entry => [entry.key.trim(), entry.value.trim()] as const)
        .filter(([key]) => key.length > 0)
    )
    const res = await verify({
      appid: form.value.appid,
      key: form.value.key,
      context
    })
    result.value = res.data
  } catch (e) {
    ElMessage.error(t('message.verifyFailed'))
  }
}

const addContextEntry = () => {
  contextEntries.value.push({
    id: nextContextEntryId++,
    key: '',
    value: ''
  })
}

const removeContextEntry = (id: number) => {
  if (contextEntries.value.length === 1) {
    contextEntries.value[0] = { id, key: '', value: '' }
    return
  }
  contextEntries.value = contextEntries.value.filter(entry => entry.id !== id)
}
</script>

<template>
  <div class="verify-page">
    <div class="verify-layout">
      <div class="verify-card verify-panel">
        <el-form :model="form" label-width="64px" class="verify-form">
          <el-form-item :label="t('verify.application')">
            <div class="verify-app-row">
              <el-select v-model="form.appid" :placeholder="t('verify.selectApp')" @change="setRecentAppId" class="verify-app-select">
                <el-option v-for="app in apps" :key="app.id" :label="`${app.name} (${app.id})`" :value="app.id" />
              </el-select>
              <el-button type="primary" @click="handleVerify">{{ t('verify.button') }}</el-button>
            </div>
          </el-form-item>
          <el-form-item :label="t('verify.key')">
            <el-input v-model="form.key" :placeholder="t('verify.keyPlaceholder')" class="verify-input" />
          </el-form-item>
          <el-form-item :label="t('verify.context')" class="context-form-item">
            <div class="context-editor">
              <div v-for="entry in contextEntries" :key="entry.id" class="context-row" :class="{ 'context-row-error': hasContextKeyError(entry) }">
                <el-input v-model="entry.key" :placeholder="t('verify.contextKey')" class="context-key-input" />
                <el-input v-model="entry.value" :placeholder="t('verify.contextValue')" class="context-value-input" />
                <el-button @click="removeContextEntry(entry.id)">{{ t('common.delete') }}</el-button>
                <div v-if="getContextRowError(entry)" class="context-row-error-text">{{ getContextRowError(entry) }}</div>
              </div>
              <el-button class="context-add-btn" @click="addContextEntry">{{ t('verify.addContext') }}</el-button>
            </div>
          </el-form-item>
        </el-form>
      </div>

      <div class="result-area verify-panel">
        <pre v-if="result">{{ JSON.stringify(result, null, 2) }}</pre>
        <div v-else class="result-empty">{{ t('verify.resultEmpty') }}</div>
      </div>
    </div>

    <!-- Access Token 明文展示属于设计决策：该 token 用于外部只读场景
         （SDK/curl 调用引擎接口时需要直接提供），不用于 admin 会话鉴权，
         此处仅供授权用户查阅以便接入调试，风险等同于 API Key 展示页。 -->
  </div>
</template>

<style scoped>
.verify-page {
  max-width: 1180px;
  margin: 0 auto;
  padding: 8px 20px 24px;
}
.verify-layout {
  max-width: 1040px;
  margin: 0 auto;
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 24px;
  align-items: stretch;
}
.verify-panel {
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 12px;
  min-height: 420px;
  padding: 20px;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.06);
}
.verify-card {
  min-width: 0;
}
.verify-form {
  margin-top: 0;
}
.verify-form :deep(.el-form-item__content) {
  width: 100%;
}
.verify-app-row {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
}
.verify-app-select {
  flex: 1;
  min-width: 0;
}
.verify-input {
  width: 100%;
}
.context-form-item :deep(.el-form-item__content) {
  align-items: flex-start;
}
.context-editor {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.context-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.context-row-error :deep(.el-input__wrapper) {
  box-shadow: 0 0 0 1px var(--el-color-danger) inset;
}
.context-row-error-text {
  width: 100%;
  color: var(--el-color-danger);
  font-size: 12px;
  line-height: 1.4;
}
.context-key-input {
  width: 120px;
}
.context-value-input {
  flex: 1;
}
.context-add-btn {
  align-self: flex-start;
}
.result-area {
  background: #f5f7fa;
}
.result-area pre {
  min-height: 380px;
  margin: 0;
  padding: 14px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.72);
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
.result-empty {
  min-height: 380px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.72);
  color: #909399;
  text-align: center;
  padding: 14px;
}
@media (max-width: 640px) {
  .verify-page {
    padding-left: 12px;
    padding-right: 12px;
  }
  .verify-layout {
    grid-template-columns: 1fr;
    gap: 16px;
  }
  .verify-input,
  .verify-app-row,
  .context-editor {
    width: 100%;
  }
  .verify-app-row {
    flex-direction: column;
    align-items: stretch;
  }
  .verify-app-select {
    width: 100%;
  }
  .context-row {
    flex-direction: column;
    align-items: stretch;
  }
  .context-key-input,
  .context-value-input {
    width: 100%;
  }
}
</style>
