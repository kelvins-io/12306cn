<template>
  <div class="auth-page">
    <el-card class="card">
      <h2>注册</h2>
      <el-form :model="form" label-width="70px" @submit.prevent="onSubmit">
        <el-form-item label="用户名">
          <el-input v-model="form.username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password />
        </el-form-item>
        <el-form-item label="昵称">
          <el-input v-model="form.nickname" />
        </el-form-item>
        <el-form-item label="手机">
          <el-input v-model="form.phone" />
        </el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading" style="width:100%">注册并登录</el-button>
      </el-form>
      <div class="foot">
        已有账号？
        <router-link to="/login">登录</router-link>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const loading = ref(false)
const form = reactive({ username: '', password: '', nickname: '', phone: '' })

async function onSubmit() {
  loading.value = true
  try {
    await auth.doRegister(form)
    ElMessage.success('注册成功')
    router.replace('/')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
}
.card {
  width: 420px;
}
.foot {
  margin-top: 16px;
  text-align: center;
}
.foot a {
  color: var(--rail-red);
}
</style>
