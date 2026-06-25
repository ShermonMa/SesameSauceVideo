<!--
  MessageCenterView.vue
  功能：消息中心页面；六 Tab 信箱（私信/回复/@/赞/关注/系统通知）列表展示与已读标记
  时间戳：2026-04-27
  2026-06-21 列表加载/全部已读后通过 inject 调用父布局的 refreshUnreadCount，同步刷新顶部未读气泡
-->
<script setup>
import { ref, computed, watch, inject } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  getMessageList,
  markRead,
  getSystemList,
  markSystemRead
} from '../api/message.js'

const activeTab = ref('private')
const loading = ref(false)
const messages = ref([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

// 从 MainLayout 注入刷新未读数方法，不存在时降级为空函数
const refreshUnreadCount = inject('refreshUnreadCount', () => {})

/**
 * Tab 配置映射：key -> { label, type, icon }
 * type 用于普通消息列表接口；系统通知为特殊逻辑
 */
const tabConfig = {
  private: { label: '我的消息', type: 1, icon: 'ChatDotRound' },
  reply:   { label: '回复我的', type: 5, icon: 'ChatLineRound' },
  mention: { label: '@我的',   type: 6, icon: 'At' },
  like:    { label: '收到的赞', type: 3, icon: 'Star' },
  follow:  { label: '新增关注', type: 2, icon: 'User' },
  system:  { label: '系统通知', type: 7, icon: 'Bell' }
}

const tabKeys = Object.keys(tabConfig)

/**
 * 当前选中 Tab 的配置对象
 */
const currentTab = computed(() => tabConfig[activeTab.value])

/**
 * 是否为系统通知 Tab（走独立接口）
 */
const isSystemTab = computed(() => activeTab.value === 'system')

/**
 * 是否为预留 Tab（@我的），固定空列表
 */
const isReservedTab = computed(() => activeTab.value === 'mention')

/**
 * 拉取当前 Tab 的消息列表
 */
const fetchMessages = async () => {
  if (isReservedTab.value) {
    messages.value = []
    total.value = 0
    return
  }

  loading.value = true
  try {
    if (isSystemTab.value) {
      const res = await getSystemList({ page: page.value, size: pageSize.value })
      messages.value = res.data.notifications || []
      total.value = res.data.total || 0
    } else {
      const res = await getMessageList({
        type: currentTab.value.type,
        page: page.value,
        size: pageSize.value
      })
      messages.value = res.data.messages || []
      total.value = res.data.total || 0
    }
    // 列表加载后主动刷新顶部未读气泡（后端已更新 last_msg_check）
    refreshUnreadCount()
  } catch (e) {
    ElMessage.error('获取消息列表失败')
  } finally {
    loading.value = false
  }
}

/**
 * 标记当前 Tab 全部已读
 */
const handleMarkAllRead = async () => {
  try {
    await ElMessageBox.confirm('确定将当前分类全部标记为已读？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'info'
    })

    if (isSystemTab.value) {
      await markSystemRead()
    } else if (!isReservedTab.value) {
      await markRead({ type: currentTab.value.type })
    }
    ElMessage.success('已标记为已读')
    await fetchMessages()
    // 全部已读后立即刷新顶部未读气泡
    refreshUnreadCount()
  } catch (e) {
    // 取消或报错静默处理
  }
}

/**
 * 分页切换
 */
const handlePageChange = (newPage) => {
  page.value = newPage
  fetchMessages()
}

/**
 * 格式化消息内容摘要，防止过长
 */
const formatContent = (content, maxLen = 120) => {
  if (!content) return ''
  return content.length > maxLen ? content.slice(0, maxLen) + '...' : content
}

watch(activeTab, () => {
  page.value = 1
  fetchMessages()
}, { immediate: true })
</script>

<template>
  <div class="message-center">
    <el-card>
      <template #header>
        <div class="card-header">
          <span class="title">消息中心</span>
          <el-button text type="primary" @click="handleMarkAllRead">
            全部已读
          </el-button>
        </div>
      </template>

      <el-tabs v-model="activeTab" type="border-card">
        <el-tab-pane
          v-for="key in tabKeys"
          :key="key"
          :name="key"
          :label="tabConfig[key].label"
        >
          <div v-loading="loading" class="message-list">
            <!-- 预留 Tab 占位提示 -->
            <el-empty
              v-if="isReservedTab"
              description="@功能即将上线，敬请期待"
            />

            <!-- 空列表 -->
            <el-empty
              v-else-if="!loading && messages.length === 0"
              description="暂无消息"
            />

            <!-- 消息列表 -->
            <div
              v-for="msg in messages"
              :key="msg.id"
              class="message-item"
              :class="{ unread: !msg.is_read && !isSystemTab }"
            >
              <!-- 系统通知样式（无头像） -->
              <template v-if="isSystemTab">
                <div class="system-item">
                  <h4 class="system-title">{{ msg.title }}</h4>
                  <p class="system-content">{{ msg.content }}</p>
                  <span class="msg-time">{{ msg.publish_time }}</span>
                </div>
              </template>

              <!-- 普通消息样式 -->
              <template v-else>
                <el-avatar :size="40" :src="msg.from_user_avatar || ''">
                  {{ msg.from_user_name ? msg.from_user_name[0] : '?' }}
                </el-avatar>
                <div class="msg-body">
                  <div class="msg-header">
                    <span class="from-user">{{ msg.from_user_name }}</span>
                    <span class="msg-time">{{ msg.created_at }}</span>
                  </div>
                  <p class="msg-content">
                    {{ formatContent(msg.content) }}
                  </p>
                </div>
              </template>
            </div>
          </div>

          <!-- 分页 -->
          <el-pagination
            v-if="!isReservedTab && total > pageSize"
            :current-page="page"
            :page-size="pageSize"
            :total="total"
            layout="prev, pager, next"
            class="pagination"
            @current-change="handlePageChange"
          />
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<style scoped>
.message-center {
  max-width: 960px;
  margin: 0 auto;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.title {
  font-size: 18px;
  font-weight: 600;
}
.message-list {
  min-height: 200px;
}
.message-item {
  display: flex;
  gap: 12px;
  padding: 14px 0;
  border-bottom: 1px solid #f1f1f1;
  transition: background 0.2s;
}
.message-item:last-child {
  border-bottom: none;
}
.message-item.unread {
  background: #f0f9ff;
  padding-left: 8px;
  padding-right: 8px;
  margin: 0 -8px;
  border-radius: 6px;
}
.msg-body {
  flex: 1;
  min-width: 0;
}
.msg-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}
.from-user {
  font-size: 14px;
  color: #fb7299;
  font-weight: 500;
}
.msg-time {
  font-size: 12px;
  color: #999;
}
.msg-content {
  margin: 0;
  font-size: 14px;
  color: #18191c;
  line-height: 1.5;
  word-break: break-all;
}
.system-item {
  flex: 1;
}
.system-title {
  margin: 0 0 6px 0;
  font-size: 15px;
  font-weight: 600;
  color: #18191c;
}
.system-content {
  margin: 0 0 6px 0;
  font-size: 14px;
  color: #333;
  line-height: 1.5;
  word-break: break-all;
}
.pagination {
  justify-content: center;
  margin-top: 16px;
}
</style>
