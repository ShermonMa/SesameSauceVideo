<!--
  CommentSection.vue
  功能：视频评论区整体容器；包含评论发布框、一级评论列表与分页；videoId 透传 hashid 字符串，不做数字强转
  时间戳：2026-05-01
-->
<script setup>
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { publishComment, getCommentList } from '../api/comment.js'
import CommentItem from './CommentItem.vue'

const props = defineProps({
  videoId: { type: [String, Number], required: true }
})

const comments = ref([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const loading = ref(false)
const content = ref('')
const publishLoading = ref(false)

/**
 * 拉取一级评论列表（含前3条二级回复）
 */
const fetchComments = async () => {
  loading.value = true
  try {
    const res = await getCommentList({
      video_id: props.videoId,
      page: page.value,
      size: pageSize.value
    })
    comments.value = res.data.comments || []
    total.value = res.data.total || 0
  } catch (e) {
    ElMessage.error('获取评论列表失败')
  } finally {
    loading.value = false
  }
}

/**
 * 发布一级评论
 */
const handlePublish = async () => {
  const text = content.value.trim()
  if (!text) {
    ElMessage.warning('评论内容不能为空')
    return
  }
  publishLoading.value = true
  try {
    await publishComment({ video_id: props.videoId, content: text })
    ElMessage.success('评论发布成功')
    content.value = ''
    page.value = 1
    await fetchComments()
  } catch (e) {
    // 错误由 request 拦截器统一提示
  } finally {
    publishLoading.value = false
  }
}

/**
 * 评论分页切换
 */
const handlePageChange = (newPage) => {
  page.value = newPage
  fetchComments()
}

/**
 * 某条一级评论的子回复数量变更时，刷新列表以同步显示
 */
const handleReplyCountChange = () => {
  fetchComments()
}

watch(() => props.videoId, (newId) => {
  if (newId) {
    page.value = 1
    fetchComments()
  }
}, { immediate: true })
</script>

<template>
  <div class="comment-section">
    <h3 class="section-title">
      评论
      <span v-if="total > 0" class="comment-count">({{ total }})</span>
    </h3>

    <!-- 发布框 -->
    <div class="publish-box">
      <el-input
        v-model="content"
        type="textarea"
        :rows="3"
        placeholder="发一条友善的评论"
        maxlength="512"
        show-word-limit
      />
      <div class="publish-actions">
        <el-button
          type="primary"
          :loading="publishLoading"
          @click="handlePublish"
        >
          发表评论
        </el-button>
      </div>
    </div>

    <!-- 评论列表 -->
    <div v-loading="loading" class="comment-list">
      <CommentItem
        v-for="item in comments"
        :key="item.id"
        :comment="item"
        :video-id="videoId"
        @reply-published="handleReplyCountChange"
      />
      <el-empty v-if="!loading && comments.length === 0" description="暂无评论，快来抢沙发" />
    </div>

    <!-- 分页 -->
    <el-pagination
      v-if="total > pageSize"
      :current-page="page"
      :page-size="pageSize"
      :total="total"
      layout="prev, pager, next"
      class="pagination"
      @current-change="handlePageChange"
    />
  </div>
</template>

<style scoped>
.comment-section {
  margin-top: 24px;
}
.section-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 16px 0;
}
.comment-count {
  font-size: 14px;
  color: #999;
  font-weight: 400;
}
.publish-box {
  background: #fff;
  padding: 16px;
  border-radius: 8px;
  margin-bottom: 16px;
}
.publish-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
.comment-list {
  min-height: 120px;
}
.pagination {
  justify-content: center;
  margin-top: 16px;
}
</style>
