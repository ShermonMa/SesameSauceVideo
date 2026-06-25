<!--
  CommentItem.vue
  功能：单条一级评论展示；包含前3条二级回复预览、展开完整楼中楼、回复输入框、点赞；videoId 透传 hashid 字符串，不做数字强转
  时间戳：2026-05-01
  2026-06-21 点赞状态使用后端返回的 comment.is_liked 初始化，避免刷新后丢失
  2026-06-22 点赞图标从 emoji 替换为 before_likes.png / after_likes.png 图片切换
-->
<script setup>
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { getReplyList, publishComment, likeComment } from '../api/comment.js'

const props = defineProps({
  comment: { type: Object, required: true },
  videoId: { type: [String, Number], required: true }
})

const emit = defineEmits(['reply-published'])

const replyContent = ref('')
const showReplyBox = ref(false)
const showAllReplies = ref(false)
const replyPage = ref(1)
const replyPageSize = ref(20)
const replyList = ref([])
const replyTotal = ref(0)
const replyLoading = ref(false)
const publishLoading = ref(false)
const likeLoading = ref(false)
const liked = ref(props.comment.is_liked || false)
const localLikeCount = ref(props.comment.like_count || 0)

/**
 * 是否已删除（后端脱敏返回"评论已删除"）
 */
const isDeleted = computed(() => props.comment.content === '评论已删除')

/**
 * 预览的二级回复（前3条，来自一级列表接口）
 */
const previewReplies = computed(() => props.comment.replies || [])

/**
 * 是否还有更多二级回复未展开
 */
const hasMoreReplies = computed(() => {
  const total = props.comment.reply_count || 0
  return total > previewReplies.value.length && !showAllReplies.value
})

/**
 * 切换回复输入框显示/隐藏
 */
const toggleReplyBox = () => {
  showReplyBox.value = !showReplyBox.value
}

/**
 * 展开楼中楼完整列表，首次展开时加载分页数据
 */
const expandReplies = async () => {
  showAllReplies.value = true
  if (replyList.value.length === 0) {
    await loadReplies()
  }
}

/**
 * 加载楼中楼分页数据
 */
const loadReplies = async () => {
  replyLoading.value = true
  try {
    const res = await getReplyList({
      root_id: props.comment.id,
      page: replyPage.value,
      size: replyPageSize.value
    })
    replyList.value = res.data.replies || []
    replyTotal.value = res.data.total || 0
  } catch (e) {
    ElMessage.error('加载回复失败')
  } finally {
    replyLoading.value = false
  }
}

/**
 * 楼中楼分页切换
 */
const handleReplyPageChange = (newPage) => {
  replyPage.value = newPage
  loadReplies()
}

/**
 * 发布二级评论（楼中楼回复）
 */
const handlePublishReply = async () => {
  const text = replyContent.value.trim()
  if (!text) {
    ElMessage.warning('回复内容不能为空')
    return
  }
  publishLoading.value = true
  try {
    await publishComment({
      video_id: props.videoId,
      content: text,
      parent_id: props.comment.id
    })
    ElMessage.success('回复成功')
    replyContent.value = ''
    showReplyBox.value = false
    // 如果是展开状态，刷新楼中楼列表；否则触发父组件刷新评论列表
    if (showAllReplies.value) {
      replyPage.value = 1
      await loadReplies()
    }
    emit('reply-published')
  } catch (e) {
    // 错误由 request 拦截器统一提示
  } finally {
    publishLoading.value = false
  }
}

/**
 * 点赞/取消点赞当前一级评论
 */
const handleLike = async () => {
  if (likeLoading.value || isDeleted.value) return
  likeLoading.value = true
  const action = liked.value ? 2 : 1
  console.log('[CommentItem] handleLike start', { commentId: props.comment.id, beforeLiked: liked.value, beforeCount: localLikeCount.value, action })
  try {
    const res = await likeComment({ comment_id: props.comment.id, action })
    console.log('[CommentItem] likeComment response', { commentId: props.comment.id, action, response: res })
    liked.value = !liked.value
    localLikeCount.value += action === 1 ? 1 : -1
    console.log('[CommentItem] local state updated', { commentId: props.comment.id, afterLiked: liked.value, afterCount: localLikeCount.value })
  } catch (e) {
    console.error('[CommentItem] likeComment error', { commentId: props.comment.id, action, error: e })
    // 错误由 request 拦截器统一提示
  } finally {
    likeLoading.value = false
  }
}
</script>

<template>
  <div class="comment-item">
    <div class="main-comment">
      <el-avatar :size="40" :src="comment.user_avatar || ''" class="avatar">
        {{ comment.user_name ? comment.user_name[0] : '?' }}
      </el-avatar>
      <div class="content-wrap">
        <div class="header-line">
          <span class="user-name">{{ comment.user_name }}</span>
          <span class="time">{{ comment.created_at }}</span>
        </div>
        <p class="content-text" :class="{ deleted: isDeleted }">
          {{ comment.content }}
        </p>
        <div class="action-bar">
          <span
            class="action-btn like-btn"
            :class="{ active: liked }"
            @click="handleLike"
          >
            <img
              :src="liked ? '/after_likes.png' : '/before_likes.png'"
              alt="点赞"
              class="like-icon"
            />
            <span class="like-count">{{ localLikeCount }}</span>
          </span>
          <span class="action-btn" @click="toggleReplyBox">回复</span>
        </div>

        <!-- 回复输入框 -->
        <div v-if="showReplyBox" class="reply-box">
          <el-input
            v-model="replyContent"
            type="textarea"
            :rows="2"
            :placeholder="`回复 @${comment.user_name}`"
            maxlength="512"
            show-word-limit
          />
          <div class="reply-actions">
            <el-button text size="small" @click="showReplyBox = false">
              取消
            </el-button>
            <el-button
              type="primary"
              size="small"
              :loading="publishLoading"
              @click="handlePublishReply"
            >
              发布回复
            </el-button>
          </div>
        </div>

        <!-- 二级回复预览（前3条） -->
        <div v-if="previewReplies.length > 0 && !showAllReplies" class="reply-preview">
          <div
            v-for="reply in previewReplies"
            :key="reply.id"
            class="reply-item"
          >
            <span class="reply-user">{{ reply.user_name }}</span>
            <span class="reply-content" :class="{ deleted: reply.content === '评论已删除' }">
              {{ reply.content }}
            </span>
          </div>
          <!-- 展开更多 -->
          <div v-if="hasMoreReplies" class="expand-btn" @click="expandReplies">
            展开更多回复（{{ comment.reply_count }}条）
          </div>
        </div>

        <!-- 展开后的完整楼中楼列表 -->
        <div v-if="showAllReplies" v-loading="replyLoading" class="reply-full">
          <div
            v-for="reply in replyList"
            :key="reply.id"
            class="reply-item"
          >
            <el-avatar :size="24" :src="reply.user_avatar || ''" />
            <div class="reply-body">
              <span class="reply-user">{{ reply.user_name }}</span>
              <span class="reply-time">{{ reply.created_at }}</span>
              <p class="reply-content" :class="{ deleted: reply.content === '评论已删除' }">
                {{ reply.content }}
              </p>
            </div>
          </div>
          <el-pagination
            v-if="replyTotal > replyPageSize"
            :current-page="replyPage"
            :page-size="replyPageSize"
            :total="replyTotal"
            layout="prev, pager, next"
            small
            @current-change="handleReplyPageChange"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.comment-item {
  padding: 16px 0;
  border-bottom: 1px solid #f1f1f1;
}
.main-comment {
  display: flex;
  gap: 12px;
}
.avatar {
  flex-shrink: 0;
  background: var(--bili-pink, #fb7299);
  color: #fff;
  font-size: 14px;
}
.content-wrap {
  flex: 1;
  min-width: 0;
}
.header-line {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.user-name {
  font-size: 14px;
  color: #fb7299;
  font-weight: 500;
}
.time {
  font-size: 12px;
  color: #999;
}
.content-text {
  margin: 0 0 8px 0;
  font-size: 14px;
  line-height: 1.6;
  color: #18191c;
  word-break: break-all;
}
.content-text.deleted {
  color: #999;
  font-style: italic;
}
.action-bar {
  display: flex;
  gap: 16px;
  margin-bottom: 8px;
}
.action-btn {
  font-size: 13px;
  color: #999;
  cursor: pointer;
  user-select: none;
  transition: color 0.2s;
}
.action-btn:hover {
  color: var(--bili-pink, #fb7299);
}
.action-btn.active {
  color: var(--bili-pink, #fb7299);
  font-weight: 500;
}
.like-btn {
  display: inline-flex;
  align-items: center;
}
.like-icon {
  width: 18px;
  height: 18px;
  vertical-align: middle;
  margin-right: 2px;
}
.like-count {
  vertical-align: middle;
}
.reply-box {
  margin: 8px 0;
  padding: 12px;
  background: #f4f5f7;
  border-radius: 6px;
}
.reply-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}
.reply-preview {
  margin-top: 8px;
  padding: 10px 12px;
  background: #f4f5f7;
  border-radius: 6px;
}
.reply-item {
  margin-bottom: 6px;
  font-size: 13px;
  line-height: 1.5;
  display: flex;
  align-items: flex-start;
  gap: 6px;
}
.reply-item:last-child {
  margin-bottom: 0;
}
.reply-user {
  color: #fb7299;
  font-weight: 500;
  white-space: nowrap;
}
.reply-content {
  color: #18191c;
  word-break: break-all;
}
.reply-content.deleted {
  color: #999;
  font-style: italic;
}
.expand-btn {
  margin-top: 6px;
  font-size: 13px;
  color: #00a1d6;
  cursor: pointer;
}
.expand-btn:hover {
  color: #00b5e5;
}
.reply-full {
  margin-top: 8px;
  padding: 10px 12px;
  background: #f4f5f7;
  border-radius: 6px;
}
.reply-full .reply-item {
  margin-bottom: 10px;
}
.reply-body {
  flex: 1;
  min-width: 0;
}
.reply-time {
  font-size: 12px;
  color: #999;
  margin-left: 6px;
}
.reply-full :deep(.el-pagination) {
  justify-content: center;
  margin-top: 8px;
}
</style>
