<template>
  <div class="plan-timeline">
    <el-timeline>
      <el-timeline-item
        v-for="(item, idx) in items"
        :key="idx"
        :type="getItemType(item)"
        :hollow="item.status !== 'completed'"
        :timestamp="item.date || ''"
        placement="top"
      >
        <el-card shadow="hover" class="!p-0">
          <div class="p-3">
            <div class="flex items-center justify-between">
              <h4 class="font-semibold text-sm">{{ item.title }}</h4>
              <el-tag :type="getItemType(item)" size="small">{{ getStatusLabel(item.status) }}</el-tag>
            </div>
            <p v-if="item.description" class="text-xs text-gray-500 mt-1">{{ item.description }}</p>
            <el-progress
              v-if="item.progress !== undefined"
              :percentage="item.progress"
              :stroke-width="4"
              class="mt-2"
              :color="item.progress >= 100 ? '#67c23a' : '#409eff'"
            />
          </div>
        </el-card>
      </el-timeline-item>
    </el-timeline>
    <el-empty v-if="!items.length" description="暂无学习计划" />
  </div>
</template>

<script setup lang="ts">
defineProps<{
  items: Array<{
    title: string
    description?: string
    date?: string
    status: 'pending' | 'in_progress' | 'completed'
    progress?: number
  }>
}>()

const getItemType = (item: { status: string }) => {
  const map: Record<string, string> = {
    completed: 'success',
    in_progress: 'primary',
    pending: 'info'
  }
  return (map[item.status] || 'info') as any
}

const getStatusLabel = (status: string) => {
  const map: Record<string, string> = {
    completed: '已完成',
    in_progress: '进行中',
    pending: '待开始'
  }
  return map[status] || '未知'
}
</script>
