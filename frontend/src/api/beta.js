/*
 * beta.js
 * 功能：内测准入相关 API 封装；新增验证通过回调供页面刷新受保护数据
 * 时间戳：2026-06-20
 */

import request from './request.js'

// 全局 Beta 过期回调，供 request.js / router.js 在收到 1005 时触发
let betaExpiredCallback = null
// 全局 Beta 校验通过回调，供需要在验证通过后刷新数据的页面注册
let betaVerifiedCallback = null

/**
 * 注册 Beta Token 过期时的回调（由 BetaGuard 组件注册）
 * @param {Function} callback
 */
export function onBetaExpired(callback) {
  betaExpiredCallback = callback
}

/**
 * 触发 Beta Token 过期处理：清除本地 token 并通知注册方重新展示遮罩
 */
export function triggerBetaExpired() {
  localStorage.removeItem('beta_token')
  if (typeof betaExpiredCallback === 'function') {
    betaExpiredCallback()
  }
}

/**
 * 注册 Beta 校验通过后的回调（由依赖 Beta 权限的页面注册）
 * @param {Function} callback
 */
export function onBetaVerified(callback) {
  betaVerifiedCallback = callback
}

/**
 * 触发 Beta 校验通过处理：通知注册方可以重新拉取受保护的数据
 */
export function triggerBetaVerified() {
  if (typeof betaVerifiedCallback === 'function') {
    betaVerifiedCallback()
  }
}

/**
 * 校验内测密钥并获取 Beta Token
 * @param {string} key 用户输入的密钥
 * @returns {Promise<{beta_token: string, expires_at: string}>}
 */
export function verifyBetaKey(key) {
  return request.post('/api/v1/beta/verify', { key })
}

/**
 * 检查当前 Beta Token 是否仍有效
 * @returns {Promise<{valid: boolean, expires_at?: string}>}
 */
export function checkBetaToken() {
  return request.get('/api/v1/beta/check')
}
