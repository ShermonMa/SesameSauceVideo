import request from './request.js'

export function register(data) {
  return request({
    url: '/api/v1/user/register',
    method: 'post',
    data
  })
}

export function login(data) {
  return request({
    url: '/api/v1/user/login',
    method: 'post',
    data
  })
}

export function getUserProfile(id) {
  return request({
    url: `/api/v1/users/${id}`,
    method: 'get'
  })
}