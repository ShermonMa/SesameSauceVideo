<!--
  CategorySidebar.vue
  功能：首页左侧分类关键词侧边栏，拉取全部视频分类并以列表展示，支持点击切换分类筛选
  时间戳：2026-06-22
-->
<script setup>
import { ref, computed, onMounted } from 'vue'
import { listCategories } from '../api/video.js'

const props = defineProps({
  /** 当前选中的分类 ID（0 表示"全部"） */
  modelValue: { type: [Number, String], default: 0 }
})

const emit = defineEmits(['update:modelValue'])

const categories = ref([])
const loading = ref(false)

/** 当前高亮分类 ID，由父组件 modelValue 驱动 */
const activeId = computed(() => {
  const id = Number(props.modelValue)
  return id > 0 ? id : 0
})

/**
 * 点击分类项：通知父组件切换筛选，点"全部"传 0
 */
function selectCategory(cat) {
  emit('update:modelValue', cat ? cat.id : 0)
}

/**
 * 拉取全部分类列表
 */
async function fetchCategories() {
  loading.value = true
  try {
    const res = await listCategories()
    categories.value = res?.data || []
  } catch {
    categories.value = []
  } finally {
    loading.value = false
  }
}

onMounted(fetchCategories)
</script>

<template>
  <aside class="category-sidebar">
    <div class="sidebar-title">视频分类</div>

    <ul class="category-list" v-loading="loading">
      <!-- 全部分类入口 -->
      <li
        class="category-item all"
        :class="{ active: activeId === 0 }"
        @click="selectCategory(null)"
      >
        <span class="cat-icon">🏠</span>
        <span class="cat-name">全部</span>
      </li>

      <li
        v-for="cat in categories"
        :key="cat.id"
        class="category-item"
        :class="{ active: activeId === cat.id }"
        @click="selectCategory(cat)"
      >
        <span class="cat-icon">📂</span>
        <span class="cat-name">{{ cat.name }}</span>
      </li>
    </ul>

    <el-empty
      v-if="!loading && categories.length === 0"
      description="暂无分类"
      :image-size="60"
    />
  </aside>
</template>

<style scoped>
.category-sidebar {
  width: 180px;
  flex-shrink: 0;
  background: #fff;
  border-radius: 12px;
  padding: 16px 0;
  position: sticky;
  top: 84px;  /* 导航栏 72px + 间距 12px */
  align-self: flex-start;
  max-height: calc(100vh - 100px);
  overflow-y: auto;
}

.sidebar-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--bili-text, #18191c);
  padding: 0 16px 12px;
  border-bottom: 1px solid #f0f0f0;
  margin-bottom: 8px;
}

.category-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.category-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  cursor: pointer;
  transition: all 0.2s ease;
  font-size: 14px;
  color: var(--bili-text, #18191c);
  border-left: 3px solid transparent;
}

.category-item:hover {
  background: #f6f7f8;
  color: var(--bili-pink, #fb7299);
}

.category-item.active {
  background: linear-gradient(to right, rgba(251, 114, 153, 0.08), transparent);
  color: var(--bili-pink, #fb7299);
  font-weight: 600;
  border-left-color: var(--bili-pink, #fb7299);
}

.category-item.all .cat-icon {
  font-size: 15px;
}

.cat-icon {
  font-size: 14px;
  flex-shrink: 0;
}

.cat-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 移动端隐藏侧边栏 */
@media (max-width: 768px) {
  .category-sidebar {
    display: none;
  }
}
</style>
