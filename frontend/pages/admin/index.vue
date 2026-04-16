<script setup lang="ts">
/**
 * 管理后台仪表盘
 */

import { 
  UserFilled, 
  QuestionFilled, 
  ShoppingCart,
  TrendCharts
} from '@element-plus/icons-vue'

definePageMeta({
  title: '管理后台',
  layout: 'admin',
  middleware: ['admin']
})

// 统计数据
const stats = [
  { title: '总用户数', value: '1,234', icon: UserFilled, color: 'bg-blue-500', trend: '+12%' },
  { title: '题目总数', value: '5,678', icon: QuestionFilled, color: 'bg-green-500', trend: '+5%' },
  { title: '今日订单', value: '89', icon: ShoppingCart, color: 'bg-purple-500', trend: '+23%' },
  { title: '日活用户', value: '456', icon: TrendCharts, color: 'bg-orange-500', trend: '+8%' },
]

// 最近注册用户
const recentUsers = [
  { username: '张三', email: 'zhangsan@example.com', date: '2024-01-15 10:30' },
  { username: '李四', email: 'lisi@example.com', date: '2024-01-15 09:15' },
  { username: '王五', email: 'wangwu@example.com', date: '2024-01-14 18:45' },
]

// 最近订单
const recentOrders = [
  { id: 'ORD-001', user: '张三', plan: '基础版', amount: 29, status: 'success' },
  { id: 'ORD-002', user: '李四', plan: '高级版', amount: 99, status: 'success' },
  { id: 'ORD-003', user: '王五', plan: '基础版', amount: 29, status: 'pending' },
]
</script>

<template>
  <div>
    <h1 class="text-2xl font-bold text-secondary-900 mb-6">仪表盘</h1>

    <!-- 统计数据 -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
      <div
        v-for="stat in stats"
        :key="stat.title"
        class="bg-white p-6 rounded-xl border border-secondary-200"
      >
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-secondary-500">{{ stat.title }}</p>
            <p class="text-2xl font-bold text-secondary-900 mt-1">{{ stat.value }}</p>
          </div>
          <div :class="`w-12 h-12 ${stat.color} rounded-lg flex items-center justify-center`">
            <el-icon :size="24" class="text-white">
              <component :is="stat.icon" />
            </el-icon>
          </div>
        </div>
        <div class="mt-4 flex items-center text-sm">
          <span class="text-green-500 font-medium">{{ stat.trend }}</span>
          <span class="text-secondary-400 ml-2">较上周</span>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- 最近注册用户 -->
      <div class="bg-white rounded-xl border border-secondary-200 p-6">
        <h2 class="text-lg font-semibold text-secondary-900 mb-4">最近注册用户</h2>
        <div class="space-y-3">
          <div
            v-for="user in recentUsers"
            :key="user.email"
            class="flex items-center justify-between py-3 border-b border-secondary-100 last:border-0"
          >
            <div class="flex items-center gap-3">
              <el-avatar :size="32">{{ user.username[0] }}</el-avatar>
              <div>
                <p class="text-sm font-medium text-secondary-900">{{ user.username }}</p>
                <p class="text-xs text-secondary-500">{{ user.email }}</p>
              </div>
            </div>
            <span class="text-xs text-secondary-400">{{ user.date }}</span>
          </div>
        </div>
      </div>

      <!-- 最近订单 -->
      <div class="bg-white rounded-xl border border-secondary-200 p-6">
        <h2 class="text-lg font-semibold text-secondary-900 mb-4">最近订单</h2>
        <div class="space-y-3">
          <div
            v-for="order in recentOrders"
            :key="order.id"
            class="flex items-center justify-between py-3 border-b border-secondary-100 last:border-0"
          >
            <div>
              <p class="text-sm font-medium text-secondary-900">{{ order.id }}</p>
              <p class="text-xs text-secondary-500">{{ order.user }} · {{ order.plan }}</p>
            </div>
            <div class="text-right">
              <p class="text-sm font-medium text-secondary-900">¥{{ order.amount }}</p>
              <el-tag :type="order.status === 'success' ? 'success' : 'warning'" size="small">
                {{ order.status === 'success' ? '已完成' : '处理中' }}
              </el-tag>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
