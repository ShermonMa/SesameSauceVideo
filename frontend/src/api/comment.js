/**
 * comment.js
 * 功能：评论相关接口封装；支持发布评论、获取一级/二级评论列表、点赞/取消点赞
 * 时间戳：2026-04-27
 */
import request from './request.js'

/**
 * 发布评论（支持一级评论和楼中楼回复）
 * @param {Object} data - { video_id, content, parent_id? }
 * @returns {Promise<{comment_id: number, created_at: string}>}
 */
export function publishComment(data) {
  return request({
    url: '/api/v1/comment/publish',
    method: 'post',
    data
  })
}

/**
 * 获取视频一级评论列表（默认携带前3条二级回复）
 * @param {Object} params - { video_id, page, size }
 * @returns {Promise<{comments: Array}>}
 */
export function getCommentList(params) {
  return request({
    url: '/api/v1/comment/list',
    method: 'get',
    params
  })
}

/**
 * 获取楼中楼完整回复列表（按 root_id 分页加载）
 * @param {Object} params - { root_id, page, size }
 * @returns {Promise<{replies: Array}>}
 */
export function getReplyList(params) {
  return request({
    url: '/api/v1/comment/replies',
    method: 'get',
    params
  })
}

/**
 * 点赞或取消点赞评论
 * @param {Object} data - { comment_id, action } action: 1点赞 2取消
 * @returns {Promise<Object>}
 */
export function likeComment(data) {
  return request({
    url: '/api/v1/comment/like',
    method: 'post',
    data
  })
}
