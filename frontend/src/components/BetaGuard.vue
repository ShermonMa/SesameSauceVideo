<!--
  BetaGuard.vue
  功能：全屏内测密钥校验遮罩层；校验通过后当日（以凌晨 04:00 为界）不再展示；校验成功后通知业务方刷新数据
  时间戳：2026-06-20
-->
<template>
  <Teleport to="body">
    <Transition name="beta-fade">
      <div v-if="!isVerified" class="beta-guard">
        <div class="beta-content">
          <img src="../../../texture/logo.png" alt="SesameSauce" class="beta-logo" />
          <p class="beta-subtitle">SesameSauce 内测版</p>

          <div class="beta-form">
            <el-input
              v-model="inputKey"
              type="password"
              placeholder="请输入内测密钥"
              size="large"
              class="beta-input"
              @keyup.enter="handleSubmit"
            />
            <p v-if="errorMsg" class="beta-error">{{ errorMsg }}</p>
            <el-button
              type="primary"
              size="large"
              class="beta-btn"
              :loading="loading"
              @click="handleSubmit"
            >
              进入内测
            </el-button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { verifyBetaKey, checkBetaToken, onBetaExpired, triggerBetaVerified } from '../api/beta.js'

const isVerified = ref(false)
const inputKey = ref('')
const errorMsg = ref('')
const loading = ref(false)

// 注册全局 Beta 过期回调：当 request.js 收到 1005 时重新展示遮罩
onBetaExpired(() => {
  isVerified.value = false
  inputKey.value = ''
  errorMsg.value = ''
  ElMessage.warning('内测权限已过期，请重新输入密钥')
})

/**
 * 判断本地 beta_token 是否仍有效（存在且未过期）
 * 与 isTokenValid 逻辑一致：exp 为秒级时间戳，允许 5s 时钟漂移
 */
function isBetaTokenValid() {
  const token = localStorage.getItem('beta_token')
  if (!token) return false
  try {
    const parts = token.split('.')
    if (parts.length !== 3) return false
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4)
    const payload = JSON.parse(atob(padded))
    return payload.exp * 1000 > Date.now() + 5000
  } catch {
    return false
  }
}

/**
 * 校验通过后的统一处理：存储 token、隐藏遮罩、清空状态、通知业务方刷新数据
 */
function markVerified(token) {
  localStorage.setItem('beta_token', token)
  isVerified.value = true
  inputKey.value = ''
  errorMsg.value = ''
  triggerBetaVerified()
}

/**
 * 提交密钥进行后端校验
 */
async function handleSubmit() {
  const key = inputKey.value.trim()
  if (!key) {
    errorMsg.value = '请输入密钥'
    return
  }
  if (key.length > 64) {
    errorMsg.value = '密钥长度不能超过 64 字符'
    return
  }

  loading.value = true
  errorMsg.value = ''

  try {
    const res = await verifyBetaKey(key)
    if (res.code === 0 && res.data?.beta_token) {
      markVerified(res.data.beta_token)
    } else {
      errorMsg.value = res.message || '校验失败，请重试'
    }
  } catch (err) {
    errorMsg.value = err.message || '网络错误，请重试'
  } finally {
    loading.value = false
  }
}

/**
 * 组件挂载时：若本地 token 未过期则尝试调接口二次确认；
 * 若本地已过期直接展示输入界面
 */
onMounted(async () => {
  if (!isBetaTokenValid()) {
    isVerified.value = false
    return
  }

  try {
    const res = await checkBetaToken()
    if (res.code === 0 && res.data?.valid) {
      isVerified.value = true
    } else {
      localStorage.removeItem('beta_token')
      isVerified.value = false
      if (res.code === 1005) {
        ElMessage.warning('内测权限已过期，请重新输入密钥')
      }
    }
  } catch {
    // 网络异常时，以本地 token 为准，允许通过（避免服务端短暂不可用时完全无法使用）
    isVerified.value = true
  }
})
</script>

<style scoped>
.beta-guard {
  position: fixed;
  inset: 0;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #0f0f0f;
}

.beta-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 360px;
  padding: 0 20px;
}

.beta-logo {
  height: 64px;
  margin-bottom: 12px;
  object-fit: contain;
}

.beta-subtitle {
  margin: 0 0 32px 0;
  font-size: 14px;
  color: #888;
  letter-spacing: 2px;
}

.beta-form {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.beta-input :deep(.el-input__wrapper) {
  background: #1a1a1a;
  box-shadow: 0 0 0 1px #333 inset;
}

.beta-input :deep(.el-input__inner) {
  color: #eee;
}

.beta-error {
  margin: 0;
  font-size: 13px;
  color: #f56c6c;
  text-align: center;
}

.beta-btn {
  width: 100%;
  font-size: 15px;
  letter-spacing: 2px;
}

/* 过渡动画 */
.beta-fade-enter-active,
.beta-fade-leave-active {
  transition: opacity 0.4s ease;
}

.beta-fade-enter-from,
.beta-fade-leave-to {
  opacity: 0;
}
</style>
