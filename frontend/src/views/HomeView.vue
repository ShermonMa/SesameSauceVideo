<!--
  HomeView.vue
  功能：首页，展示左侧分类侧边栏、首屏轮播 Banner、视频卡片网格、分页器；内测验证通过后自动刷新视频列表
  时间戳：2026-06-22 新增左侧 CategorySidebar，支持按分类关键词筛选视频；新增排序栏 UI
-->
<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { listVideos } from '../api/video.js'
import { onBetaVerified } from '../api/beta.js'
import VideoCard from '../components/VideoCard.vue'
import CategorySidebar from '../components/CategorySidebar.vue'

const router = useRouter()
const route = useRoute()

const list = ref([])
const total = ref(0)
const loading = ref(false)
const pageSize = 20

// 当前页码、关键词、排序、分类 ID 均从 URL 派生，刷新可保持状态
const page = computed(() => {
  const p = parseInt(route.query.page, 10)
  return p > 0 ? p : 1
})
const keyword = computed(() => (route.query.keyword || '').trim())
const orderBy = computed(() => route.query.order_by || 'publish_time_desc')
const categoryId = computed(() => {
  const id = parseInt(route.query.category_id, 10)
  return id > 0 ? id : 0
})

// Banner 取列表首屏前 5 条；后续翻页或搜索时不展示 Banner
const bannerList = computed(() => {
  if (page.value !== 1 || keyword.value || categoryId.value > 0) return []
  return list.value.slice(0, 5)
})

// 拉取列表，写入 list/total
async function fetchList() {
  loading.value = true
  try {
    const res = await listVideos({
      page: page.value,
      pageSize,
      keyword: keyword.value || undefined,
      orderBy: orderBy.value,
      categoryId: categoryId.value > 0 ? categoryId.value : undefined
    })
    list.value = res?.data?.list || []
    total.value = res?.data?.total || 0
  } catch (e) {
    list.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// 分页切换：写入 query，watch 触发重拉并滚到顶部
function handlePageChange(p) {
  router.push({ path: '/', query: { ...route.query, page: p } })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

// 排序切换：写入 query，重置页码到 1
function handleOrderChange(val) {
  router.push({ path: '/', query: { ...route.query, order_by: val, page: 1 } })
}

// 分类切换：写入 query，重置页码到 1
function handleCategoryChange(id) {
  const q = { ...route.query, page: 1 }
  if (id > 0) {
    q.category_id = String(id)
  } else {
    delete q.category_id
  }
  router.push({ path: '/', query: q })
}

// 点击 Banner 跳详情
function goDetail(id) {
  router.push('/video/' + id)
}

onMounted(() => {
  fetchList()
  // 内测密钥验证通过后，当前页面可能已因缺少 X-Beta-Token 提前请求失败，需重新拉取
  onBetaVerified(fetchList)
})
watch(() => [route.query.page, route.query.keyword, route.query.order_by, route.query.category_id], fetchList)
</script>

<template>
  <div class="home-layout">
    <!-- 左侧分类侧边栏，搜索时隐藏 -->
    <CategorySidebar
      v-if="!keyword"
      :model-value="categoryId"
      @update:model-value="handleCategoryChange"
    />

    <!-- 右侧主内容区 -->
    <div class="home-main" v-loading="loading">
      <el-carousel
        v-if="bannerList.length > 0"
        class="banner"
        height="320px"
        :interval="4000"
        arrow="hover"
      >
        <el-carousel-item v-for="v in bannerList" :key="v.id" @click="goDetail(v.id)">
          <div class="banner-slide">
            <img :src="v.cover_url" :alt="v.title" class="banner-img" />
            <div class="banner-mask">
              <div class="banner-title">{{ v.title }}</div>
              <div class="banner-author">{{ v.author?.name || '未知用户' }}</div>
            </div>
          </div>
        </el-carousel-item>
      </el-carousel>

      <div v-if="keyword" class="search-tip">
        搜索 "<span>{{ keyword }}</span>" 的结果（共 {{ total }} 条）
      </div>

      <div v-if="keyword" class="sort-bar">
        <span class="sort-label">排序：</span>
        <el-radio-group v-model="orderBy" size="small" @change="handleOrderChange">
          <el-radio-button value="publish_time_desc">最新发布</el-radio-button>
          <el-radio-button value="play_count_desc">最多播放</el-radio-button>
          <el-radio-button value="duration_desc">最长时长</el-radio-button>
          <el-radio-button value="duration_asc">最短时长</el-radio-button>
        </el-radio-group>
      </div>

      <div class="section-title">{{ keyword ? '搜索结果' : (categoryId > 0 ? '分类筛选' : '推荐视频') }}</div>

      <div v-if="list.length > 0" class="video-grid">
        <VideoCard v-for="v in list" :key="v.id" :video="v" />
      </div>

      <el-empty v-else-if="!loading" :description="keyword ? '未找到相关视频' : '暂无视频'" />

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
  </div>
</template>

<style scoped>
.home-layout {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  min-height: 60vh;
}

.home-main {
  flex: 1;
  min-width: 0;  /* 防止 flex 子元素溢出 */
}

.banner {
  border-radius: 12px;
  overflow: hidden;
  margin-bottom: 24px;
}
.banner-slide {
  position: relative;
  width: 100%;
  height: 100%;
  cursor: pointer;
  background: #000;
}
.banner-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.banner-mask {
  position: absolute;
  inset: auto 0 0 0;
  padding: 24px;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.7), transparent);
  color: #fff;
}
.banner-title {
  font-size: 22px;
  font-weight: 600;
  margin-bottom: 6px;
  display: -webkit-box;
  -webkit-line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.banner-author {
  font-size: 14px;
  opacity: 0.9;
}
.search-tip {
  font-size: 14px;
  color: var(--bili-text-light, #61666d);
  margin-bottom: 12px;
}
.sort-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}
.sort-label {
  font-size: 14px;
  color: var(--bili-text-light, #61666d);
  flex-shrink: 0;
}
.section-title {
  font-size: 18px;
  font-weight: 600;
  margin: 4px 0 16px;
  color: var(--bili-text, #18191c);
}
.search-tip span {
  color: var(--bili-pink, #fb7299);
  font-weight: 600;
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
    grid-template-columns: repeat(4, 1fr);
  }
}
.pagination {
  margin-top: 32px;
  display: flex;
  justify-content: center;
}

/* 移动端隐藏侧边栏后，全宽显示 */
@media (max-width: 768px) {
  .home-layout {
    flex-direction: column;
    gap: 0;
  }
}
</style>
