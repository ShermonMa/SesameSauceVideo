<!--
  MyVideosView.vue
  功能：我的视频列表页；展示登录用户全部视频含状态徽章（已发布/处理中/失败/已取消），
        处理中视频显示进度条与取消按钮；点击仅"已发布"状态的卡片跳详情
  时间戳：2026-05-01
-->
<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { listMyVideos, cancelUpload } from '../api/video.js'
import { formatDuration, formatPlayCount, formatRelativeTime } from '../utils/format.js'

const router = useRouter()
const route = useRoute()

const list = ref([])
const total = ref(0)
const loading = ref(false)
const pageSize = 20

// 当前页码从 URL 派生，刷新可保持状态
const page = computed(() => {
  const p = parseInt(route.query.page, 10)
  return p > 0 ? p : 1
})

// 状态映射：用 el-tag 的 type 表达视觉语义
const STATUS_MAP = {
  0: { type: 'success', label: '已发布' },
  1: { type: 'warning', label: '处理中' },
  2: { type: 'danger', label: '处理失败' },
  3: { type: 'info', label: '已取消' }
}

// statusOf 返回当前 status 对应的展示配置，未知状态兜底为"未知"
function statusOf(status) {
  return STATUS_MAP[status] || { type: 'info', label: '未知' }
}

// fetchList 拉取我的视频列表，写入 list/total
async function fetchList() {
  loading.value = true
  try {
    const res = await listMyVideos({ page: page.value, pageSize })
    list.value = res?.data?.list || []
    total.value = res?.data?.total || 0
  } catch (e) {
    list.value = []
    total.value = 0
    ElMessage.error('获取我的视频失败')
  } finally {
    loading.value = false
  }
}

// handleRefresh 刷新按钮点击：重新拉取当前页
async function handleRefresh() {
  await fetchList()
  ElMessage.success('已刷新')
}

// handlePageChange 分页切换：写入 query，触发重拉并滚到顶部
function handlePageChange(p) {
  router.push({ path: '/my-videos', query: { ...route.query, page: p } })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

// goDetail 仅"已发布"状态的视频允许跳转详情，其余状态点击给出提示
function goDetail(item) {
  if (item.status === 0) {
    router.push('/video/' + item.id)
    return
  }
  if (item.status === 1) {
    ElMessage.info('视频处理中，请稍后刷新')
    return
  }
  if (item.status === 2) {
    ElMessage.error('视频处理失败，请重新上传')
    return
  }
  if (item.status === 3) {
    ElMessage.info('视频已取消，可重新确认或删除')
    return
  }
}

// handleCancel 取消处理中的视频
async function handleCancel(item, e) {
  e.stopPropagation()
  try {
    await cancelUpload(item.id)
    ElMessage.success('已取消处理')
    await fetchList()
  } catch (e) {
    ElMessage.error('取消失败')
  }
}

onMounted(fetchList)
</script>

<template>
  <div class="my-videos" v-loading="loading">
    <div class="header-bar">
      <h2 class="title">我的视频</h2>
      <el-button :icon="Refresh" type="primary" plain @click="handleRefresh">刷新</el-button>
    </div>

    <el-alert
      type="info"
      :closable="false"
      show-icon
      title="新上传的视频需要数秒至数十秒完成处理（转码为多码率 m3u8），请用刷新按钮查看最新状态"
      style="margin-bottom: 16px;"
    />

    <div v-if="list.length > 0" class="video-grid">
      <div
        v-for="item in list"
        :key="item.id"
        class="video-card"
        :class="{ 'is-disabled': item.status !== 0 }"
        @click="goDetail(item)"
      >
        <div class="cover-wrapper">
          <img
            v-if="item.cover_url"
            class="cover"
            :src="item.cover_url"
            :alt="item.title"
            loading="lazy"
          />
          <div v-else class="cover-placeholder">暂无封面</div>
          <span v-if="item.duration > 0" class="duration-badge">
            {{ formatDuration(item.duration) }}
          </span>
          <el-tag
            class="status-badge"
            :type="statusOf(item.status).type"
            size="small"
            effect="dark"
          >
            {{ statusOf(item.status).label }}
          </el-tag>
        </div>
        <div class="info">
          <div class="title-row" :title="item.title">{{ item.title || '无标题' }}</div>
          <div v-if="item.status === 1 && item.progress > 0" class="progress-row">
            <el-progress :percentage="item.progress" :show-text="true" :stroke-width="6" />
          </div>
          <div class="stats">
            <span>{{ formatPlayCount(item.play_count) }} 次观看</span>
            <span class="dot">·</span>
            <span>{{ formatRelativeTime(item.created_at) }}</span>
          </div>
          <div v-if="item.status === 1" class="action-row">
            <el-button size="small" type="danger" plain @click="(e) => handleCancel(item, e)">
              取消处理
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <el-empty
      v-else-if="!loading"
      description="暂无视频，去上传第一个吧"
    >
      <el-button type="primary" @click="router.push('/upload')">去上传</el-button>
    </el-empty>

    <div v-if="total > pageSize" class="pagination">
      <el-pagination
        background
        layout="prev, pager, next, total"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="handlePageChange"
      />
    </div>
  </div>
</template>

<style scoped>
.my-videos {
  min-height: 60vh;
}
.header-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--bili-text, #18191c);
}
.video-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}
@media (min-width: 768px) {
  .video-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}
@media (min-width: 1200px) {
  .video-grid {
    grid-template-columns: repeat(5, 1fr);
  }
}
.video-card {
  cursor: pointer;
  background: #fff;
  border-radius: 8px;
  overflow: hidden;
  transition: box-shadow 0.2s, transform 0.2s;
}
.video-card:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  transform: translateY(-2px);
}
.video-card.is-disabled {
  cursor: not-allowed;
  opacity: 0.78;
}
.video-card.is-disabled:hover {
  transform: none;
}
.cover-wrapper {
  position: relative;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  background: #f0f0f0;
}
.cover {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.cover-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #999;
  font-size: 13px;
  background: linear-gradient(135deg, #f5f5f5 0%, #e0e0e0 100%);
}
.duration-badge {
  position: absolute;
  right: 8px;
  bottom: 8px;
  background: rgba(0, 0, 0, 0.7);
  color: #fff;
  font-size: 12px;
  padding: 2px 6px;
  border-radius: 4px;
  line-height: 1.4;
}
.status-badge {
  position: absolute;
  left: 8px;
  top: 8px;
}
.info {
  padding: 10px 12px 12px;
}
.title-row {
  font-size: 14px;
  font-weight: 500;
  color: var(--bili-text, #18191c);
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  height: 40px;
}
.progress-row {
  margin-top: 8px;
}
.stats {
  margin-top: 8px;
  font-size: 12px;
  color: var(--bili-text-light, #61666d);
}
.dot {
  margin: 0 4px;
}
.action-row {
  margin-top: 8px;
}
.pagination {
  margin-top: 32px;
  display: flex;
  justify-content: center;
}
</style>
