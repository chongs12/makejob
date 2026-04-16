<script setup lang="ts">
import { Calendar, Check, Refresh, DataAnalysis } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

definePageMeta({
  title: '学习计划',
  layout: 'default',
  middleware: ['auth'],
})

type PlanTask = {
  id: number
  title: string
  description?: string
  task_type: string
  status: string
  due_date?: string
  day_number?: number
}

type PlanDetail = {
  id: number
  title: string
  description?: string
  progress?: number
  tasks?: PlanTask[]
}

type PlanProgress = {
  progress: number
  task_type_stats: Array<{ task_type: string; total: number; completed: number }>
  daily_progress: Array<{ day_number: number; total: number; completed: number }>
}

const INDUSTRY_CODE_TO_ID: Record<string, number> = {
  go: 1,
  java: 2,
  frontend: 3,
}

const appStore = useAppStore()
const { $api } = useNuxtApp()

const loading = ref(true)
const creating = ref(false)
const plan = ref<PlanDetail | null>(null)
const progress = ref<PlanProgress | null>(null)
const showProgress = ref(false)
const adjusting = ref(false)

const form = ref({
  level: 'beginner',
  daily_study_time: 60,
  duration_days: 30,
  goal_description: '',
})

const levelOptions = [
  { label: '初学者', value: 'beginner' },
  { label: '中级', value: 'intermediate' },
  { label: '高级', value: 'advanced' },
]

const currentIndustryCode = computed(() => appStore.currentIndustry || 'go')
const currentIndustryId = computed(() => INDUSTRY_CODE_TO_ID[currentIndustryCode.value] || 1)

const taskTypeLabel = (taskType: string) => {
  const labels: Record<string, string> = {
    study: '学习',
    practice: '练习',
    interview: '面试',
    review: '复习',
  }
  return labels[taskType] || taskType
}

const taskTypeColor = (taskType: string) => {
  const colors: Record<string, string> = {
    study: '',
    practice: 'success',
    interview: 'warning',
    review: 'info',
  }
  return (colors[taskType] || '') as any
}

const formatGroupLabel = (task: PlanTask) => {
  if (task.day_number) {
    return `第 ${task.day_number} 天`
  }
  if (task.due_date) {
    return new Date(task.due_date).toLocaleDateString('zh-CN')
  }
  return '未分配'
}

const loadPlan = async () => {
  loading.value = true
  plan.value = null

  try {
    const res = await $api.get<any>('/plans/current')
    if (res.code === 200 && res.data?.id) {
      const detail = await $api.get<any>(`/plans/${res.data.id}`)
      if (detail.code === 200 && detail.data) {
        plan.value = detail.data
      } else {
        plan.value = res.data
      }
    }
  } catch {
    plan.value = null
  } finally {
    loading.value = false
  }
}

const createPlan = async () => {
  if (!form.value.goal_description.trim()) {
    ElMessage.warning('请输入学习目标')
    return
  }

  creating.value = true
  try {
    const res = await $api.post<any>('/plans', {
      level: form.value.level,
      daily_study_time: form.value.daily_study_time,
      duration_days: form.value.duration_days,
      goal_description: form.value.goal_description,
      industry_id: currentIndustryId.value,
      industry_code: currentIndustryCode.value,
    })

    if (res.code === 200 && res.data) {
      plan.value = res.data
      ElMessage.success('学习计划生成成功')
      await loadPlan()
    }
  } catch (e: any) {
    ElMessage.error(e?.data?.message || '生成计划失败')
  } finally {
    creating.value = false
  }
}

const toggleTask = async (task: PlanTask) => {
  if (!plan.value?.id) return

  const newStatus = task.status === 'completed' ? 'pending' : 'completed'

  try {
    const res = await $api.put<any>(`/plans/${plan.value.id}/tasks/${task.id}`, { status: newStatus })
    if (res.code === 200) {
      ElMessage.success(newStatus === 'completed' ? '任务已完成' : '已取消完成')
      await loadPlan()
      if (showProgress.value) {
        await loadProgress()
      }
    }
  } catch {
    ElMessage.error('更新失败')
  }
}

const loadProgress = async () => {
  if (!plan.value?.id) return
  showProgress.value = true

  try {
    const res = await $api.get<PlanProgress>(`/plans/${plan.value.id}/progress`)
    if (res.code === 200) {
      progress.value = res.data
    }
  } catch (e) {
    console.error(e)
  }
}

const adjustPlan = async () => {
  if (!plan.value?.id) return

  adjusting.value = true
  try {
    const res = await $api.post<any>(`/plans/${plan.value.id}/adjust`)
    if (res.code === 200) {
      ElMessage.success('计划已调整')
      await loadPlan()
      if (showProgress.value) {
        await loadProgress()
      }
    }
  } catch {
    ElMessage.error('调整失败')
  } finally {
    adjusting.value = false
  }
}

const planProgress = computed(() => {
  if (typeof plan.value?.progress === 'number') {
    return Math.round(plan.value.progress)
  }
  if (!plan.value?.tasks?.length) return 0

  const completed = plan.value.tasks.filter(task => task.status === 'completed').length
  return Math.round((completed / plan.value.tasks.length) * 100)
})

const completedCount = computed(() => {
  if (!plan.value?.tasks?.length) return 0
  return plan.value.tasks.filter(task => task.status === 'completed').length
})

const groupedTasks = computed(() => {
  if (!plan.value?.tasks?.length) return []

  const groups: Record<string, PlanTask[]> = {}
  for (const task of plan.value.tasks) {
    const label = formatGroupLabel(task)
    if (!groups[label]) {
      groups[label] = []
    }
    groups[label].push(task)
  }

  return Object.entries(groups).map(([label, tasks]) => ({ label, tasks }))
})

const maxDailyCompleted = computed(() => {
  if (!progress.value?.daily_progress?.length) return 1
  return Math.max(...progress.value.daily_progress.map(item => item.completed || 0), 1)
})

onMounted(() => {
  appStore.setPageTitle('学习计划')
  loadPlan()
})
</script>

<template>
  <div v-loading="loading">
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900">学习计划</h1>
      <p class="text-gray-500 mt-1">制定科学的学习路径，高效备战面试</p>
    </div>

    <div v-if="!loading && !plan" class="max-w-2xl mx-auto">
      <div class="bg-white rounded-lg shadow-sm p-8 text-center">
        <div class="w-20 h-20 mx-auto bg-blue-50 rounded-full flex items-center justify-center mb-4">
          <el-icon :size="40" class="text-blue-400"><Calendar /></el-icon>
        </div>
        <h2 class="text-xl font-bold text-gray-900 mb-2">创建你的专属学习计划</h2>
        <p class="text-gray-500 mb-8">AI 将根据你的情况生成个性化学习方案</p>

        <el-form label-position="top" class="text-left space-y-4">
          <el-form-item label="当前水平">
            <el-radio-group v-model="form.level" size="large">
              <el-radio-button v-for="opt in levelOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</el-radio-button>
            </el-radio-group>
          </el-form-item>

          <el-form-item label="每日学习时间">
            <div class="w-full">
              <el-slider
                v-model="form.daily_study_time"
                :min="15"
                :max="240"
                :step="15"
                show-stops
                :marks="{ 15: '15分钟', 60: '1小时', 120: '2小时', 240: '4小时' }"
              />
              <p class="text-sm text-gray-400 mt-1">{{ form.daily_study_time }} 分钟/天</p>
            </div>
          </el-form-item>

          <el-form-item label="计划天数">
            <el-input-number v-model="form.duration_days" :min="7" :max="90" :step="7" size="large" />
            <span class="ml-2 text-sm text-gray-400">天 (7-90天)</span>
          </el-form-item>

          <el-form-item label="学习目标">
            <el-input
              v-model="form.goal_description"
              type="textarea"
              :rows="3"
              placeholder="描述你的学习目标，例如：准备 Go 后端面试，重点掌握并发和微服务。"
            />
          </el-form-item>

          <el-form-item>
            <el-button type="primary" size="large" :loading="creating" @click="createPlan" class="w-full !h-12 !text-base">
              {{ creating ? 'AI 正在生成计划...' : '生成学习计划' }}
            </el-button>
          </el-form-item>
        </el-form>
      </div>
    </div>

    <div v-if="!loading && plan">
      <div class="bg-white rounded-lg shadow-sm p-6 mb-6">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h2 class="text-lg font-semibold text-gray-800">{{ plan.title || '我的学习计划' }}</h2>
            <p class="text-sm text-gray-500 mt-1">{{ plan.description || '' }}</p>
          </div>
          <div class="flex gap-2">
            <el-button :icon="Refresh" :loading="adjusting" @click="adjustPlan">调整计划</el-button>
            <el-button :icon="DataAnalysis" @click="loadProgress">进度统计</el-button>
          </div>
        </div>
        <div class="flex items-center gap-4">
          <el-progress :percentage="planProgress" :stroke-width="12" class="flex-1" :color="planProgress >= 80 ? '#22c55e' : '#409eff'" />
          <span class="text-sm text-gray-500 whitespace-nowrap">{{ completedCount }} / {{ plan.tasks?.length || 0 }} 已完成</span>
        </div>
      </div>

      <div class="space-y-4">
        <div v-for="group in groupedTasks" :key="group.label" class="flex gap-4">
          <div class="w-24 flex-shrink-0 pt-2">
            <div class="text-sm font-medium text-gray-700">{{ group.label }}</div>
            <div class="text-xs text-gray-400">{{ group.tasks.length }} 个任务</div>
          </div>

          <div class="flex flex-col items-center">
            <div class="w-3 h-3 rounded-full bg-blue-400 border-2 border-white shadow" />
            <div class="w-0.5 flex-1 bg-gray-200" />
          </div>

          <div class="flex-1 pb-4 space-y-2">
            <div
              v-for="task in group.tasks"
              :key="task.id"
              class="bg-white rounded-lg shadow-sm border p-4 cursor-pointer transition-all hover:shadow-md"
              :class="task.status === 'completed' ? 'border-green-200 bg-green-50/50' : 'border-gray-200'"
              @click="toggleTask(task)"
            >
              <div class="flex items-center gap-3">
                <div
                  class="w-6 h-6 rounded-full border-2 flex items-center justify-center flex-shrink-0 transition-colors"
                  :class="task.status === 'completed' ? 'border-green-500 bg-green-500' : 'border-gray-300'"
                >
                  <el-icon v-if="task.status === 'completed'" class="text-white" :size="14"><Check /></el-icon>
                </div>
                <span class="flex-1 text-sm" :class="task.status === 'completed' ? 'line-through text-gray-400' : 'text-gray-800'">
                  {{ task.title }}
                </span>
                <el-tag v-if="task.task_type" :type="taskTypeColor(task.task_type)" size="small">{{ taskTypeLabel(task.task_type) }}</el-tag>
              </div>
              <p v-if="task.description" class="text-xs text-gray-400 mt-1 ml-9">{{ task.description }}</p>
            </div>
          </div>
        </div>
      </div>

      <div v-if="!groupedTasks.length" class="bg-white rounded-lg shadow-sm p-12 text-center">
        <el-empty description="计划正在生成中..." />
      </div>
    </div>

    <el-drawer v-model="showProgress" title="进度统计" size="400px">
      <div v-if="progress" class="space-y-6">
        <div>
          <h4 class="text-sm font-medium text-gray-700 mb-3">总体完成度</h4>
          <el-progress :percentage="progress.progress || planProgress" :stroke-width="14" />
        </div>

        <div v-if="progress.task_type_stats?.length">
          <h4 class="text-sm font-medium text-gray-700 mb-3">各类型任务完成情况</h4>
          <div class="space-y-3">
            <div v-for="item in progress.task_type_stats" :key="item.task_type">
              <div class="flex justify-between text-sm mb-1">
                <span class="text-gray-600">{{ taskTypeLabel(item.task_type) }}</span>
                <span class="text-gray-400">{{ item.completed }} / {{ item.total }}</span>
              </div>
              <el-progress :percentage="item.total ? Math.round((item.completed / item.total) * 100) : 0" :stroke-width="8" :show-text="false" />
            </div>
          </div>
        </div>

        <div v-if="progress.daily_progress?.length">
          <h4 class="text-sm font-medium text-gray-700 mb-3">每日完成趋势</h4>
          <div class="flex items-end gap-1 h-32">
            <div v-for="day in progress.daily_progress" :key="day.day_number" class="flex-1 flex flex-col items-center justify-end gap-1">
              <div
                class="w-full bg-blue-400 rounded-t min-h-[4px]"
                :style="{ height: `${((day.completed || 0) / maxDailyCompleted) * 100}%` }"
              />
              <span class="text-xs text-gray-400">D{{ day.day_number }}</span>
            </div>
          </div>
        </div>
      </div>
      <el-empty v-else description="暂无统计数据" />
    </el-drawer>
  </div>
</template>
