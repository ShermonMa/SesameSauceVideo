<!--
 * VideoDetailView.vue
 * 功能：视频详情与播放页；2026-06-19 新增点赞按钮、倍速/分辨率选中高亮
 *      2026-06-20 点赞按钮增加 loading 与 disabled 状态，防止请求期间重复点击
 *      2026-06-20 修复 HLS 分辨率切换：关闭自动级别并改用 nextLevel 平滑切换，避免 currentLevel 立即 flush buffer 导致画面卡死与回退到 1080p
-->
<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getVideo, getPlayUrl, reportProgress, likeVideo } from '../api/video.js'
import CommentSection from '../components/CommentSection.vue'
import Hls from 'hls.js'

const route = useRoute()
const video = ref(null)
const playUrl = ref('')
const loading = ref(true)
const videoEl = ref(null)
const liking = ref(false)
let hls = null

const token = localStorage.getItem('token')
let lastReportTime = 0
let lastReportProgress = -1
let resumed = false

const fetchVideo = async () => {
  try {
    const res = await getVideo(route.params.id)
    video.value = res.data
  } catch (e) {
    ElMessage.error('获取视频详情失败')
  }
}

const fetchPlayUrl = async () => {
  try {
    const res = await getPlayUrl(route.params.id)
    playUrl.value = res.data.play_url
  } catch (e) {
    ElMessage.error('获取播放地址失败')
  }
}

const initPlayer = () => {
  if (!videoEl.value || !playUrl.value) return

  if (hls) {
    hls.destroy()
    hls = null
  }

  const url = playUrl.value
  // 预签名 URL 携带 query string，不能直接用 endsWith('.m3u8')，需用 pathname 判断
  let isM3u8 = false
  try {
    isM3u8 = new URL(url).pathname.endsWith('.m3u8')
  } catch (e) {
    isM3u8 = url.endsWith('.m3u8')
  }
  if (Hls.isSupported() && isM3u8) {
    hls = new Hls()
    hls.loadSource(url)
    hls.attachMedia(videoEl.value)
    hls.on(Hls.Events.MANIFEST_PARSED, () => {
      // 自动播放可在此触发
    })
  } else if (videoEl.value.canPlayType('application/vnd.apple.mpegurl')) {
    // Safari 原生支持 HLS
    videoEl.value.src = url
  } else {
    videoEl.value.src = url
  }
}

const doReport = async (progress) => {
  if (!token || !video.value) return
  try {
    await reportProgress(route.params.id, progress)
    lastReportProgress = progress
  } catch (e) {
    // 上报失败静默处理，不影响播放体验
  }
}

const handleTimeUpdate = () => {
  if (!videoEl.value) return
  const now = Date.now()
  const current = Math.floor(videoEl.value.currentTime)
  if (now - lastReportTime >= 15000 && current !== lastReportProgress) {
    doReport(current)
    lastReportTime = now
  }
}

const handlePause = () => {
  if (!videoEl.value) return
  doReport(Math.floor(videoEl.value.currentTime))
}

const handleEnded = () => {
  if (!video.value) return
  doReport(video.value.duration)
}

const handleSeeked = () => {
  if (!videoEl.value) return
  doReport(Math.floor(videoEl.value.currentTime))
}

const handleVisibilityChange = () => {
  if (document.hidden && videoEl.value) {
    doReport(Math.floor(videoEl.value.currentTime))
  }
}

let errorRetryCount = 0
const MAX_ERROR_RETRY = 1

const handleError = async () => {
  if (!video.value) return
  // MEDIA_ERR_SRC_NOT_SUPPORTED=4 表示源类型不支持，属于不可恢复错误，重拉 URL 无济于事
  if (videoEl.value?.error?.code === 4) {
    ElMessage.error('视频源格式不支持')
    return
  }
  // 限制重试次数，防止源加载持续失败导致 error 事件死循环
  if (errorRetryCount >= MAX_ERROR_RETRY) {
    ElMessage.error('播放失败，请刷新页面重试')
    return
  }
  errorRetryCount++
  try {
    const res = await getPlayUrl(route.params.id)
    playUrl.value = res.data.play_url
    initPlayer()
    if (videoEl.value) {
      videoEl.value.play()
    }
  } catch (e) {
    ElMessage.error('播放地址已过期，刷新失败')
  }
}

const currentRate = ref(1)
const currentLevel = ref(-1)

// setRate 设置视频播放倍速并记录当前选中状态
const setRate = (rate) => {
  if (videoEl.value) {
    videoEl.value.playbackRate = rate
    currentRate.value = rate
  }
}

// setLevel 设置 HLS 清晰度并记录当前选中状态
// 使用 nextLevel 让播放器在当前片段播完后平滑切换，避免 currentLevel 立即 flush buffer
// 导致画面卡死并回退到旧清晰度；hls.js 中 nextLevel=-1 表示启用自动级别选择
const setLevel = (level) => {
  if (!hls) return
  currentLevel.value = level
  hls.nextLevel = level
}

// toggleLike 切换视频点赞状态，未登录时提示登录；请求期间锁定按钮防止重复提交
const toggleLike = async () => {
  if (!token) {
    ElMessage.warning('请先登录')
    return
  }
  if (!video.value || liking.value) return

  liking.value = true
  const action = video.value.is_liked ? 2 : 1
  try {
    const res = await likeVideo(route.params.id, action)
    video.value.is_liked = action === 1
    video.value.favorite_count = res.data.like_count
    ElMessage.success(action === 1 ? '点赞成功' : '已取消点赞')
  } catch (e) {
    ElMessage.error('操作失败')
  } finally {
    liking.value = false
  }
}

const formatTime = (dateStr) => {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return d.toLocaleString()
}

onMounted(async () => {
  await fetchVideo()
  await fetchPlayUrl()
  loading.value = false

  initPlayer()

  if (video.value && video.value.progress > 0 && videoEl.value) {
    ElMessageBox.confirm(
      `上次观看到 ${video.value.progress} 秒，是否继续？`,
      '继续观看',
      { confirmButtonText: '继续', cancelButtonText: '从头开始', type: 'info' }
    ).then(() => {
      videoEl.value.currentTime = video.value.progress
      resumed = true
    }).catch(() => {
      videoEl.value.currentTime = 0
    })
  }

  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onUnmounted(() => {
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  if (hls) {
    hls.destroy()
    hls = null
  }
})
</script>

<template>
  <div class="detail-container" v-loading="loading">
    <el-card v-if="video">
      <video
        ref="videoEl"
        controls
        :poster="video.cover_url"
        style="width: 100%; max-height: 480px; background: #000;"
        @timeupdate="handleTimeUpdate"
        @pause="handlePause"
        @ended="handleEnded"
        @seeked="handleSeeked"
        @error="handleError"
      ></video>

      <div class="rate-bar">
        <el-button-group>
          <el-button size="small" :type="currentRate === 0.5 ? 'primary' : ''" @click="setRate(0.5)">0.5x</el-button>
          <el-button size="small" :type="currentRate === 1 ? 'primary' : ''" @click="setRate(1)">1x</el-button>
          <el-button size="small" :type="currentRate === 1.5 ? 'primary' : ''" @click="setRate(1.5)">1.5x</el-button>
          <el-button size="small" :type="currentRate === 2 ? 'primary' : ''" @click="setRate(2)">2x</el-button>
        </el-button-group>
        <el-button-group style="margin-left: 12px;">
          <el-button size="small" :type="currentLevel === -1 ? 'primary' : ''" @click="setLevel(-1)">自动</el-button>
          <el-button size="small" :type="currentLevel === 1 ? 'primary' : ''" @click="setLevel(1)">1080p</el-button>
          <el-button size="small" :type="currentLevel === 0 ? 'primary' : ''" @click="setLevel(0)">360p</el-button>
        </el-button-group>
      </div>

      <div class="meta-section">
        <h2>{{ video.title }}</h2>
        <p class="desc">{{ video.description }}</p>

        <div class="author-row" v-if="video.author">
          <el-avatar :size="40" :src="video.author.avatar || ''" />
          <div class="author-info">
            <span class="author-name">{{ video.author.name }}</span>
            <span class="author-stats">
              关注 {{ video.author.follow_count }} · 粉丝 {{ video.author.follower_count }}
            </span>
          </div>
        </div>

        <div class="like-row">
          <el-button
            size="small"
            :type="video.is_liked ? 'primary' : ''"
            :loading="liking"
            :disabled="liking || !video"
            @click="toggleLike"
          >
            {{ video.is_liked ? '已点赞' : '点赞' }} {{ video.favorite_count }}
          </el-button>
        </div>

        <div class="tag-row">
          <el-tag v-for="cat in video.categories" :key="cat" size="small" style="margin-right: 8px;">
            {{ cat }}
          </el-tag>
        </div>

        <div class="stats-row">
          <span>时长: {{ video.duration }}秒</span>
          <span>分辨率: {{ video.width }}x{{ video.height }}</span>
          <span>播放: {{ video.play_count }}</span>
          <span>点赞: {{ video.favorite_count }}</span>
          <span>评论: {{ video.comment_count }}</span>
          <span>发布时间: {{ formatTime(video.publish_time) }}</span>
        </div>
      </div>

      <CommentSection :video-id="route.params.id" />
    </el-card>
    <el-empty v-else description="视频加载失败或不存在" />
  </div>
</template>

<style scoped>
.detail-container {
  max-width: 960px;
  margin: 24px auto;
}
.rate-bar {
  margin-top: 12px;
}
.meta-section {
  margin-top: 16px;
}
.meta-section h2 {
  margin: 0 0 8px 0;
}
.desc {
  color: #666;
  margin: 0 0 12px 0;
}
.author-row {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
}
.author-info {
  margin-left: 12px;
  display: flex;
  flex-direction: column;
}
.author-name {
  font-weight: 500;
  font-size: 15px;
}
.author-stats {
  color: #999;
  font-size: 13px;
}
.tag-row {
  margin-bottom: 12px;
}
.like-row {
  margin-bottom: 12px;
}
.stats-row {
  color: #999;
  font-size: 14px;
  line-height: 1.8;
}
.stats-row span {
  margin-right: 16px;
}
</style>
