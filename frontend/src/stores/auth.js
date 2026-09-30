import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as api from '../api'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const user = ref(JSON.parse(localStorage.getItem('user') || 'null'))

  const isLogin = computed(() => !!token.value)
  const role = computed(() => user.value?.role || 'user')
  const isAdmin = computed(() => role.value === 'admin')
  const canVerify = computed(() => role.value === 'admin' || role.value === 'station')

  function setSession(t, u) {
    token.value = t
    user.value = u
    localStorage.setItem('token', t)
    localStorage.setItem('user', JSON.stringify(u))
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
    localStorage.removeItem('user')
  }

  async function doLogin(form) {
    const data = await api.login(form)
    setSession(data.token, data.user)
    return data
  }

  async function doRegister(form) {
    const data = await api.register(form)
    setSession(data.token, data.user)
    return data
  }

  return { token, user, isLogin, role, isAdmin, canVerify, setSession, logout, doLogin, doRegister }
})
