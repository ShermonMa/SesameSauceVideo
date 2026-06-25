/*
 * request.js
 * 功能：axios 实例与拦截器；请求注入 JWT，响应统一处理 code=1001 登录失效（清token+跳登录页+防抖）
 * 时间戳：2026-05-03
 */

import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'
import { triggerBetaExpired } from './beta.js'

// 防抖标志：避免轮询/并发请求同时触发 1001 时反复弹窗与跳转
let isHandling401 = false

const request = axios.create({
  // 开发环境直连后端；生产环境通过 Nginx 代理，url 已包含 /api/v1 完整前缀
  baseURL: import.meta.env.DEV ? 'http://localhost:8080' : '',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 请求拦截器：注入 JWT Token 与 Beta Token
request.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    const betaToken = localStorage.getItem('beta_token')
    if (betaToken) {
      config.headers['X-Beta-Token'] = betaToken
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器：登录失效/内测过期统一兜底，其他业务错误维持原有 ElMessage 提示
request.interceptors.response.use(
  (response) => {
    const res = response.data
    if (res.code === 1001) {
      handleAuthExpired()
      return Promise.reject(new Error(res.message || '登录已过期'))
    }
    if (res.code === 1005) {
      handleBetaExpired()
      return Promise.reject(new Error(res.message || '内测权限已过期'))
    }
    if (res.code !== 0) {
      ElMessage.error(res.message || '请求失败')
      return Promise.reject(new Error(res.message || '请求失败'))
    }
    return res
  },
  (error) => {
    ElMessage.error(error.message || '网络错误')
    return Promise.reject(error)
  }
)

/**
 * 登录态失效处理：清除本地 token、提示用户、跳转到登录页
 * 通过模块级 flag 在 1 秒窗口内只执行一次提示与跳转，避免轮询并发触发风暴
 * token 的清理是幂等的，每次都执行；提示与跳转受 flag 保护
 */
function handleAuthExpired() {
  localStorage.removeItem('token')
  if (isHandling401) return
  isHandling401 = true
  ElMessage.error('登录已过期，请重新登录')
  if (router.currentRoute.value.path !== '/login') {
    router.push('/login')
  }
  setTimeout(() => {
    isHandling401 = false
  }, 1000)
}

/**
 * 内测权限过期处理：清除本地 Beta Token 并触发全局遮罩重新展示
 */
function handleBetaExpired() {
  triggerBetaExpired()
  ElMessage.warning('内测权限已过期，请重新输入密钥')
}

export default request
