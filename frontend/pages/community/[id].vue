<script setup lang="ts">
import { ArrowLeft, ChatDotRound, Clock, View } from '@element-plus/icons-vue'

definePageMeta({
  title: '帖子详情',
  layout: 'default',
})

interface CommunityPostAuthor {
  id: number
  username: string
  avatar?: string
  role: string
}

interface CommunityPostItem {
  id: number
  post_type: 'article' | 'moment'
  title: string
  content: string
  summary: string
  tags: string[]
  view_count: number
  comment_count: number
  like_count: number
  is_pinned: boolean
  is_recommended: boolean
  created_at: string
  author: CommunityPostAuthor
}

const route = useRoute()
const { $api } = useNuxtApp()

const loading = ref(false)
const post = ref<CommunityPostItem | null>(null)

const fetchDetail = async () => {
  const id = Number(route.params.id)
  if (!id) return

  loading.value = true

  try {
    const response = await $api.get<CommunityPostItem>(`/community/posts/${id}`)
    post.value = response.data
  } catch {
    post.value = null
  } finally {
    loading.value = false
  }
}

watch(() => route.params.id, () => {
  void fetchDetail()
})

onMounted(() => {
  void fetchDetail()
})
</script>

<template>
  <div class="space-y-6">
    <NuxtLink
      to="/community"
      class="inline-flex items-center gap-2 rounded-full border border-secondary-200 bg-white px-4 py-2 text-sm font-medium text-secondary-700 transition-colors hover:border-primary-200 hover:text-primary-600"
    >
      <el-icon><ArrowLeft /></el-icon>
      返回社区
    </NuxtLink>

    <section
      v-loading="loading"
      class="rounded-[32px] border border-white/70 bg-white/95 p-6 shadow-[0_18px_48px_rgba(15,23,42,0.06)] sm:p-8"
    >
      <template v-if="post">
        <div class="flex flex-wrap items-center gap-2">
          <span
            class="rounded-full px-3 py-1 text-xs font-medium"
            :class="post.post_type === 'article'
              ? 'bg-blue-100 text-blue-700'
              : 'bg-emerald-100 text-emerald-700'"
          >
            {{ post.post_type === 'article' ? '文章' : '说说' }}
          </span>
          <span
            v-if="post.is_pinned"
            class="rounded-full bg-amber-100 px-3 py-1 text-xs font-medium text-amber-700"
          >
            置顶
          </span>
          <span
            v-if="post.is_recommended"
            class="rounded-full bg-rose-100 px-3 py-1 text-xs font-medium text-rose-700"
          >
            推荐
          </span>
        </div>

        <h2 class="mt-4 text-2xl font-semibold text-secondary-900 sm:text-3xl">
          {{ post.title || '社区动态' }}
        </h2>

        <div class="mt-5 flex flex-wrap items-center gap-4 text-sm text-secondary-500">
          <div class="flex items-center gap-2">
            <el-avatar :src="post.author.avatar" :size="36">
              {{ post.author.username.slice(0, 1).toUpperCase() }}
            </el-avatar>
            <span class="font-medium text-secondary-800">{{ post.author.username }}</span>
          </div>
          <span class="inline-flex items-center gap-1">
            <el-icon><Clock /></el-icon>
            {{ post.created_at }}
          </span>
          <span class="inline-flex items-center gap-1">
            <el-icon><View /></el-icon>
            {{ post.view_count }}
          </span>
          <span class="inline-flex items-center gap-1">
            <el-icon><ChatDotRound /></el-icon>
            {{ post.comment_count }}
          </span>
        </div>

        <div class="mt-8 whitespace-pre-line rounded-[28px] bg-secondary-50/80 px-5 py-6 text-[15px] leading-8 text-secondary-700">
          {{ post.content }}
        </div>

        <div class="mt-6 flex flex-wrap gap-2">
          <span
            v-for="tag in post.tags"
            :key="tag"
            class="rounded-full bg-secondary-50 px-3 py-1.5 text-sm text-secondary-500 ring-1 ring-secondary-200"
          >
            # {{ tag }}
          </span>
        </div>
      </template>

      <div
        v-else-if="!loading"
        class="rounded-[28px] border border-dashed border-secondary-200 bg-secondary-50 px-6 py-16 text-center"
      >
        <p class="text-base font-medium text-secondary-700">帖子不存在或暂时无法加载</p>
        <p class="mt-2 text-sm text-secondary-500">可以返回社区列表继续浏览其他内容。</p>
      </div>
    </section>
  </div>
</template>
