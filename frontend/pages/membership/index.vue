<script setup lang="ts">
import { Check, Close, Medal } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

definePageMeta({
  title: '会员中心',
  layout: 'default',
  middleware: ['auth'],
})

type MembershipStatus = {
  level: string
  expires_at?: string
  is_active: boolean
  daily_practice_limit: number
  daily_interview_limit: number
  practice_used_today: number
  interview_used_today: number
}

const appStore = useAppStore()
const { $api } = useNuxtApp()

const loading = ref(true)
const memberStatus = ref<MembershipStatus | null>(null)
const orders = ref<any[]>([])
const subscribing = ref('')

const features = [
  { name: '每日刷题', free: '20题', pro: '无限制' },
  { name: '模拟面试', free: '2次/天', pro: '无限制' },
  { name: 'AI 解析', free: '基础', pro: '深度' },
  { name: '学习计划', free: '基础', pro: '个性化' },
  { name: '专属客服', free: false, pro: true },
]

const plans = [
  { type: 'monthly', name: '月度会员', price: 29.9, originalPrice: 49.9, daily: '1.0', period: '月' },
  { type: 'quarterly', name: '季度会员', price: 69.9, originalPrice: 149.7, daily: '0.8', period: '季', popular: true },
  { type: 'yearly', name: '年度会员', price: 199.9, originalPrice: 598.8, daily: '0.5', period: '年' },
]

const loadData = async () => {
  loading.value = true
  try {
    const [statusRes, ordersRes] = await Promise.all([
      $api.get<MembershipStatus>('/membership/status').catch(() => null),
      $api.get<any>('/membership/orders').catch(() => null),
    ])

    if (statusRes?.code === 200) memberStatus.value = statusRes.data
    if (ordersRes?.code === 200) orders.value = ordersRes.data?.list || []
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const isPro = computed(() => memberStatus.value?.level === 'pro' && memberStatus.value?.is_active)
const expireDate = computed(() => {
  if (!memberStatus.value?.expires_at) return '-'
  return new Date(memberStatus.value.expires_at).toLocaleDateString('zh-CN')
})

const displayLimit = (limit: number) => (limit < 0 ? '∞' : String(limit))

const subscribe = async (planType: string) => {
  subscribing.value = planType
  try {
    const res = await $api.post<any>('/membership/orders', { plan_type: planType })
    if (res.code === 200 && res.data) {
      const orderNo = res.data.order_no

      await ElMessageBox.confirm('是否确认支付？（模拟支付）', '支付确认', {
        type: 'info',
        confirmButtonText: '确认支付',
      })

      const callbackRes = await $api.post<any>('/membership/callback', { order_no: orderNo })
      if (callbackRes.code === 200) {
        ElMessage.success('订阅成功，会员已开通')
        await loadData()
      }
    }
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e?.data?.message || '订阅失败')
    }
  } finally {
    subscribing.value = ''
  }
}

const formatDate = (value: string) => {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN')
}

const orderStatus = (status: string) => {
  const map: Record<string, { label: string; type: string }> = {
    pending: { label: '待支付', type: 'warning' },
    paid: { label: '已支付', type: 'success' },
    cancelled: { label: '已取消', type: 'info' },
    refunded: { label: '已退款', type: 'danger' },
  }
  return map[status] || { label: status, type: 'info' }
}

onMounted(() => {
  appStore.setPageTitle('会员中心')
  loadData()
})
</script>

<template>
  <div v-loading="loading">
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900">会员中心</h1>
      <p class="text-gray-500 mt-1">升级会员，解锁全部 AI 功能</p>
    </div>

    <div class="mb-6 rounded-lg shadow-sm p-6 relative overflow-hidden" :class="isPro ? 'bg-gradient-to-r from-amber-50 via-yellow-50 to-orange-50 border border-amber-200' : 'bg-white border border-gray-200'">
      <div v-if="isPro" class="absolute top-0 right-0 bg-gradient-to-l from-amber-400 to-yellow-400 text-white text-xs px-4 py-1 rounded-bl-lg font-medium">
        PRO
      </div>
      <div class="flex items-center gap-6">
        <div class="w-16 h-16 rounded-full flex items-center justify-center" :class="isPro ? 'bg-gradient-to-br from-amber-400 to-orange-400' : 'bg-gray-100'">
          <el-icon :size="28" :class="isPro ? 'text-white' : 'text-gray-400'"><Medal /></el-icon>
        </div>
        <div class="flex-1">
          <h3 class="font-bold text-lg" :class="isPro ? 'text-amber-800' : 'text-gray-900'">
            {{ isPro ? '专业版会员' : '免费版' }}
          </h3>
          <p v-if="isPro" class="text-sm text-amber-600 mt-1">到期时间: {{ expireDate }}</p>
          <p v-else class="text-sm text-gray-400 mt-1">升级专业版获取完整体验</p>
        </div>
        <div v-if="memberStatus" class="text-right text-sm text-gray-500 space-y-1">
          <p>今日刷题: {{ memberStatus.practice_used_today }} / {{ displayLimit(memberStatus.daily_practice_limit) }}</p>
          <p>今日面试: {{ memberStatus.interview_used_today }} / {{ displayLimit(memberStatus.daily_interview_limit) }}</p>
        </div>
      </div>
    </div>

    <div class="bg-white rounded-lg shadow-sm p-6 mb-6">
      <h2 class="text-lg font-semibold text-gray-800 mb-4">功能对比</h2>
      <div class="overflow-hidden rounded-lg border border-gray-200">
        <table class="w-full">
          <thead>
            <tr class="bg-gray-50">
              <th class="text-left px-6 py-3 text-sm font-medium text-gray-500">功能</th>
              <th class="text-center px-6 py-3 text-sm font-medium text-gray-500">免费版</th>
              <th class="text-center px-6 py-3 text-sm font-medium text-amber-600 bg-amber-50/50">专业版</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="feature in features" :key="feature.name" class="border-t border-gray-100">
              <td class="px-6 py-3 text-sm text-gray-700">{{ feature.name }}</td>
              <td class="px-6 py-3 text-center text-sm">
                <span v-if="typeof feature.free === 'boolean'">
                  <el-icon v-if="feature.free" class="text-green-500"><Check /></el-icon>
                  <el-icon v-else class="text-gray-300"><Close /></el-icon>
                </span>
                <span v-else class="text-gray-500">{{ feature.free }}</span>
              </td>
              <td class="px-6 py-3 text-center text-sm bg-amber-50/30">
                <span v-if="typeof feature.pro === 'boolean'">
                  <el-icon v-if="feature.pro" class="text-green-500"><Check /></el-icon>
                  <el-icon v-else class="text-gray-300"><Close /></el-icon>
                </span>
                <span v-else class="text-amber-700 font-medium">{{ feature.pro }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-6">
      <div
        v-for="plan in plans"
        :key="plan.type"
        class="relative bg-white rounded-lg shadow-sm border-2 p-6 flex flex-col transition-transform hover:-translate-y-1"
        :class="plan.popular ? 'border-amber-400 shadow-amber-100' : 'border-gray-200'"
      >
        <div v-if="plan.popular" class="absolute -top-3 left-1/2 -translate-x-1/2 bg-gradient-to-r from-amber-400 to-orange-400 text-white text-xs font-bold px-4 py-1 rounded-full shadow">
          最受欢迎
        </div>

        <h3 class="text-lg font-semibold text-gray-800 mb-2">{{ plan.name }}</h3>
        <div class="mb-1">
          <span class="text-sm text-gray-400 line-through">¥{{ plan.originalPrice }}</span>
        </div>
        <div class="mb-1">
          <span class="text-3xl font-bold" :class="plan.popular ? 'text-amber-600' : 'text-gray-900'">¥{{ plan.price }}</span>
          <span class="text-gray-400 text-sm"> / {{ plan.period }}</span>
        </div>
        <p class="text-xs text-gray-400 mb-6">约 ¥{{ plan.daily }} / 天</p>

        <div class="flex-1" />

        <el-button :type="plan.popular ? 'warning' : 'primary'" size="large" class="w-full" :loading="subscribing === plan.type" @click="subscribe(plan.type)">
          立即订阅
        </el-button>
      </div>
    </div>

    <div class="bg-white rounded-lg shadow-sm p-6">
      <h2 class="text-lg font-semibold text-gray-800 mb-4">订单记录</h2>
      <el-table :data="orders" stripe empty-text="暂无订单记录" style="width: 100%">
        <el-table-column label="订单号" prop="order_no" min-width="180">
          <template #default="{ row }">
            <span class="text-sm font-mono">{{ row.order_no || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="方案" width="120">
          <template #default="{ row }">{{ row.plan_type || '-' }}</template>
        </el-table-column>
        <el-table-column label="金额" width="100" align="right">
          <template #default="{ row }">
            <span class="font-medium">¥{{ row.amount || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="orderStatus(row.status).type as any" size="small">{{ orderStatus(row.status).label }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="时间" min-width="160">
          <template #default="{ row }">
            <span class="text-sm text-gray-500">{{ formatDate(row.created_at) }}</span>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>
