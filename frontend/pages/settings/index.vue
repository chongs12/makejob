<script setup lang="ts">
/**
 * 个人设置页
 * 头像 + 基本信息 + 修改密码 + 通知偏好
 */

import { User, Lock, Bell, Edit } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

definePageMeta({
  title: '个人设置',
  layout: 'default',
  middleware: ['auth'],
})

const appStore = useAppStore()
const authStore = useAuthStore()
const userStore = useUserStore()

const loading = ref(false)
const saving = ref(false)
const changingPwd = ref(false)
const showPwdSection = ref(false)

// 基本信息表单
const profileForm = ref({
  username: '',
  email: '',
  createdAt: '',
})

// 密码表单
const pwdForm = ref({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

// 通知偏好
const notifications = ref({
  studyReminder: true,
  interviewReminder: true,
  emailNotification: false,
})

// 头像首字母
const avatarInitial = computed(() => {
  const name = profileForm.value.username || authStore.username || '?'
  return name.charAt(0).toUpperCase()
})

const avatarBg = computed(() => {
  const colors = ['from-blue-400 to-indigo-500', 'from-green-400 to-teal-500', 'from-purple-400 to-pink-500', 'from-amber-400 to-orange-500']
  const name = profileForm.value.username || ''
  const idx = name.charCodeAt(0) % colors.length
  return colors[idx] || colors[0]
})

const formatDate = (d: string) => {
  if (!d) return '-'
  return new Date(d).toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' })
}

// 加载用户信息
const loadProfile = async () => {
  loading.value = true
  try {
    await userStore.fetchProfile()
    if (userStore.profile) {
      profileForm.value.username = userStore.profile.username
      profileForm.value.email = userStore.profile.email
      profileForm.value.createdAt = userStore.profile.createdAt
    } else {
      // fallback to auth store
      profileForm.value.username = authStore.user?.username || ''
      profileForm.value.email = authStore.user?.email || ''
      profileForm.value.createdAt = authStore.user?.createdAt || ''
    }
  } catch (e) {
    // fallback
    profileForm.value.username = authStore.user?.username || ''
    profileForm.value.email = authStore.user?.email || ''
  } finally {
    loading.value = false
  }
}

// 保存基本信息
const saveProfile = async () => {
  if (!profileForm.value.username.trim()) {
    ElMessage.warning('用户名不能为空')
    return
  }
  saving.value = true
  try {
    const success = await userStore.updateProfile({ username: profileForm.value.username })
    if (success) {
      authStore.updateUserInfo({ username: profileForm.value.username })
    }
  } finally {
    saving.value = false
  }
}

// 修改密码
const changePassword = async () => {
  if (!pwdForm.value.oldPassword || !pwdForm.value.newPassword) {
    ElMessage.warning('请填写完整密码信息')
    return
  }
  if (pwdForm.value.newPassword !== pwdForm.value.confirmPassword) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  if (pwdForm.value.newPassword.length < 6) {
    ElMessage.warning('新密码至少6位')
    return
  }
  changingPwd.value = true
  try {
    const success = await userStore.changePassword(pwdForm.value.oldPassword, pwdForm.value.newPassword)
    if (success) {
      pwdForm.value = { oldPassword: '', newPassword: '', confirmPassword: '' }
      showPwdSection.value = false
    }
  } finally {
    changingPwd.value = false
  }
}

onMounted(() => {
  appStore.setPageTitle('个人设置')
  loadProfile()
})
</script>

<template>
  <div v-loading="loading" class="max-w-3xl mx-auto">
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900">个人设置</h1>
      <p class="text-gray-500 mt-1">管理你的账号和个人信息</p>
    </div>

    <!-- 头像区 -->
    <div class="bg-white rounded-lg shadow-sm p-6 mb-6">
      <div class="flex items-center gap-6">
        <div class="w-24 h-24 rounded-full bg-gradient-to-br flex items-center justify-center text-white text-3xl font-bold shadow-lg"
          :class="avatarBg">
          {{ avatarInitial }}
        </div>
        <div>
          <h3 class="text-lg font-semibold text-gray-900">{{ profileForm.username || '-' }}</h3>
          <p class="text-sm text-gray-500">{{ profileForm.email }}</p>
          <el-button size="small" class="mt-2" disabled>
            <el-icon class="mr-1"><Edit /></el-icon>
            更换头像
          </el-button>
        </div>
      </div>
    </div>

    <!-- 基本信息 -->
    <div class="bg-white rounded-lg shadow-sm p-6 mb-6">
      <h2 class="text-lg font-semibold text-gray-800 mb-4 flex items-center gap-2">
        <el-icon class="text-blue-500"><User /></el-icon>
        基本信息
      </h2>
      <el-form label-position="top" class="space-y-4">
        <el-form-item label="用户名">
          <el-input v-model="profileForm.username" size="large" placeholder="输入用户名" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="profileForm.email" size="large" disabled>
            <template #append>
              <el-tag type="info" size="small">不可修改</el-tag>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item label="注册时间">
          <el-input :model-value="formatDate(profileForm.createdAt)" size="large" disabled />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="large" :loading="saving" @click="saveProfile" class="!px-8">
            保存修改
          </el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 修改密码 -->
    <div class="bg-white rounded-lg shadow-sm p-6 mb-6">
      <div class="flex items-center justify-between cursor-pointer" @click="showPwdSection = !showPwdSection">
        <h2 class="text-lg font-semibold text-gray-800 flex items-center gap-2">
          <el-icon class="text-amber-500"><Lock /></el-icon>
          修改密码
        </h2>
        <el-icon class="transition-transform" :class="showPwdSection ? 'rotate-90' : ''">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 18l6-6-6-6" /></svg>
        </el-icon>
      </div>
      <transition name="el-zoom-in-top">
        <div v-if="showPwdSection" class="mt-4">
          <el-form label-position="top" class="space-y-4">
            <el-form-item label="旧密码">
              <el-input v-model="pwdForm.oldPassword" type="password" show-password size="large" placeholder="输入当前密码" />
            </el-form-item>
            <el-form-item label="新密码">
              <el-input v-model="pwdForm.newPassword" type="password" show-password size="large" placeholder="输入新密码(至少6位)" />
            </el-form-item>
            <el-form-item label="确认新密码">
              <el-input v-model="pwdForm.confirmPassword" type="password" show-password size="large" placeholder="再次输入新密码" />
            </el-form-item>
            <el-form-item>
              <el-button type="warning" size="large" :loading="changingPwd" @click="changePassword" class="!px-8">
                修改密码
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </transition>
    </div>

    <!-- 通知偏好 -->
    <div class="bg-white rounded-lg shadow-sm p-6 mb-6">
      <h2 class="text-lg font-semibold text-gray-800 mb-4 flex items-center gap-2">
        <el-icon class="text-green-500"><Bell /></el-icon>
        通知偏好
      </h2>
      <div class="space-y-4">
        <div class="flex items-center justify-between py-2">
          <div>
            <p class="text-sm font-medium text-gray-700">学习提醒</p>
            <p class="text-xs text-gray-400">每日学习任务提醒通知</p>
          </div>
          <el-switch v-model="notifications.studyReminder" />
        </div>
        <div class="border-t border-gray-100" />
        <div class="flex items-center justify-between py-2">
          <div>
            <p class="text-sm font-medium text-gray-700">面试提醒</p>
            <p class="text-xs text-gray-400">模拟面试相关提醒</p>
          </div>
          <el-switch v-model="notifications.interviewReminder" />
        </div>
        <div class="border-t border-gray-100" />
        <div class="flex items-center justify-between py-2">
          <div>
            <p class="text-sm font-medium text-gray-700">邮件通知</p>
            <p class="text-xs text-gray-400">接收邮件推送通知</p>
          </div>
          <el-switch v-model="notifications.emailNotification" />
        </div>
      </div>
    </div>
  </div>
</template>
