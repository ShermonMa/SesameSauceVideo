import request from './request.js'

// followUser 关注用户；action: 1=关注, 2=取消关注
// 2026-06-22 新增，支撑个人主页关注按钮
export function followUser(userId, action) {
  return request({
    url: '/api/v1/follow',
    method: 'post',
    data: { user_id: userId, action }
  })
}
