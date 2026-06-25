/*
 * format.js
 * 功能：通用格式化工具（视频时长、播放量、相对时间）
 * 时间戳：2026-04-26
 */

// formatDuration 把秒数转为 mm:ss 或 hh:mm:ss
export function formatDuration(seconds) {
  const s = Math.max(0, Math.floor(Number(seconds) || 0))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  const pad = (n) => String(n).padStart(2, '0')
  if (h > 0) return `${pad(h)}:${pad(m)}:${pad(sec)}`
  return `${pad(m)}:${pad(sec)}`
}

// formatPlayCount 大数转中文单位：<1万原值；≥1万 转「N.N万」；≥1亿 转「N.N亿」
export function formatPlayCount(n) {
  const num = Number(n) || 0
  if (num < 10000) return String(num)
  if (num < 1_0000_0000) return (num / 10000).toFixed(1) + '万'
  return (num / 1_0000_0000).toFixed(1) + '亿'
}

// formatRelativeTime 把 ISO 时间转中文相对时间：刚刚 / N分钟前 / N小时前 / N天前 / yyyy-MM-dd
export function formatRelativeTime(iso) {
  if (!iso) return ''
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return ''
  const diff = Date.now() - t
  if (diff < 60_000) return '刚刚'
  if (diff < 3600_000) return Math.floor(diff / 60_000) + ' 分钟前'
  if (diff < 86400_000) return Math.floor(diff / 3600_000) + ' 小时前'
  if (diff < 30 * 86400_000) return Math.floor(diff / 86400_000) + ' 天前'
  const d = new Date(t)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}
