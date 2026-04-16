<script setup lang="ts">
/**
 * 登录页面
 */

import { User, Lock } from '@element-plus/icons-vue'

// 页面元数据
definePageMeta({
  title: '登录',
  layout: 'auth',
})

const authStore = useAuthStore()
const router = useRouter()

// 表单数据
const form = reactive({
  email: '',
  password: '',
  remember: false
})

// 表单引用
const formRef = ref()

// 表单验证规则
const rules = {
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '邮箱格式不正确', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' }
  ]
}

// 登录处理
const handleLogin = async () => {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  const success = await authStore.login(form.email, form.password)
  if (success) {
    router.push('/dashboard')
  }
}
</script>

<template>
  <div>
    <h2 class="text-2xl font-bold text-secondary-900 mb-2">欢迎回来</h2>
    <p class="text-secondary-600 mb-6">登录你的 MakeJob 账号</p>
    
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      @keyup.enter="handleLogin"
    >
      <el-form-item prop="email">
        <el-input
          v-model="form.email"
          placeholder="请输入邮箱"
          size="large"
          :prefix-icon="User"
        />
      </el-form-item>
      
      <el-form-item prop="password">
        <el-input
          v-model="form.password"
          type="password"
          placeholder="请输入密码"
          size="large"
          :prefix-icon="Lock"
          show-password
        />
      </el-form-item>
      
      <div class="flex items-center justify-between mb-6">
        <el-checkbox v-model="form.remember">记住我</el-checkbox>
        <NuxtLink to="/auth/forgot-password" class="text-primary-600 hover:text-primary-700 text-sm">
          忘记密码？
        </NuxtLink>
      </div>
      
      <el-button
        type="primary"
        size="large"
        class="w-full"
        :loading="authStore.loading"
        @click="handleLogin"
      >
        登录
      </el-button>
    </el-form>
    
    <div class="mt-6 text-center text-sm text-secondary-600">
      还没有账号？
      <NuxtLink to="/auth/register" class="text-primary-600 hover:text-primary-700 font-medium">
        立即注册
      </NuxtLink>
    </div>
  </div>
</template>
