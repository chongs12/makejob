<template>
  <el-card shadow="hover" class="stats-card">
    <div class="flex items-center gap-4">
      <div class="icon-wrapper w-12 h-12 rounded-xl flex items-center justify-center" :style="{ backgroundColor: bgColor }">
        <el-icon :size="24" :style="{ color: iconColor }">
          <component :is="icon" />
        </el-icon>
      </div>
      <div class="flex-1">
        <p class="text-sm text-gray-500">{{ title }}</p>
        <div class="flex items-baseline gap-2">
          <span class="text-2xl font-bold text-gray-900">{{ displayValue }}</span>
          <span v-if="suffix" class="text-sm text-gray-400">{{ suffix }}</span>
        </div>
        <div v-if="trend !== undefined" class="text-xs mt-1" :class="trend >= 0 ? 'text-green-500' : 'text-red-500'">
          {{ trend >= 0 ? '↑' : '↓' }} {{ Math.abs(trend) }}%
        </div>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { TrendCharts } from '@element-plus/icons-vue'

const props = withDefaults(defineProps<{
  title: string
  value: number | string
  icon?: any
  bgColor?: string
  iconColor?: string
  suffix?: string
  trend?: number
}>(), {
  icon: TrendCharts,
  bgColor: '#ecf5ff',
  iconColor: '#409eff'
})

const displayValue = computed(() => {
  if (typeof props.value === 'number' && props.value >= 10000) {
    return (props.value / 10000).toFixed(1) + 'w'
  }
  return String(props.value)
})
</script>
