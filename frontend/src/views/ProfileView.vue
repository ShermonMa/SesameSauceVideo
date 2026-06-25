<!--
  ProfileView.vue
  功能：个人主页；展示用户背景图、头像、统计信息、关注按钮与投稿视频网格
  时间戳：2026-06-22
-->
<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Plus, UserFilled, VideoCameraFilled } from '@element-plus/icons-vue'
import { isTokenValid } from '../utils/auth.js'
import { getUserProfile } from '../api/user.js'
import { getUserVideos } from '../api/video.js'
import { followUser } from '../api/follow.js'
import VideoCard from '../components/VideoCard.vue'

const route = useRoute()
const router = useRouter()

// ── 状态 ──
const user = ref({
  id: 0,
  name: '未知用户',
  avatar: '',
  background_image: '',
  signature: '',
  follow_count: 0,
  follower_count: 0,
  total_favorited: 0,
  work_count: 0,
  favorite_count: 0,
  created_at: 0,
  is_following: false
})
const loading = ref(true)
const videoLoading = ref(false)
const videos = ref([])
const videoPage = ref(1)
const videoTotal = ref(0)
const videoPageSize = 12
const activeTab = ref('videos')
const followLoading = ref(false)

// ── 计算属性 ──

/** 当前登录用户 ID */
const currentUserId = computed(() => {
  const token = localStorage.getItem('token')
  if (!isTokenValid(token)) return 0
  const parts = token.split('.')
  if (parts.length !== 3) return 0
  try {
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4)
    const payload = JSON.parse(atob(padded))
    return payload.user_id || 0
  } catch {
    return 0
  }
})

/** 是否查看自己的主页 */
const isSelf = computed(() => currentUserId.value > 0 && currentUserId.value === user.value.id)

/** 关注按钮文案 */
const followText = computed(() => user.value.is_following ? '已关注' : '+ 关注')

/** 关注按钮样式 */
const followButtonClass = computed(() => user.value.is_following ? 'followed' : '')

/** 注册日期格式化 */
const joinDate = computed(() => {
  if (!user.value.created_at) return ''
  const d = new Date(user.value.created_at * 1000)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
})

/** 是否有更多视频 */
const hasMoreVideos = computed(() => videos.value.length < videoTotal.value)

// ── 方法 ──

/**
 * 加载用户资料
 */
async function loadUser() {
  loading.value = true
  try {
    const uid = route.params.id ? Number(route.params.id) : currentUserId.value
    if (!uid) {
      ElMessage.warning('请先登录')
      return
    }
    const res = await getUserProfile(uid)
    const data = res.data || {}
    user.value = {
      id: data.id || uid,
      name: data.name || `用户 ${uid}`,
      avatar: data.avatar || '',
      background_image: data.background_image || '',
      signature: data.signature || '',
      follow_count: data.follow_count || 0,
      follower_count: data.follower_count || 0,
      total_favorited: data.total_favorited || 0,
      work_count: data.work_count || 0,
      favorite_count: data.favorite_count || 0,
      created_at: data.created_at || 0,
      is_following: data.is_following || false
    }
    // 加载视频
    await loadVideos(true)
  } catch {
    ElMessage.error('加载用户信息失败')
  } finally {
    loading.value = false
  }
}

/**
 * 加载用户投稿视频
 */
async function loadVideos(reset = false) {
  if (!user.value.id) return
  if (reset) {
    videoPage.value = 1
    videos.value = []
  }
  videoLoading.value = true
  try {
    const res = await getUserVideos(user.value.id, { page: videoPage.value, pageSize: videoPageSize })
    const data = res.data || {}
    const list = data.list || []
    videoTotal.value = data.total || 0
    videos.value = videoPage.value === 1 ? list : [...videos.value, ...list]
  } catch {
    ElMessage.error('加载视频列表失败')
  } finally {
    videoLoading.value = false
  }
}

/** 加载更多视频 */
function loadMoreVideos() {
  videoPage.value++
  loadVideos(false)
}

/**
 * 关注/取消关注
 */
async function handleFollow() {
  if (!isTokenValid()) {
    ElMessage.warning('请先登录')
    return
  }
  followLoading.value = true
  try {
    const action = user.value.is_following ? 2 : 1
    const res = await followUser(user.value.id, action)
    const data = res.data || {}
    user.value.is_following = !user.value.is_following
    // 更新后端返回的粉丝数（实时计数更准确）
    if (data.follower_count !== undefined) {
      user.value.follower_count = data.follower_count
    } else {
      user.value.follower_count += action === 1 ? 1 : -1
    }
    ElMessage.success(action === 1 ? '关注成功' : '已取消关注')
  } catch {
    ElMessage.error('操作失败，请重试')
  } finally {
    followLoading.value = false
  }
}

/** 格式化数字（K/W） */
function formatCount(n) {
  if (n >= 10000) return (n / 10000).toFixed(1) + '万'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k'
  return String(n)
}

// ── 生命周期 ──
onMounted(loadUser)
</script>

<template>
  <div class="profile-page" v-loading="loading">
    <!-- 背景图横幅 -->
    <div class="banner" :class="{ 'has-bg': user.background_image }">
      <img
        v-if="user.background_image"
        :src="user.background_image"
        class="banner-img"
        alt="背景图"
      />
      <div class="banner-overlay"></div>
    </div>

    <!-- 用户信息卡片 -->
    <div class="user-card">
      <div class="user-card-inner">
        <!-- 头像区 -->
        <div class="avatar-section">
          <el-avatar :size="96" :src="user.avatar" class="user-avatar">
            {{ user.name.slice(0, 1) }}
          </el-avatar>
        </div>

        <!-- 信息区 -->
        <div class="info-section">
          <div class="name-row">
            <h1 class="user-name">{{ user.name }}</h1>
            <span class="user-uid">UID: {{ user.id }}</span>
          </div>

          <p class="user-signature" v-if="user.signature">
            {{ user.signature }}
          </p>
          <p class="user-signature placeholder" v-else>
            这个人很懒，什么都没写~
          </p>

          <!-- 统计数字行 -->
          <div class="stats-row">
            <div class="stat-item" title="关注数">
              <span class="stat-num">{{ formatCount(user.follow_count) }}</span>
              <span class="stat-label">关注</span>
            </div>
            <div class="stat-divider"></div>
            <div class="stat-item" title="粉丝数">
              <span class="stat-num">{{ formatCount(user.follower_count) }}</span>
              <span class="stat-label">粉丝</span>
            </div>
            <div class="stat-divider"></div>
            <div class="stat-item" title="获赞数">
              <span class="stat-num">{{ formatCount(user.total_favorited) }}</span>
              <span class="stat-label">获赞</span>
            </div>
            <div class="stat-divider"></div>
            <div class="stat-item" title="投稿数">
              <span class="stat-num">{{ formatCount(user.work_count) }}</span>
              <span class="stat-label">投稿</span>
            </div>
            <div class="stat-divider"></div>
            <div class="stat-item date-item" v-if="user.created_at" title="加入时间">
              <el-icon :size="15"><UserFilled /></el-icon>
              <span class="stat-label join-date">{{ joinDate }} 加入</span>
            </div>
          </div>

          <!-- 关注按钮（非本人时显示） -->
          <div class="action-row" v-if="!isSelf && user.id">
            <el-button
              :class="followButtonClass"
              :loading="followLoading"
              :icon="user.is_following ? undefined : Plus"
              round
              size="large"
              @click="handleFollow"
            >
              {{ followText }}
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- Tab 栏 + 内容区 -->
    <div class="content-section">
      <div class="tabs-bar">
        <div
          class="tab-item"
          :class="{ active: activeTab === 'videos' }"
          @click="activeTab = 'videos'"
        >
          <el-icon :size="18"><VideoCameraFilled /></el-icon>
          <span>投稿</span>
          <span class="tab-count">{{ user.work_count }}</span>
        </div>
      </div>

      <!-- 投稿视频网格 -->
      <div v-if="activeTab === 'videos'" class="tab-content">
        <!-- 空状态 -->
        <el-empty v-if="!videoLoading && videos.length === 0" description="暂无投稿视频" />

        <!-- 视频网格 -->
        <div v-else class="video-grid">
          <VideoCard v-for="v in videos" :key="v.id" :video="v" />
        </div>

        <!-- 加载更多 -->
        <div class="load-more" v-if="hasMoreVideos">
          <el-button
            :loading="videoLoading"
            size="large"
            round
            plain
            @click="loadMoreVideos"
          >
            加载更多
          </el-button>
        </div>

        <!-- 加载中 -->
        <div class="loading-row" v-if="videoLoading && videos.length > 0">
          <span>加载中...</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* ── 页面容器 ── */
.profile-page {
  max-width: 1200px;
  margin: -28px auto 40px;
  background: #fff;
  border-radius: 0 0 12px 12px;
  overflow: hidden;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
}

/* ── 背景图横幅 ── */
.banner {
  position: relative;
  width: 100%;
  height: 200px;
  background: linear-gradient(135deg, #fb7299 0%, #ff9db5 30%, #ffb6c1 60%, #ffc8d6 100%);
  overflow: hidden;
}
.banner.has-bg {
  background: #e8e8e8;
}
.banner-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.banner-overlay {
  position: absolute;
  inset: 0;
  background: linear-gradient(to bottom, transparent 60%, rgba(255,255,255,0.15) 100%);
}

/* ── 用户信息卡片 ── */
.user-card {
  position: relative;
  margin-top: -50px;
  padding: 0 32px;
  z-index: 2;
}
.user-card-inner {
  display: flex;
  align-items: flex-start;
  gap: 24px;
  padding-bottom: 24px;
}

/* 头像 */
.avatar-section {
  flex-shrink: 0;
}
.user-avatar {
  border: 4px solid #fff;
  border-radius: 50%;
  box-shadow: 0 2px 12px rgba(0,0,0,0.08);
  background: var(--bili-pink, #fb7299);
  color: #fff;
  font-size: 36px;
  font-weight: 600;
}

/* 信息 */
.info-section {
  flex: 1;
  padding-top: 50px;
  min-width: 0;
}

.name-row {
  display: flex;
  align-items: baseline;
  gap: 12px;
  margin-bottom: 8px;
}
.user-name {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: #18191c;
  line-height: 1.2;
}
.user-uid {
  font-size: 13px;
  color: #9499a0;
  flex-shrink: 0;
}

.user-signature {
  margin: 0 0 16px;
  font-size: 14px;
  color: #61666d;
  line-height: 1.5;
  max-width: 560px;
  word-break: break-all;
}
.user-signature.placeholder {
  color: #c9ccd0;
  font-style: italic;
}

/* 统计数字行 */
.stats-row {
  display: flex;
  align-items: center;
  gap: 0;
  flex-wrap: wrap;
}
.stat-item {
  display: flex;
  align-items: baseline;
  gap: 4px;
  cursor: default;
  padding: 4px 0;
}
.stat-num {
  font-size: 18px;
  font-weight: 600;
  color: #18191c;
  line-height: 1;
}
.stat-label {
  font-size: 13px;
  color: #9499a0;
}
.stat-divider {
  width: 1px;
  height: 16px;
  background: #e3e5e7;
  margin: 0 16px;
}
.date-item {
  margin-left: 8px;
  color: #9499a0;
}
.join-date {
  font-size: 12px;
}

/* 关注按钮 */
.action-row {
  margin-top: 20px;
}
.action-row .el-button {
  min-width: 120px;
  height: 38px;
  font-size: 15px;
  font-weight: 500;
  background: var(--bili-pink, #fb7299);
  border-color: var(--bili-pink, #fb7299);
  color: #fff;
}
.action-row .el-button:hover {
  background: #fc8bab;
  border-color: #fc8bab;
}
.action-row .el-button.followed {
  background: #f1f2f3;
  border-color: #e3e5e7;
  color: #61666d;
}
.action-row .el-button.followed:hover {
  background: #e3e5e7;
  border-color: #c9ccd0;
}

/* ── 内容区 ── */
.content-section {
  padding: 0 32px 32px;
}

.tabs-bar {
  display: flex;
  gap: 0;
  border-bottom: 2px solid #f1f2f3;
  margin-bottom: 24px;
}
.tab-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 12px 20px;
  font-size: 15px;
  color: #61666d;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  margin-bottom: -2px;
  transition: color 0.2s, border-color 0.2s;
  user-select: none;
}
.tab-item:hover {
  color: var(--bili-pink, #fb7299);
}
.tab-item.active {
  color: var(--bili-pink, #fb7299);
  border-bottom-color: var(--bili-pink, #fb7299);
  font-weight: 600;
}
.tab-count {
  font-size: 12px;
  color: #9499a0;
  background: #f1f2f3;
  padding: 1px 8px;
  border-radius: 10px;
  margin-left: 2px;
}
.tab-item.active .tab-count {
  background: #fff0f3;
  color: var(--bili-pink, #fb7299);
}

.tab-content {
  min-height: 200px;
}

/* 视频网格 */
.video-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 20px 16px;
}

/* 加载更多 */
.load-more {
  text-align: center;
  padding: 32px 0 8px;
}
.loading-row {
  text-align: center;
  padding: 24px 0;
  color: #9499a0;
  font-size: 14px;
}

/* ── 响应式 ── */
@media (max-width: 1100px) {
  .video-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}
@media (max-width: 768px) {
  .user-card-inner {
    flex-direction: column;
    align-items: center;
    text-align: center;
  }
  .info-section {
    padding-top: 12px;
  }
  .name-row {
    justify-content: center;
  }
  .stats-row {
    justify-content: center;
  }
  .action-row {
    display: flex;
    justify-content: center;
  }
  .banner {
    height: 140px;
  }
  .user-avatar {
    margin-top: -48px;
  }
  .video-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
@media (max-width: 480px) {
  .video-grid {
    grid-template-columns: 1fr;
  }
  .content-section {
    padding: 0 16px 24px;
  }
  .user-card {
    padding: 0 16px;
  }
  .stat-divider {
    margin: 0 8px;
  }
}
</style>
