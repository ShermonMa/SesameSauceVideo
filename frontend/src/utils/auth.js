/*
 * auth.js
 * 功能：本地登录态工具——解析 JWT 的 exp 字段判断 token 是否仍然有效
 * 时间戳：2026-05-03
 */

/**
 * 解析 JWT payload；不校验签名，仅供前端读取 exp 等公开字段
 * 解析失败（格式非法/非 JWT）返回 null
 */
function decodeJwtPayload(token) {
  if (!token || typeof token !== 'string') return null
  const parts = token.split('.')
  if (parts.length !== 3) return null
  try {
    // base64url -> base64 + 补齐 padding
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4)
    return JSON.parse(atob(padded))
  } catch {
    return null
  }
}

/**
 * 判断本地 token 是否有效（存在且未过期）
 * 不传参时从 localStorage 读取；允许 5 秒时钟漂移容忍
 */
export function isTokenValid(token) {
  const t = token ?? localStorage.getItem('token')
  if (!t) return false
  const payload = decodeJwtPayload(t)
  if (!payload || typeof payload.exp !== 'number') return false
  // exp 是秒级时间戳；当前时间 + 5s 仍小于 exp 才算有效
  return payload.exp * 1000 > Date.now() + 5000
}
