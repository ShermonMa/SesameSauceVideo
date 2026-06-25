<!--
  VideoCard.vue
  功能：视频卡片组件，主页网格使用，展示封面、标题、UP主、播放量、时长
  时间戳：2026-04-26
-->
<script setup>
import { useRouter } from 'vue-router'
import { formatDuration, formatPlayCount, formatRelativeTime } from '../utils/format.js'

const props = defineProps({
  video: { type: Object, required: true }
})

const router = useRouter()

// 跳转到视频详情页
function goDetail() {
  router.push('/video/' + props.video.id)
}
</script>

<template>
  <div class="video-card" @click="goDetail">
    <div class="cover-wrapper">
      <img class="cover" :src="video.cover_url || ''" :alt="video.title" loading="lazy" />
      <span class="duration-badge">{{ formatDuration(video.duration) }}</span>
    </div>
    <div class="info">
      <div class="title" :title="video.title">{{ video.title || '无标题' }}</div>
      <div class="meta">
        <el-avatar :src="video.author?.avatar" :size="22" class="avatar">
          {{ (video.author?.name || 'U').slice(0, 1) }}
        </el-avatar>
        <span class="up-name">{{ video.author?.name || '未知用户' }}</span>
      </div>
      <div class="stats">
        <span>{{ formatPlayCount(video.play_count) }} 次观看</span>
        <span class="dot">·</span>
        <span>{{ formatRelativeTime(video.publish_time) }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
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
  transition: transform 0.3s;
}
.video-card:hover .cover {
  transform: scale(1.04);
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
.info {
  padding: 10px 12px 12px;
}
.title {
  font-size: 14px;
  font-weight: 500;
  color: var(--bili-text, #18191c);
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  height: 40px;
  transition: color 0.2s;
}
.video-card:hover .title {
  color: var(--bili-pink, #fb7299);
}
.meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  color: var(--bili-text-light, #61666d);
  font-size: 12px;
}
.up-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.avatar {
  flex-shrink: 0;
  background: #f4f5f7;
  color: #61666d;
  font-size: 12px;
}
.stats {
  margin-top: 4px;
  font-size: 12px;
  color: var(--bili-text-light, #61666d);
}
.dot {
  margin: 0 4px;
}
</style>
