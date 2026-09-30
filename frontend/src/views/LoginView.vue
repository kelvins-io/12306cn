<template>
  <div class="auth-page">
    <el-card class="card">
      <h2>登录</h2>
      <p class="hint">演示账号：demo / 123456</p>
      <el-form :model="form" @submit.prevent="onSubmit">
        <el-form-item label="用户名">
          <el-input v-model="form.username" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password autocomplete="current-password" />
        </el-form-item>
        <el-form-item label="验证码">
          <div class="captcha-row">
            <el-input v-model="form.captcha_code" placeholder="计算结果" style="flex:1" />
            <el-button @click="loadCaptcha" :title="captcha.question">{{ captcha.question || '获取' }}</el-button>
          </div>
        </el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading" style="width:100%">登录</el-button>
      </el-form>
      <div class="foot">
        没有账号？
        <router-link to="/register">注册</router-link>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../stores/auth'
import * as api from '../api'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const loading = ref(false)
const captcha = reactive({ id: '', question: '' })
const form = reactive({ username: 'demo', password: '123456', captcha_code: '' })

async function loadCaptcha() {
  const data = await api.getCaptcha()
  captcha.id = data.id
  captcha.question = data.question
  form.captcha_code = ''
}

async function onSubmit() {
  loading.value = true
  try {
    await auth.doLogin({
      username: form.username,
      password: form.password,
      captcha_id: captcha.id,
      captcha_code: form.captcha_code,
    })
    ElMessage.success('登录成功')
    router.replace(route.query.redirect || '/')
  } catch {
    await loadCaptcha()
  } finally {
    loading.value = false
  }
}

onMounted(loadCaptcha)
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(180deg, #fff5f5 0%, #f5f6f8 60%);
}
.card {
  width: 380px;
}
.hint {
  color: #888;
  font-size: 13px;
  margin-top: -8px;
}
.foot {
  margin-top: 16px;
  text-align: center;
  color: #666;
}
.foot a {
  color: var(--rail-red);
}
.captcha-row {
  display: flex;
  gap: 8px;
  width: 100%;
}
</style>
