import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  { path: '/login', name: 'login', component: () => import('../views/LoginView.vue'), meta: { guest: true } },
  { path: '/register', name: 'register', component: () => import('../views/RegisterView.vue'), meta: { guest: true } },
  {
    path: '/',
    component: () => import('../layouts/MainLayout.vue'),
    children: [
      { path: '', name: 'home', component: () => import('../views/HomeView.vue') },
      { path: 'orders', name: 'orders', component: () => import('../views/OrdersView.vue'), meta: { auth: true } },
      { path: 'passengers', name: 'passengers', component: () => import('../views/PassengersView.vue'), meta: { auth: true } },
      { path: 'waitlist', name: 'waitlist', component: () => import('../views/WaitlistView.vue'), meta: { auth: true } },
      { path: 'admin', name: 'admin', component: () => import('../views/AdminView.vue'), meta: { auth: true, admin: true } },
      { path: 'verify', name: 'verify', component: () => import('../views/VerifyView.vue'), meta: { auth: true, verify: true } },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.auth && !auth.isLogin) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.admin && !auth.isAdmin) {
    return { name: 'home' }
  }
  if (to.meta.verify && !auth.canVerify) {
    return { name: 'home' }
  }
  if (to.meta.guest && auth.isLogin) {
    return { name: 'home' }
  }
})

export default router
