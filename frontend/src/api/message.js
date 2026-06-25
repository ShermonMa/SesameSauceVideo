/**
 * message.js
 * 功能：消息中心相关接口封装；支持未读数聚合、各 Tab 消息列表、标记已读、系统通知
 * 时间戳：2026-04-27
 */
import request from './request.js'

/**
 * 获取六 Tab 未读消息气泡数聚合
 * @returns {Promise<{private: number, reply: number, mention: number, like: number, follow: number, system: number}>}
 */
export function getUnreadCount() {
  return request({
    url: '/api/v1/message/unread-count',
    method: 'get'
  })
}

/**
 * 获取指定 Tab 的消息列表（私信/回复/赞/关注，不含系统通知）
 * @param {Object} params - { type, page, size }
 *   type: 1私信 2关注 3点赞 5回复
 * @returns {Promise<{messages: Array}>}
 */
export function getMessageList(params) {
  return request({
    url: '/api/v1/message/list',
    method: 'get',
    params
  })
}

/**
 * 标记消息已读（支持批量或按类型全标）
 * @param {Object} data - { ids?: number[], type?: number }
 *   传 ids 则标记指定消息；不传 ids 仅传 type 则标记该 Tab 全部已读
 * @returns {Promise<Object>}
 */
export function markRead(data) {
  return request({
    url: '/api/v1/message/read',
    method: 'post',
    data
  })
}

/**
 * 获取系统通知列表
 * @param {Object} params - { page, size }
 * @returns {Promise<{notifications: Array, has_new: boolean}>}
 */
export function getSystemList(params) {
  return request({
    url: '/api/v1/message/system-list',
    method: 'get',
    params
  })
}

/**
 * 标记系统通知已读（仅更新用户 last_sys_msg_check 时间戳）
 * @returns {Promise<Object>}
 */
export function markSystemRead() {
  return request({
    url: '/api/v1/message/system-read',
    method: 'post'
  })
}
