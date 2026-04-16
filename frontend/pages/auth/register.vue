<script setup lang="ts">
/**
 * 注册页面
 */

import { User, Message, Lock } from '@element-plus/icons-vue'

// 页面元数据
definePageMeta({
  title: '注册',
  layout: 'auth',
})

const authStore = useAuthStore()
const router = useRouter()

// 表单数据
const form = reactive({
  username: '',
  email: '',
  password: '',
  confirmPassword: '',
  agreeTerms: false
})

// 表单引用
const formRef = ref()

// 自定义密码确认验证
const validateConfirmPassword = (rule: any, value: string, callback: Function) => {
  if (value !== form.password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

// 表单验证规则
const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 2, max: 20, message: '用户名长度2-20位', trigger: 'blur' }
  ],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '邮箱格式不正确', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ],
  agreeTerms: [
    { 
      validator: (rule: any, value: boolean, callback: Function) => {
        if (!value) {
          callback(new Error('请同意服务条款'))
        } else {
          callback()
        }
      },
      trigger: 'change'
    }
  ]
}

// 注册处理
const handleRegister = async () => {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  const success = await authStore.register(form.username, form.email, form.password)
  if (success) {
    router.push('/dashboard')
  }
}
</script>

<template>
  <div>
    <h2 class="text-2xl font-bold text-secondary-900 mb-2">创建账号</h2>
    <p class="text-secondary-600 mb-6">开始你的面试准备之旅</p>
    
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      @keyup.enter="handleRegister"
    >
      <el-form-item prop="username">
        <el-input
          v-model="form.username"
          placeholder="请输入用户名"
          size="large"
          :prefix-icon="User"
        />
      </el-form-item>
      
      <el-form-item prop="email">
        <el-input
          v-model="form.email"
          placeholder="请输入邮箱"
          size="large"
          :prefix-icon="Message"
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
      
      <el-form-item prop="confirmPassword">
        <el-input
          v-model="form.confirmPassword"
          type="password"
          placeholder="请确认密码"
          size="large"
          :prefix-icon="Lock"
          show-password
        />
      </el-form-item>
      
      <el-form-item prop="agreeTerms">
        <el-checkbox v-model="form.agreeTerms">
          我已阅读并同意
          <NuxtLink to="/terms" class="text-primary-600 hover:text-primary-700">服务条款</NuxtLink>
          和
          <NuxtLink to="/privacy" class="text-primary-600 hover:text-primary-700">隐私政策</NuxtLink>
        </el-checkbox>
      </el-form-item>
      
      <el-button
        type="primary"
        size="large"
        class="w-full"
        :loading="authStore.loading"
        @click="handleRegister"
      >
        注册
      </el-button>
    </el-form>
    
    <div class="mt-6 text-center text-sm text-secondary-600">
      已有账号？
      <NuxtLink to="/auth/login" class="text-primary-600 hover:text-primary-700 font-medium">
        立即登录
      </NuxtLink>
    </div>
  </div>
</template>
