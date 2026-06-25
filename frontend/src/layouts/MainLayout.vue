<!--
  MainLayout.vue
  功能：主页公共布局；顶部导航（Logo / 搜索 / 投稿 / 登录态菜单）+ 嵌套 router-view
  时间戳：2026-05-03
  变更：登录态判断改为 isTokenValid()——同时校验 token 存在与未过期，
        避免持有过期 token 仍尝试调用受保护接口（如轮询未读数）触发后端 1001
  2026-06-20 顶部栏整体放大，新增个人主页入口
  2026-06-21 通过 provide 暴露 refreshUnreadCount，供消息中心在标记已读后主动刷新顶部未读气泡
  2026-06-22 Logo 图标从蜂蜜罐 emoji 替换为应用图标 app-icon.png
-->
<script setup>
import { ref, computed, onMounted, onUnmounted, provide } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, UploadFilled, Bell } from '@element-plus/icons-vue'
import { getUnreadCount } from '../api/message.js'
import { isTokenValid } from '../utils/auth.js'

const router = useRouter()
const route = useRoute()

// 登录态：token 存在且未过期才视为登录中；通过 router.afterEach 钩子刷新
const isLogin = ref(isTokenValid())
const keyword = ref(route.query.keyword || '')
const unreadCount = ref({ private: 0, reply: 0, mention: 0, like: 0, follow: 0, system: 0 })

/**
 * 计算总未读数（用于顶部铃铛气泡）
 */
const totalUnread = computed(() => {
  return Object.values(unreadCount.value).reduce((sum, v) => sum + (v || 0), 0)
})

/**
 * 拉取未读消息气泡数聚合
 */
const fetchUnreadCount = async () => {
  if (!isLogin.value) return
  try {
    const res = await getUnreadCount()
    unreadCount.value = res.data || unreadCount.value
  } catch (e) {
    // 静默失败，不影响主流程
  }
}

// 向子页面暴露刷新未读数的方法，消息中心标记已读或切换 Tab 后可主动更新顶部气泡
provide('refreshUnreadCount', fetchUnreadCount)

let unregisterAfterEach = null
let unreadTimer = null

onMounted(() => {
  fetchUnreadCount()
  // 每 30 秒轮询一次未读数（私信/回复/赞等需要实时感）
  unreadTimer = setInterval(fetchUnreadCount, 30000)

  unregisterAfterEach = router.afterEach(() => {
    isLogin.value = isTokenValid()
    keyword.value = route.query.keyword || ''
    fetchUnreadCount()
  })
})
onUnmounted(() => {
  if (unregisterAfterEach) unregisterAfterEach()
  if (unreadTimer) clearInterval(unreadTimer)
})

// 触发搜索：跳到首页携带 keyword query
function handleSearch() {
  const kw = keyword.value.trim()
  router.push({ path: '/', query: kw ? { keyword: kw, page: 1 } : {} })
}

// 投稿入口；未登录时弹确认框引导到登录页
function handleUpload() {
  if (!isLogin.value) {
    ElMessageBox.confirm('请先登录后再投稿', '提示', {
      confirmButtonText: '去登录',
      cancelButtonText: '取消',
      type: 'info'
    }).then(() => {
      router.push('/login')
    }).catch(() => {})
    return
  }
  router.push('/upload')
}

function goLogin() { router.push('/login') }
function goRegister() { router.push('/register') }
function goHome() { router.push('/') }
function goMyVideos() { router.push('/my-videos') }
function goMessageCenter() { router.push('/messages') }
function goProfile() { router.push('/profile') }

// 退出登录：清除 token 并刷新本组件登录态
function logout() {
  localStorage.removeItem('token')
  isLogin.value = false
  ElMessage.success('已退出登录')
  router.push('/')
}
</script>

<template>
  <el-container class="layout">
    <el-header class="header">
      <div class="header-inner">
        <div class="logo" @click="goHome">
          <img class="logo-icon" src="/app-icon.png" alt="logo" />
          <span class="logo-text">芝麻酱</span>
        </div>

        <div class="search-bar">
          <el-input
            v-model="keyword"
            size="large"
            placeholder="搜索你感兴趣的视频"
            :prefix-icon="Search"
            clearable
            @keyup.enter="handleSearch"
          />
          <el-button size="large" type="primary" class="search-btn" @click="handleSearch">搜索</el-button>
        </div>

        <div class="actions">
          <el-button size="large" :icon="UploadFilled" type="primary" plain @click="handleUpload">投稿</el-button>

          <template v-if="!isLogin">
            <el-button size="large" text @click="goLogin">登录</el-button>
            <el-button size="large" type="primary" @click="goRegister">注册</el-button>
          </template>

          <el-badge v-else :value="totalUnread" :hidden="totalUnread === 0" class="msg-badge">
            <el-button size="large" text circle :icon="Bell" @click="goMessageCenter" />
          </el-badge>

          <el-dropdown v-if="isLogin" trigger="click">
            <el-avatar :size="42" class="user-avatar">我</el-avatar>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="goProfile">个人主页</el-dropdown-item>
                <el-dropdown-item @click="goMyVideos">我的视频</el-dropdown-item>
                <el-dropdown-item @click="goMessageCenter">消息中心</el-dropdown-item>
                <el-dropdown-item divided @click="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
    </el-header>

    <el-main class="main">
      <router-view />
    </el-main>
  </el-container>
</template>

<style scoped>
.layout {
  min-height: 100vh;
  background: #f4f5f7;
}
.header {
  background: #fff;
  border-bottom: 1px solid #e7e7e7;
  padding: 0;
  height: 72px;
  position: sticky;
  top: 0;
  z-index: 100;
}
.header-inner {
  max-width: 1600px;
  margin: 0 auto;
  height: 72px;
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 0 16px;
}
.logo {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  flex-shrink: 0;
}
.logo-icon {
  width: 30px;
  height: 28px;
}
.logo-text {
  font-size: 20px;
  font-weight: 600;
  color: var(--bili-pink, #fb7299);
}
.search-bar {
  flex: 1;
  max-width: 640px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.search-bar :deep(.el-input__wrapper) {
  border-radius: 24px 0 0 24px;
}
.search-btn {
  border-radius: 0 24px 24px 0 !important;
  background: var(--bili-pink, #fb7299) !important;
  border-color: var(--bili-pink, #fb7299) !important;
  margin-left: -8px;
}
.actions {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-left: auto;
}
.user-avatar {
  cursor: pointer;
  background: var(--bili-pink, #fb7299);
  color: #fff;
}
.main {
  padding: 20px 16px;
  max-width: 1600px;
  width: 100%;
  margin: 0 auto;
  box-sizing: border-box;
}
.msg-badge :deep(.el-badge__content) {
  background: #ff4d4f;
  border: none;
}
</style>
