<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n, type Locale } from '@/i18n'
import AuthDialog from '@/components/AuthDialog.vue'
import { clearSession, loadSession, openAuthDialog, useAuth, closeAuthDialog } from '@/auth'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteUser, updateUserPassword } from '@/api'
import { isStrongPassword } from '@/utils/password'

const { locale, setLocale, t } = useI18n()
const { session, isLoggedIn } = useAuth()
const accountDialogVisible = ref(false)
const accountForm = ref({ password: '' })
const updatingPassword = ref(false)
const deletingUser = ref(false)
const loggingOut = ref(false)

const handleLocaleChange = (nextLocale: string) => {
  setLocale(nextLocale as Locale)
}

const openAccountDialog = () => {
  accountForm.value.password = ''
  accountDialogVisible.value = true
}

const handleLogout = async () => {
  try {
    loggingOut.value = true
    await ElMessageBox.confirm(t('confirm.logout'), t('common.warning'), { type: 'warning' })
    clearSession()
    closeAuthDialog()
    accountDialogVisible.value = false
  } catch {
    // ignore cancel
  } finally {
    loggingOut.value = false
  }
}

const handleUpdatePassword = async () => {
  if (!session.value?.uid) {
    openAuthDialog()
    return
  }
  if (!accountForm.value.password) {
    ElMessage.error(t('message.passwordRequired'))
    return
  }
  if (!isStrongPassword(accountForm.value.password)) {
    ElMessage.error(t('message.passwordRule'))
    return
  }
  updatingPassword.value = true
  try {
    await updateUserPassword(session.value.uid, { password: accountForm.value.password })
    accountForm.value.password = ''
    ElMessage.success(t('message.passwordUpdated'))
  } catch {
    ElMessage.error(t('message.updateFailed'))
  } finally {
    updatingPassword.value = false
  }
}

const handleDeleteUser = async () => {
  if (!session.value?.uid) {
    openAuthDialog()
    return
  }
  try {
    deletingUser.value = true
    await ElMessageBox.confirm(t('confirm.deleteUser'), t('common.warning'), { type: 'warning' })
    await deleteUser(session.value.uid)
    clearSession()
    closeAuthDialog()
    accountDialogVisible.value = false
    ElMessage.success(t('message.userDeleted'))
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(t('message.deleteFailed'))
  } finally {
    deletingUser.value = false
  }
}

onMounted(() => {
  loadSession()
})
</script>

<template>
  <el-container class="layout-container">
    <el-header class="header">
      <div class="logo">{{ t('app.title') }}</div>
      <el-menu class="nav-menu" mode="horizontal" router :default-active="$route.path" :ellipsis="false">
        <el-menu-item index="/">{{ t('app.experiments') }}</el-menu-item>
        <el-menu-item index="/verify">{{ t('app.verify') }}</el-menu-item>
      </el-menu>
      <div class="session-actions">
        <template v-if="isLoggedIn">
          <span class="username">{{ session?.name }}</span>
          <el-button size="small" @click="openAccountDialog">{{ t('app.accountSettings') }}</el-button>
        </template>
        <template v-else>
          <el-button size="small" type="primary" @click="openAuthDialog">{{ t('app.login') }}</el-button>
        </template>
      </div>
      <div class="lang-switcher">
        <span class="lang-label">{{ t('app.language') }}</span>
        <el-select :model-value="locale" style="width: 120px" size="small" @change="handleLocaleChange">
          <el-option value="zh-CN" :label="t('app.langZh')" />
          <el-option value="en-US" :label="t('app.langEn')" />
        </el-select>
      </div>
    </el-header>
    <el-main>
      <router-view />
    </el-main>
    <el-dialog v-model="accountDialogVisible" :title="t('settings.title')" width="460px">
      <el-form :model="accountForm">
        <el-form-item :label="t('auth.password')">
          <el-input v-model="accountForm.password" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="account-footer">
          <el-button :loading="deletingUser" type="danger" @click="handleDeleteUser">{{ t('settings.deleteUser') }}</el-button>
          <div class="account-footer-right">
            <el-button :loading="updatingPassword" type="primary" @click="handleUpdatePassword">{{ t('settings.updatePassword') }}</el-button>
            <el-button :loading="loggingOut" @click="handleLogout">{{ t('settings.logout') }}</el-button>
          </div>
        </div>
      </template>
    </el-dialog>
    <AuthDialog />
  </el-container>
</template>

<style scoped>
.layout-container {
  height: 100vh;
}
.header {
  display: flex;
  align-items: center;
  border-bottom: 1px solid #dcdfe6;
  gap: 24px;
}
.logo {
  font-size: 1.2rem;
  font-weight: bold;
}
.nav-menu {
  flex: 1;
}
.lang-switcher {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.session-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.username {
  color: #606266;
  font-size: 13px;
}
.account-footer {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.account-footer-right {
  display: flex;
  gap: 8px;
}
.lang-label {
  color: #606266;
  font-size: 13px;
}
</style>
