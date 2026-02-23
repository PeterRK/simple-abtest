<template>
  <el-container class="layout-container">
    <el-header class="header">
      <div class="logo">AB Testing Platform</div>
      <el-menu mode="horizontal" router :default-active="$route.path" :ellipsis="false">
        <el-menu-item index="/">Experiments</el-menu-item>
        <el-menu-item index="/verify">Online Verify</el-menu-item>
      </el-menu>
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
}
.logo {
  font-size: 1.2rem;
  font-weight: bold;
  margin-right: 2rem;
}
</style>
