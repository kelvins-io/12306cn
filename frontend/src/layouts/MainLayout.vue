<template>
  <el-container class="layout">
    <el-header class="header">
      <div class="brand" @click="$router.push('/')">
        <span class="logo">12306cn</span>
        <span class="sub">演示票务系统</span>
      </div>
      <el-menu mode="horizontal" :ellipsis="false" router :default-active="active" class="nav">
        <el-menu-item index="/">车票查询</el-menu-item>
        <el-menu-item index="/orders">我的订单</el-menu-item>
        <el-menu-item index="/passengers">乘车人</el-menu-item>
        <el-menu-item index="/waitlist">候补</el-menu-item>
        <el-menu-item v-if="auth.canVerify" index="/verify">取票核验</el-menu-item>
        <el-menu-item v-if="auth.isAdmin" index="/admin">运营配置</el-menu-item>
      </el-menu>
      <div class="user">
        <template v-if="auth.isLogin">
          <span>{{ auth.user?.nickname || auth.user?.username }}</span>
          <el-tag size="small" type="info">{{ auth.role }}</el-tag>
          <el-button link type="danger" @click="onLogout">退出</el-button>
        </template>
        <template v-else>
          <el-button type="primary" @click="$router.push('/login')">登录</el-button>
        </template>
      </div>
    </el-header>
    <el-main>
      <router-view />
    </el-main>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const active = computed(() => route.path)

function onLogout() {
  auth.logout()
  router.push('/login')
}
</script>

<style scoped>
.layout {
  min-height: 100vh;
}
.header {
  display: flex;
  align-items: center;
  gap: 24px;
  background: #fff;
  border-bottom: 3px solid var(--rail-red);
  height: 64px !important;
}
.brand {
  cursor: pointer;
  display: flex;
  flex-direction: column;
  line-height: 1.2;
  min-width: 120px;
}
.logo {
  color: var(--rail-red);
  font-size: 22px;
  font-weight: 700;
  letter-spacing: 1px;
}
.sub {
  font-size: 12px;
  color: #888;
}
.nav {
  flex: 1;
  border-bottom: none !important;
}
.user {
  display: flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
}
</style>
