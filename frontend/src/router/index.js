/*
 * router/index.js
 * 功能：路由配置；改为嵌套路由，主页/上传/详情/我的视频共用 MainLayout，登录/注册独立
 * 时间戳：2026-05-03
 * 变更：beforeEach 守卫改用 isTokenValid()，避免持过期 token 进入受保护路由后才被后端 1001 弹回
 */
import { createRouter, createWebHistory } from 'vue-router'
import RegisterView from '../views/RegisterView.vue'
import LoginView from '../views/LoginView.vue'
import UploadView from '../views/UploadView.vue'
import VideoDetailView from '../views/VideoDetailView.vue'
import HomeView from '../views/HomeView.vue'
import MyVideosView from '../views/MyVideosView.vue'
import MessageCenterView from '../views/MessageCenterView.vue'
import ProfileView from '../views/ProfileView.vue'
import MainLayout from '../layouts/MainLayout.vue'
import { isTokenValid } from '../utils/auth.js'
import { triggerBetaExpired } from '../api/beta.js'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: LoginView
  },
  {
    path: '/register',
    name: 'Register',
    component: RegisterView
  },
  {
    path: '/',
    component: MainLayout,
    children: [
      {
        path: '',
        name: 'Home',
        component: HomeView
      },
      {
        path: 'upload',
        name: 'Upload',
        component: UploadView,
        meta: { requiresAuth: true }
      },
      {
        path: 'video/:id',
        name: 'VideoDetail',
        component: VideoDetailView
      },
      {
        path: 'my-videos',
        name: 'MyVideos',
        component: MyVideosView,
        meta: { requiresAuth: true }
      },
      {
        path: 'messages',
        name: 'MessageCenter',
        component: MessageCenterView,
        meta: { requiresAuth: true }
      },
      {
        path: 'profile/:id?',
        name: 'Profile',
        component: ProfileView,
        meta: { requiresAuth: true }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

/**
 * 判断本地 beta_token 是否仍有效（存在且未过期）
 * 与 auth.js 中 isTokenValid 逻辑一致
 */
function isBetaTokenValid() {
  const token = localStorage.getItem('beta_token')
  if (!token) return false
  try {
    const parts = token.split('.')
    if (parts.length !== 3) return false
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4)
    const payload = JSON.parse(atob(padded))
    return payload.exp * 1000 > Date.now() + 5000
  } catch {
    return false
  }
}

// 路由守卫：
// 1. 若 Beta Token 本地已过期，触发全局清理（由 BetaGuard 重新展示遮罩）
// 2. 受保护路由必须持有有效 JWT，否则跳登录页
router.beforeEach((to, from, next) => {
  if (!isBetaTokenValid()) {
    triggerBetaExpired()
  }

  if (to.meta.requiresAuth && !isTokenValid()) {
    next('/login')
  } else {
    next()
  }
})

export default router
