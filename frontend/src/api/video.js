import request from './request.js'
import axios from 'axios'

export function createVideo(data) {
  return request({
    url: '/api/v1/videos',
    method: 'post',
    data
  })
}

// listVideos 获取公开视频列表，支持分页、分类、关键词与排序
export function listVideos({ page = 1, pageSize = 20, categoryId, keyword, orderBy } = {}) {
  return request({
    url: '/api/v1/videos',
    method: 'get',
    params: { page, page_size: pageSize, category_id: categoryId, keyword, order_by: orderBy }
  })
}

export function confirmUpload(videoId, data = {}) {
  return request({
    url: `/api/v1/videos/${videoId}/confirm`,
    method: 'post',
    data
  })
}

export function cancelUpload(videoId) {
  return request({
    url: `/api/v1/videos/${videoId}/cancel`,
    method: 'post'
  })
}

export function getVideoStatus(videoId) {
  return request({
    url: `/api/v1/videos/${videoId}/status`,
    method: 'get'
  })
}

export function uploadCoverToMinIO(uploadUrl, file) {
  return axios.put(uploadUrl, file, {
    headers: {
      'Content-Type': file.type || 'application/octet-stream'
    }
  })
}

export function getVideo(videoId) {
  return request({
    url: `/api/v1/videos/${videoId}`,
    method: 'get'
  })
}

export function getPlayUrl(videoId) {
  return request({
    url: `/api/v1/videos/${videoId}/play`,
    method: 'get'
  })
}

export function reportProgress(videoId, progress) {
  return request({
    url: `/api/v1/videos/${videoId}/progress`,
    method: 'post',
    data: { progress }
  })
}

// likeVideo 点赞或取消点赞视频；action: 1 点赞，2 取消点赞
export function likeVideo(videoId, action) {
  return request({
    url: `/api/v1/videos/${videoId}/like`,
    method: 'post',
    data: { action }
  })
}

// listMyVideos 拉取登录用户自己的全部视频（含处理中/失败/已发布），用于"我的视频"页
export function listMyVideos({ page = 1, pageSize = 20 } = {}) {
  return request({
    url: '/api/v1/users/me/videos',
    method: 'get',
    params: { page, page_size: pageSize }
  })
}

// getUserVideos 拉取指定用户的已发布视频，用于个人主页"投稿"Tab
// 2026-06-22 新增
export function getUserVideos(userId, { page = 1, pageSize = 12 } = {}) {
  return request({
    url: `/api/v1/users/${userId}/videos`,
    method: 'get',
    params: { page, page_size: pageSize }
  })
}

export function listCategories() {
  return request({
    url: '/api/v1/categories',
    method: 'get'
  })
}

export function uploadToMinIO(uploadUrl, file, onProgress) {
  return axios.put(uploadUrl, file, {
    headers: {
      'Content-Type': file.type || 'application/octet-stream'
    },
    onUploadProgress: (progressEvent) => {
      if (onProgress && progressEvent.total) {
        const percent = Math.round((progressEvent.loaded * 100) / progressEvent.total)
        onProgress(percent)
      }
    }
  })
}
