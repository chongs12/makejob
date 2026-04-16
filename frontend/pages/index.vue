<script setup lang="ts">
/**
 * 首页
 * 产品展示与功能入口
 */

import { EditPen, VideoCamera, Calendar, MagicStick, ArrowRight } from '@element-plus/icons-vue'

// 页面元数据
definePageMeta({
  title: '首页',
  layout: 'default',
})

const appStore = useAppStore()

// 功能特性
const features = [
  {
    icon: EditPen,
    title: '智能刷题',
    description: 'AI驱动的个性化题库，根据你的薄弱环节智能推荐题目',
    color: 'bg-blue-500',
    path: '/practice'
  },
  {
    icon: VideoCamera,
    title: '模拟面试',
    description: '与AI面试官进行真实场景模拟，提升面试表现',
    color: 'bg-purple-500',
    path: '/interview'
  },
  {
    icon: Calendar,
    title: '学习计划',
    description: '定制专属学习路径，科学规划备考时间',
    color: 'bg-green-500',
    path: '/plan'
  },
  {
    icon: MagicStick,
    title: 'AI陪伴',
    description: 'Live2D虚拟助手全程陪伴，让学习不再孤单',
    color: 'bg-pink-500',
    path: '/companion'
  }
]

// 支持的行业
const industries = [
  { name: 'Go后端开发', icon: '🔵', count: 1200 },
  { name: 'Java开发', icon: '☕', count: 2500 },
  { name: '前端开发', icon: '🌐', count: 1800 },
  { name: 'Python开发', icon: '🐍', count: 1500 },
  { name: '产品经理', icon: '📱', count: 800 },
  { name: '数据分析', icon: '📊', count: 600 },
]

// 设置页面标题
onMounted(() => {
  appStore.setPageTitle('首页')
})
</script>

<template>
  <div>
    <!-- Hero Section -->
    <section class="py-16 lg:py-24">
      <div class="text-center max-w-4xl mx-auto px-4">
        <h1 class="text-4xl lg:text-6xl font-bold text-secondary-900 mb-6">
          AI驱动的
          <span class="text-transparent bg-clip-text bg-gradient-to-r from-primary-600 to-primary-400">
            面试刷题
          </span>
          平台
        </h1>
        <p class="text-xl text-secondary-600 mb-8 max-w-2xl mx-auto">
          智能题库、AI模拟面试、个性化学习计划，助你轻松拿到心仪Offer
        </p>
        <div class="flex flex-col sm:flex-row gap-4 justify-center">
          <NuxtLink 
            to="/practice"
            class="inline-flex items-center justify-center gap-2 px-8 py-4 bg-primary-600 text-white rounded-xl font-semibold hover:bg-primary-700 transition-colors"
          >
            开始刷题
            <el-icon><ArrowRight /></el-icon>
          </NuxtLink>
          <NuxtLink 
            to="/interview"
            class="inline-flex items-center justify-center gap-2 px-8 py-4 bg-white text-secondary-700 border border-secondary-300 rounded-xl font-semibold hover:bg-secondary-50 transition-colors"
          >
            模拟面试
          </NuxtLink>
        </div>
      </div>
    </section>

    <!-- 功能特性 -->
    <section class="py-16 bg-white">
      <div class="container-custom">
        <h2 class="text-3xl font-bold text-center text-secondary-900 mb-12">
          核心功能
        </h2>
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-8">
          <NuxtLink
            v-for="feature in features"
            :key="feature.title"
            :to="feature.path"
            class="group p-6 rounded-2xl border border-secondary-200 hover:border-primary-300 hover:shadow-lg transition-all"
          >
            <div :class="`w-14 h-14 ${feature.color} rounded-xl flex items-center justify-center mb-4 group-hover:scale-110 transition-transform`">
              <el-icon :size="28" class="text-white">
                <component :is="feature.icon" />
              </el-icon>
            </div>
            <h3 class="text-xl font-semibold text-secondary-900 mb-2">{{ feature.title }}</h3>
            <p class="text-secondary-600">{{ feature.description }}</p>
          </NuxtLink>
        </div>
      </div>
    </section>

    <!-- 支持的行业 -->
    <section class="py-16">
      <div class="container-custom">
        <h2 class="text-3xl font-bold text-center text-secondary-900 mb-4">
          覆盖多个热门行业
        </h2>
        <p class="text-center text-secondary-600 mb-12">
          精选各岗位高频面试题，持续更新中
        </p>
        <div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
          <div
            v-for="industry in industries"
            :key="industry.name"
            class="p-4 bg-white rounded-xl border border-secondary-200 text-center hover:border-primary-300 hover:shadow-md transition-all cursor-pointer"
          >
            <div class="text-3xl mb-2">{{ industry.icon }}</div>
            <h4 class="font-medium text-secondary-900">{{ industry.name }}</h4>
            <p class="text-sm text-secondary-500">{{ industry.count }}+ 题目</p>
          </div>
        </div>
      </div>
    </section>

    <!-- CTA Section -->
    <section class="py-16 bg-gradient-to-r from-primary-600 to-primary-700">
      <div class="container-custom text-center">
        <h2 class="text-3xl font-bold text-white mb-4">
          准备好开始了吗？
        </h2>
        <p class="text-primary-100 mb-8 max-w-2xl mx-auto">
          立即注册，开启你的面试准备之旅。免费体验基础功能，升级会员解锁全部AI功能。
        </p>
        <NuxtLink 
          to="/auth/register"
          class="inline-flex items-center gap-2 px-8 py-4 bg-white text-primary-600 rounded-xl font-semibold hover:bg-primary-50 transition-colors"
        >
          免费注册
          <el-icon><ArrowRight /></el-icon>
        </NuxtLink>
      </div>
    </section>
  </div>
</template>
