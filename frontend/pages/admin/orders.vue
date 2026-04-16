<script setup lang="ts">
/**
 * 订单管理页面
 */

import { ShoppingCart, View, TrendCharts, Money } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

definePageMeta({
  title: '订单管理',
  layout: 'admin',
  middleware: ['admin']
})

const api = useApi()
const loading = ref(false)
const orders = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const statusFilter = ref('')
const dateRange = ref<[string, string] | null>(null)

// 统计数据
const stats = ref({
  total_orders: 0,
  total_revenue: 0,
  month_orders: 0,
  month_revenue: 0,
})

// 状态选项
const statusOptions = [
  { label: '全部', value: '' },
  { label: '待支付', value: 'pending' },
  { label: '已支付', value: 'paid' },
  { label: '已取消', value: 'cancelled' },
]

const statusTagMap: Record<string, { type: string; label: string }> = {
  pending: { type: 'warning', label: '待支付' },
  paid: { type: 'success', label: '已支付' },
  cancelled: { type: 'info', label: '已取消' },
  refunded: { type: 'danger', label: '已退款' },
}

const planTagMap: Record<string, { type: string; label: string }> = {
  basic: { type: '', label: '基础版' },
  premium: { type: 'warning', label: '高级版' },
  enterprise: { type: 'danger', label: '企业版' },
}

// 统计卡片
const statCards = computed(() => [
  { title: '订单总数', value: stats.value.total_orders, icon: ShoppingCart, color: 'bg-blue-500' },
  { title: '总收入', value: `¥${stats.value.total_revenue.toLocaleString()}`, icon: Money, color: 'bg-green-500' },
  { title: '本月订单', value: stats.value.month_orders, icon: TrendCharts, color: 'bg-purple-500' },
  { title: '本月收入', value: `¥${stats.value.month_revenue.toLocaleString()}`, icon: Money, color: 'bg-orange-500' },
])

// 获取订单列表
const fetchOrders = async () => {
  loading.value = true
  try {
    const params: Record<string, any> = { page: page.value, page_size: pageSize.value }
    if (statusFilter.value) params.status = statusFilter.value
    if (dateRange.value) {
      params.start_date = dateRange.value[0]
      params.end_date = dateRange.value[1]
    }

    const res = await api.get<any>('/api/admin/orders', params)
    if (res.code === 0 || res.code === 200) {
      orders.value = res.data?.list || res.data || []
      total.value = res.data?.total || 0
    }
  } catch (error) {
    console.error('获取订单失败', error)
  } finally {
    loading.value = false
  }
}

// 获取统计数据
const fetchStats = async () => {
  try {
    const res = await api.get<any>('/api/admin/dashboard')
    if (res.code === 0 || res.code === 200) {
      const data = res.data || {}
      stats.value = {
        total_orders: data.total_orders || 0,
        total_revenue: data.total_revenue || 0,
        month_orders: data.month_orders || 0,
        month_revenue: data.month_revenue || 0,
      }
    }
  } catch (error) {
    console.error('获取统计数据失败', error)
  }
}

// 查看详情
const detailDialogVisible = ref(false)
const detailOrder = ref<any>(null)

const viewDetail = (row: any) => {
  detailOrder.value = row
  detailDialogVisible.value = true
}

const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN')
}

const handleSearch = () => {
  page.value = 1
  fetchOrders()
}

const handlePageChange = (newPage: number) => {
  page.value = newPage
  fetchOrders()
}

onMounted(() => {
  fetchOrders()
  fetchStats()
})
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold text-secondary-900">订单管理</h1>

    <!-- 统计卡片 -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      <div
        v-for="stat in statCards"
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
      </div>
    </div>

    <!-- 筛选工具栏 -->
    <div class="bg-white rounded-lg shadow-sm border border-secondary-200 p-4">
      <div class="flex items-center gap-4">
        <el-select v-model="statusFilter" placeholder="订单状态" class="w-36" @change="handleSearch">
          <el-option v-for="opt in statusOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          @change="handleSearch"
        />
      </div>
    </div>

    <!-- 订单表格 -->
    <div class="bg-white rounded-lg shadow-sm border border-secondary-200">
      <el-table :data="orders" v-loading="loading" style="width: 100%">
        <el-table-column prop="order_no" label="订单号" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="font-mono text-sm">{{ row.order_no || row.id }}</span>
          </template>
        </el-table-column>
        <el-table-column label="用户" min-width="140">
          <template #default="{ row }">
            <span class="text-sm">{{ row.username || row.user_id }}</span>
          </template>
        </el-table-column>
        <el-table-column label="方案" width="120">
          <template #default="{ row }">
            <el-tag :type="(planTagMap[row.plan]?.type as any) || 'info'" size="small">
              {{ planTagMap[row.plan]?.label || row.plan || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="金额" width="100" align="right">
          <template #default="{ row }">
            <span class="font-medium text-secondary-900">¥{{ row.amount || 0 }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="(statusTagMap[row.status]?.type as any) || 'info'" size="small">
              {{ statusTagMap[row.status]?.label || row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">
            <span class="text-sm text-secondary-500">{{ formatDate(row.created_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="支付时间" width="180">
          <template #default="{ row }">
            <span class="text-sm text-secondary-500">{{ formatDate(row.paid_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" :icon="View" @click="viewDetail(row)">查看</el-button>
          </template>
        </el-table-column>
        <template #empty><el-empty description="暂无订单数据" /></template>
      </el-table>

      <div class="flex justify-end p-4 border-t border-secondary-100">
        <el-pagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="handlePageChange"
        />
      </div>
    </div>

    <!-- 订单详情弹窗 -->
    <el-dialog v-model="detailDialogVisible" title="订单详情" width="560px">
      <div v-if="detailOrder" class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div>
            <span class="text-sm text-secondary-500">订单号</span>
            <p class="font-mono font-medium">{{ detailOrder.order_no || detailOrder.id }}</p>
          </div>
          <div>
            <span class="text-sm text-secondary-500">状态</span>
            <p>
              <el-tag :type="(statusTagMap[detailOrder.status]?.type as any) || 'info'" size="small">
                {{ statusTagMap[detailOrder.status]?.label || detailOrder.status }}
              </el-tag>
            </p>
          </div>
          <div>
            <span class="text-sm text-secondary-500">用户</span>
            <p class="font-medium">{{ detailOrder.username || detailOrder.user_id }}</p>
          </div>
          <div>
            <span class="text-sm text-secondary-500">方案</span>
            <p>{{ planTagMap[detailOrder.plan]?.label || detailOrder.plan || '-' }}</p>
          </div>
          <div>
            <span class="text-sm text-secondary-500">金额</span>
            <p class="text-lg font-bold text-primary-600">¥{{ detailOrder.amount || 0 }}</p>
          </div>
          <div>
            <span class="text-sm text-secondary-500">支付方式</span>
            <p>{{ detailOrder.payment_method || '-' }}</p>
          </div>
          <div>
            <span class="text-sm text-secondary-500">创建时间</span>
            <p class="text-sm">{{ formatDate(detailOrder.created_at) }}</p>
          </div>
          <div>
            <span class="text-sm text-secondary-500">支付时间</span>
            <p class="text-sm">{{ formatDate(detailOrder.paid_at) }}</p>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="detailDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>
