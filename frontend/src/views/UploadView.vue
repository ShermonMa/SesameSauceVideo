<script setup>
import { reactive, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { createVideo, confirmUpload, uploadToMinIO, uploadCoverToMinIO, listCategories } from '../api/video.js'

const router = useRouter()

const formRef = ref(null)
const fileInputRef = ref(null)
const coverInputRef = ref(null)
const file = ref(null)
const coverFile = ref(null)
const uploading = ref(false)
const videoProgress = ref(0)
const coverProgress = ref(0)
const categories = ref([])

const form = reactive({
  title: '',
  description: '',
  categoryIds: []
})

const rules = {
  title: [
    { required: true, message: '请输入标题', trigger: 'blur' },
    { max: 128, message: '标题最多 128 个字符', trigger: 'blur' }
  ],
  categoryIds: [
    { type: 'array', max: 3, message: '最多选择 3 个分类', trigger: 'change' }
  ]
}

/**
 * 校验并保存用户选择的视频文件
 * @param {File} rawFile - 原始文件对象
 */
const handleFileChange = (rawFile) => {
  if (!rawFile) return
  const allowedTypes = ['video/mp4', 'video/quicktime', 'video/webm']
  if (!allowedTypes.includes(rawFile.type)) {
    ElMessage.error('仅支持 mp4、mov、webm 格式')
    file.value = null
    return
  }
  const maxSizeMB = 500
  if (rawFile.size > maxSizeMB * 1024 * 1024) {
    ElMessage.error(`文件大小不能超过 ${maxSizeMB}MB`)
    file.value = null
    return
  }
  file.value = rawFile
}

/**
 * 校验并保存用户选择的封面图片
 * @param {File} rawFile - 原始文件对象
 */
const handleCoverChange = (rawFile) => {
  if (!rawFile) return
  const allowedTypes = ['image/jpeg', 'image/png']
  const allowedExts = ['.jpg', '.jpeg', '.png']
  const ext = rawFile.name.slice(rawFile.name.lastIndexOf('.')).toLowerCase()
  if (!allowedTypes.includes(rawFile.type) || !allowedExts.includes(ext)) {
    ElMessage.error('仅支持 jpg、jpeg、png 格式')
    coverFile.value = null
    return
  }
  const maxSizeMB = 5
  if (rawFile.size > maxSizeMB * 1024 * 1024) {
    ElMessage.error(`封面大小不能超过 ${maxSizeMB}MB`)
    coverFile.value = null
    return
  }
  coverFile.value = rawFile
}

const handleDrop = (e) => {
  e.preventDefault()
  const droppedFile = e.dataTransfer.files[0]
  if (droppedFile) handleFileChange(droppedFile)
}

const handleDragOver = (e) => {
  e.preventDefault()
}

const fetchCategories = async () => {
  try {
    const res = await listCategories()
    categories.value = res.data || []
  } catch (e) {
    categories.value = []
  }
}

onMounted(() => {
  fetchCategories()
})

/**
 * 提取文件扩展名（不含点）
 */
const getExt = (filename) => {
  if (!filename) return ''
  const idx = filename.lastIndexOf('.')
  return idx > 0 ? filename.slice(idx + 1).toLowerCase() : ''
}

/**
 * 执行视频上传主流程：创建记录 → 直传视频 → 直传封面 → 确认完成
 */
const handleUpload = async () => {
  if (!file.value) {
    ElMessage.error('请选择视频文件')
    return
  }
  if (!coverFile.value) {
    ElMessage.error('请上传封面图片')
    return
  }
  await formRef.value.validate()

  uploading.value = true
  videoProgress.value = 0
  coverProgress.value = 0

  try {
    const coverExt = getExt(coverFile.value.name)
    const createRes = await createVideo({
      title: form.title,
      description: form.description,
      category_ids: form.categoryIds,
      filename: file.value.name,
      cover_ext: coverExt
    })
    const { video_id, video_upload_url, cover_upload_url } = createRes.data

    await uploadToMinIO(video_upload_url, file.value, (percent) => {
      videoProgress.value = percent
    })

    if (cover_upload_url) {
      await uploadCoverToMinIO(cover_upload_url, coverFile.value, (percent) => {
        coverProgress.value = percent
      })
    }

    await confirmUpload(video_id, { cover_ext: coverExt })

    ElMessage.success('上传成功，视频处理中，请到"我的视频"查看进度')
    router.push('/my-videos')
  } catch (e) {
    ElMessage.error('上传失败，请重试')
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <div class="upload-container">
    <el-card class="upload-card">
      <h2>上传视频</h2>

      <div
        class="drop-zone"
        @drop="handleDrop"
        @dragover="handleDragOver"
        @click="fileInputRef.click()"
      >
        <input
          ref="fileInputRef"
          type="file"
          accept="video/mp4,video/quicktime,video/webm"
          style="display: none"
          @change="(e) => handleFileChange(e.target.files[0])"
        />
        <div v-if="!file">
          <p>点击选择文件或拖拽到此处</p>
          <p class="hint">支持 mp4、mov、webm，最大 500MB</p>
        </div>
        <div v-else>
          <p>{{ file.name }}</p>
          <p class="hint">{{ (file.size / 1024 / 1024).toFixed(2) }} MB</p>
        </div>
      </div>

      <div v-if="uploading" style="margin-top: 12px;">
        <div style="font-size: 12px; color: #666; margin-bottom: 4px;">视频上传 {{ videoProgress }}%</div>
        <el-progress :percentage="videoProgress" :show-text="false" />
      </div>

      <div
        class="drop-zone"
        style="margin-top: 16px; padding: 24px;"
        @click="coverInputRef.click()"
      >
        <input
          ref="coverInputRef"
          type="file"
          accept="image/jpeg,image/png"
          style="display: none"
          @change="(e) => handleCoverChange(e.target.files[0])"
        />
        <div v-if="!coverFile">
          <p>点击选择封面图片</p>
          <p class="hint">支持 jpg、jpeg、png，建议 16:9，最大 5MB</p>
        </div>
        <div v-else>
          <p>{{ coverFile.name }}</p>
          <p class="hint">{{ (coverFile.size / 1024 / 1024).toFixed(2) }} MB</p>
        </div>
      </div>

      <div v-if="uploading && coverFile" style="margin-top: 12px;">
        <div style="font-size: 12px; color: #666; margin-bottom: 4px;">封面上传 {{ coverProgress }}%</div>
        <el-progress :percentage="coverProgress" :show-text="false" />
      </div>

      <el-form ref="formRef" :model="form" :rules="rules" label-width="80px" style="margin-top: 20px;">
        <el-form-item label="标题" prop="title">
          <el-input v-model="form.title" placeholder="1-128 字符" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" rows="3" placeholder="视频描述（选填，最多500字符）" />
        </el-form-item>
        <el-form-item label="分类" prop="categoryIds">
          <el-select v-model="form.categoryIds" multiple placeholder="请选择分类（选填，最多3个）" style="width: 100%;">
            <el-option
              v-for="cat in categories"
              :key="cat.id"
              :label="cat.name"
              :value="cat.id"
            />
          </el-select>
        </el-form-item>
      </el-form>

      <div style="margin-top: 20px; text-align: right;">
        <el-button type="primary" :loading="uploading" @click="handleUpload">
          开始上传
        </el-button>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.upload-container {
  max-width: 720px;
  margin: 40px auto;
}
.drop-zone {
  border: 2px dashed #ccc;
  border-radius: 8px;
  padding: 40px;
  text-align: center;
  cursor: pointer;
  transition: border-color 0.3s;
}
.drop-zone:hover {
  border-color: #409eff;
}
.hint {
  color: #999;
  font-size: 12px;
}
</style>
