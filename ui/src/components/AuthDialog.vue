<script setup lang="ts">
import { ref } from 'vue'
import { loginUser, registerUser } from '@/api'
import { saveSession, useAuth } from '@/auth'
import { ElMessage } from 'element-plus'
import { useI18n, type Locale } from '@/i18n'
import { isStrongPassword } from '@/utils/password'
import { useRouter } from 'vue-router'

const { authDialogVisible } = useAuth()
const router = useRouter()
const mode = ref<'login' | 'register'>('login')
const loading = ref(false)
const form = ref({ name: '', password: '', confirmPassword: '' })
const { t, locale, setLocale } = useI18n()

const handleLocaleChange = (nextLocale: string) => {
  setLocale(nextLocale as Locale)
}

const resetForm = () => {
  form.value = { name: '', password: '', confirmPassword: '' }
}

const submit = async () => {
  if (!form.value.name || !form.value.password) {
    ElMessage.error(t('message.authRequired'))
    return
  }
  if (mode.value === 'register' && !form.value.confirmPassword) {
    ElMessage.error(t('message.confirmPasswordRequired'))
    return
  }
  if (mode.value === 'register' && form.value.password !== form.value.confirmPassword) {
    ElMessage.error(t('message.passwordMismatch'))
    return
  }
  if (mode.value === 'register' && !isStrongPassword(form.value.password)) {
    ElMessage.error(t('message.passwordRule'))
    return
  }
  loading.value = true
  try {
    const req = { name: form.value.name.trim(), password: form.value.password }
    const res = mode.value === 'login' ? await loginUser(req) : await registerUser(req)
    saveSession({ uid: res.data.uid, token: res.data.token, name: req.name })
    ElMessage.success(mode.value === 'login' ? t('message.loginSuccess') : t('message.registerSuccess'))
    await router.replace({ name: 'ExperimentList' })
    resetForm()
  } catch (e: any) {
    const status = e?.response?.status
    if (mode.value === 'register' && status === 409) {
      ElMessage.error(t('message.userExists'))
      return
    }
    if (mode.value === 'login' && status === 401) {
      ElMessage.error(t('message.loginFailed'))
      return
    }
    ElMessage.error(t('message.operationFailed'))
  } finally {
    loading.value = false
  }
}

const switchMode = () => {
  mode.value = mode.value === 'login' ? 'register' : 'login'
  resetForm()
}
</script>

<template>
  <el-dialog
    :model-value="authDialogVisible"
    width="420px"
    :close-on-click-modal="false"
    :show-close="false"
    :close-on-press-escape="false"
  >
    <template #header>
      <div class="auth-header">
        <span>{{ mode === 'login' ? t('auth.loginTitle') : t('auth.registerTitle') }}</span>
        <div class="auth-lang">
          <span class="auth-lang-label">{{ t('app.language') }}</span>
          <el-select :model-value="locale" size="small" style="width: 120px" @change="handleLocaleChange">
            <el-option value="zh-CN" :label="t('app.langZh')" />
            <el-option value="en-US" :label="t('app.langEn')" />
          </el-select>
        </div>
      </div>
    </template>
    <div class="auth-tip">{{ t('auth.needLoginTip') }}</div>
    <el-form :model="form" @submit.prevent>
      <el-form-item :label="t('common.name')">
        <el-input v-model="form.name" autocomplete="username" />
      </el-form-item>
      <el-form-item :label="t('auth.password')">
        <el-input v-model="form.password" type="password" show-password autocomplete="current-password" @keyup.enter="submit" />
      </el-form-item>
      <el-form-item v-if="mode === 'register'" :label="t('auth.confirmPassword')">
        <el-input v-model="form.confirmPassword" type="password" show-password autocomplete="new-password" @keyup.enter="submit" />
      </el-form-item>
    </el-form>
    <template #footer>
      <div class="auth-footer">
        <el-button link type="primary" @click="switchMode">
          {{ mode === 'login' ? t('auth.goRegister') : t('auth.goLogin') }}
        </el-button>
        <el-button type="primary" :loading="loading" @click="submit">
          {{ mode === 'login' ? t('auth.login') : t('auth.register') }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.auth-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.auth-lang {
  display: flex;
  align-items: center;
  gap: 8px;
}
.auth-lang-label {
  color: #606266;
  font-size: 13px;
}
.auth-tip {
  margin-bottom: 12px;
  color: #606266;
}
.auth-footer {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
