<script setup lang="ts">
import { useI18n, type Locale } from '@/i18n'

const { locale, setLocale, t } = useI18n()

const handleLocaleChange = (nextLocale: string) => {
  setLocale(nextLocale as Locale)
}
</script>

<template>
  <el-container class="layout-container">
    <el-header class="header">
      <div class="logo">{{ t('app.title') }}</div>
      <el-menu class="nav-menu" mode="horizontal" router :default-active="$route.path" :ellipsis="false">
        <el-menu-item index="/">{{ t('app.experiments') }}</el-menu-item>
        <el-menu-item index="/verify">{{ t('app.verify') }}</el-menu-item>
      </el-menu>
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
.lang-label {
  color: #606266;
  font-size: 13px;
}
</style>
