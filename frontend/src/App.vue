<script setup>
/**
 * App.vue
 * 功能：根组件，挂载全局路由视图与内测准入遮罩层
 * 时间戳：2026-05-29
 */
import BetaGuard from './components/BetaGuard.vue'
</script>

<template>
  <div>
    <BetaGuard />
    <router-view />
  </div>
</template>
