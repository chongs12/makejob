<script setup lang="ts">
/**
 * 用户管理页面
 */

import { Search, Refresh, UserFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

definePageMeta({
  title: '用户管理',
  layout: 'admin',
  middleware: ['admin']
})

const api = useApi()

// 状态
const loading = ref(false)
const users = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const keyword = ref('')
const roleFilter = ref('')

// 角色选项
const roleOptions = [
  { label: '全部', value: '' },
  { label: '管理员', value: 'admin' },
  { label: '高级会员', value: 'pro_member' },
  { label: '免费用户', value: 'free_member' },
]

// 角色标签配置
const roleTagMap: Record<string, { type: string; label: string }> = {
  admin: { type: 'danger', label: '管理员' },
  pro_member: { type: 'warning', label: '高级会员' },
  free_member: { type: 'info', label: '免费用户' },
  user: { type: 'info', label: '普通用户' },
}

// 获取用户列表
const fetchUsers = async () => {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: page.value,
      page_size: pageSize.value,
    }
    if (keyword.value) params.keyword = keyword.value
    if (roleFilter.value) params.role = roleFilter.value

    const res = await api.get<any>('/admin/users', params)
    if (res.code === 0 || res.code === 200) {
      users.value = res.data?.list || res.data || []
      total.value = res.data?.total || 0
    }
  } catch (error) {
    console.error('获取用户列表失败', error)
  } finally {
    loading.value = false
  }
}

// 搜索
const handleSearch = () => {
  page.value = 1
  fetchUsers()
}

// 修改角色
const handleRoleChange = async (userId: number, newRole: string) => {
  try {
    await api.put(`/admin/users/${userId}/role`, { role: newRole })
    ElMessage.success('角色修改成功')
    fetchUsers()
  } catch (error) {
    ElMessage.error('角色修改失败')
  }
}

// 禁用/启用用户
const handleToggleDisable = async (user: any) => {
  const action = user.is_disabled ? '启用' : '禁用'
  try {
    await ElMessageBox.confirm(`确定要${action}用户 ${user.username} 吗？`, '确认操作', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await api.put(`/admin/users/${user.id}/disable`)
    ElMessage.success(`${action}成功`)
    fetchUsers()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(`${action}失败`)
    }
  }
}

// 格式化时间
const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN')
}

// 分页变化
const handlePageChange = (newPage: number) => {
  page.value = newPage
  fetchUsers()
}

const handleSizeChange = (newSize: number) => {
  pageSize.value = newSize
  page.value = 1
  fetchUsers()
}

onMounted(() => {
  fetchUsers()
})
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold text-secondary-900">用户管理</h1>

    <!-- 工具栏 -->
    <div class="bg-white rounded-lg shadow-sm border border-secondary-200 p-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-4">
          <el-input
            v-model="keyword"
            placeholder="搜索用户名/邮箱"
            class="w-64"
            clearable
            @keyup.enter="handleSearch"
            @clear="handleSearch"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
          <el-select v-model="roleFilter" placeholder="角色筛选" class="w-36" @change="handleSearch">
            <el-option
              v-for="opt in roleOptions"
              :key="opt.value"
              :label="opt.label"
              :value="opt.value"
            />
          </el-select>
        </div>
        <el-button :icon="Refresh" @click="fetchUsers">刷新</el-button>
      </div>
    </div>

    <!-- 用户表格 -->
    <div class="bg-white rounded-lg shadow-sm border border-secondary-200">
      <el-table :data="users" v-loading="loading" style="width: 100%">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="用户" min-width="200">
          <template #default="{ row }">
            <div class="flex items-center gap-3">
              <el-avatar :size="36" :src="row.avatar">
                {{ (row.username || '?')[0] }}
              </el-avatar>
              <span class="font-medium text-secondary-900">{{ row.username }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="email" label="邮箱" min-width="200" />
        <el-table-column label="角色" width="140">
          <template #default="{ row }">
            <el-tag
              :type="(roleTagMap[row.role]?.type as any) || 'info'"
              size="small"
            >
              {{ roleTagMap[row.role]?.label || row.role }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="会员等级" width="120">
          <template #default="{ row }">
            <span class="text-sm text-secondary-600">{{ row.membership_type || '免费' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="注册时间" width="180">
          <template #default="{ row }">
            <span class="text-sm text-secondary-500">{{ formatDate(row.created_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <div class="flex items-center gap-2">
              <el-select
                :model-value="row.role"
                size="small"
                class="w-28"
                @change="(val: string) => handleRoleChange(row.id, val)"
              >
                <el-option label="管理员" value="admin" />
                <el-option label="高级会员" value="pro_member" />
                <el-option label="免费用户" value="free_member" />
              </el-select>
              <el-switch
                :model-value="!row.is_disabled"
                size="small"
                active-text="启用"
                inactive-text="禁用"
                @change="handleToggleDisable(row)"
              />
            </div>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无用户数据" />
        </template>
      </el-table>

      <div class="flex justify-end p-4 border-t border-secondary-100">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          @current-change="handlePageChange"
          @size-change="handleSizeChange"
        />
      </div>
    </div>
  </div>
</template>
