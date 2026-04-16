<script setup lang="ts">
/**
 * 用户仪表盘
 */

import { 
  EditPen, 
  VideoCamera, 
  Calendar,
  TrendCharts,
  Timer,
  Trophy
} from '@element-plus/icons-vue'

// 页面元数据
definePageMeta({
  title: '个人中心',
  layout: 'default',
  middleware: ['auth']
})

const appStore = useAppStore()
const userStore = useUserStore()

// 学习统计数据
const stats = [
  { title: '刷题总数', value: 128, icon: EditPen, color: 'bg-blue-500' },
  { title: '正确率', value: '78%', icon: TrendCharts, color: 'bg-green-500' },
  { title: '模拟面试', value: 5, icon: VideoCamera, color: 'bg-purple-500' },
  { title: '学习天数', value: 15, icon: Calendar, color: 'bg-orange-500' },
]

// 最近活动
const recentActivities = [
  { type: '刷题', content: '完成了 Go 并发编程 10 道题目', time: '2小时前' },
  { type: '面试', content: '完成了一次模拟面试', time: '昨天' },
  { type: '学习', content: '制定了新的学习计划', time: '3天前' },
]

// 待办事项
const todos = [
  { title: '完成今日刷题目标', completed: false },
  { title: '预约模拟面试', completed: true },
  { title: '复习错题本', completed: false },
]

onMounted(() => {
  appStore.setPageTitle('个人中心')
  userStore.fetchProfile()
})
</script>

<template>
  <div>
    <!-- 欢迎语 -->
    <div class="mb-8">
      <h1 class="text-2xl font-bold text-secondary-900">
        欢迎回来，{{ userStore.profile?.username || '用户' }}！
      </h1>
      <p class="text-secondary-600 mt-1">
        今天是学习的好日子，继续加油！
      </p>
    </div>

    <!-- 统计数据 -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
      <div
        v-for="stat in stats"
        :key="stat.title"
        class="bg-white p-6 rounded-xl border border-secondary-200"
      >
        <div class="flex items-center gap-4">
          <div :class="`w-12 h-12 ${stat.color} rounded-lg flex items-center justify-center`">
            <el-icon :size="24" class="text-white">
              <component :is="stat.icon" />
            </el-icon>
          </div>
          <div>
            <p class="text-sm text-secondary-500">{{ stat.title }}</p>
            <p class="text-2xl font-bold text-secondary-900">{{ stat.value }}</p>
          </div>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- 最近活动 -->
      <div class="lg:col-span-2 bg-white rounded-xl border border-secondary-200 p-6">
        <h2 class="text-lg font-semibold text-secondary-900 mb-4">最近活动</h2>
        <div class="space-y-4">
          <div
            v-for="(activity, index) in recentActivities"
            :key="index"
            class="flex items-start gap-4 pb-4 border-b border-secondary-100 last:border-0 last:pb-0"
          >
            <div class="w-2 h-2 mt-2 rounded-full bg-primary-500" />
            <div class="flex-1">
              <span class="inline-block px-2 py-0.5 bg-primary-100 text-primary-700 text-xs rounded-full mb-1">
                {{ activity.type }}
              </span>
              <p class="text-secondary-900">{{ activity.content }}</p>
              <p class="text-sm text-secondary-500">{{ activity.time }}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- 待办事项 -->
      <div class="bg-white rounded-xl border border-secondary-200 p-6">
        <h2 class="text-lg font-semibold text-secondary-900 mb-4">今日待办</h2>
        <div class="space-y-3">
          <label
            v-for="(todo, index) in todos"
            :key="index"
            class="flex items-center gap-3 cursor-pointer group"
          >
            <input
              v-model="todo.completed"
              type="checkbox"
              class="w-5 h-5 rounded border-secondary-300 text-primary-600 focus:ring-primary-500"
            >
            <span
              class="text-sm"
              :class="todo.completed ? 'text-secondary-400 line-through' : 'text-secondary-700'"
            >
              {{ todo.title }}
            </span>
          </label>
        </div>
        
        <!-- 快捷入口 -->
        <div class="mt-6 pt-6 border-t border-secondary-100">
          <h3 class="text-sm font-medium text-secondary-500 mb-3">快捷入口</h3>
          <div class="grid grid-cols-2 gap-2">
            <NuxtLink
              to="/practice"
              class="flex items-center justify-center gap-2 p-3 bg-primary-50 text-primary-700 rounded-lg hover:bg-primary-100 transition-colors text-sm"
            >
              <el-icon><EditPen /></el-icon>
              去刷题
            </NuxtLink>
            <NuxtLink
              to="/interview"
              class="flex items-center justify-center gap-2 p-3 bg-purple-50 text-purple-700 rounded-lg hover:bg-purple-100 transition-colors text-sm"
            >
              <el-icon><VideoCamera /></el-icon>
              模拟面试
            </NuxtLink>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
