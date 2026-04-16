<template>
  <el-card shadow="hover" class="question-card cursor-pointer" @click="$emit('click', question)">
    <div class="flex justify-between items-start">
      <div class="flex-1">
        <div class="flex items-center gap-2 mb-2">
          <el-tag size="small">{{ question.category || '未分类' }}</el-tag>
          <el-tag size="small" :type="difficultyType">{{ difficultyLabel }}</el-tag>
          <el-tag v-if="question.type" size="small" type="info">{{ question.type }}</el-tag>
        </div>
        <h3 class="font-semibold text-gray-900 line-clamp-2">{{ question.title }}</h3>
        <p v-if="question.description" class="text-sm text-gray-500 mt-1 line-clamp-2">
          {{ question.description }}
        </p>
      </div>
      <div v-if="showActions" class="ml-4 flex flex-col gap-1">
        <el-button size="small" type="primary" text @click.stop="$emit('practice', question)">
          练习
        </el-button>
        <el-button size="small" text @click.stop="$emit('favorite', question)">
          {{ question.isFavorite ? '⭐' : '☆' }}
        </el-button>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
const props = withDefaults(defineProps<{
  question: Record<string, any>
  showActions?: boolean
}>(), {
  showActions: true
})

defineEmits<{
  click: [question: Record<string, any>]
  practice: [question: Record<string, any>]
  favorite: [question: Record<string, any>]
}>()

const difficultyMap: Record<string, { type: string; label: string }> = {
  easy: { type: 'success', label: '简单' },
  medium: { type: 'warning', label: '中等' },
  hard: { type: 'danger', label: '困难' }
}

const difficultyType = computed(() => (difficultyMap[props.question.difficulty]?.type || 'info') as any)
const difficultyLabel = computed(() => difficultyMap[props.question.difficulty]?.label || '未知')
</script>
