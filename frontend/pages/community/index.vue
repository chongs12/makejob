<script setup lang="ts">
import { ChatDotRound, Clock, Document, EditPen, Plus, Search, View } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

definePageMeta({
  title: '交流社区',
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

interface PagePayload<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

const authStore = useAuthStore()
const { $api } = useNuxtApp()

const loading = ref(false)
const posting = ref(false)
const page = ref(1)
const pageSize = 10
const total = ref(0)
const keyword = ref('')
const activeType = ref<'all' | 'article' | 'moment'>('all')
const posts = ref<CommunityPostItem[]>([])

const composer = reactive({
  postType: 'moment' as 'article' | 'moment',
  title: '',
  content: '',
  tags: '',
})

const hotTags = computed(() => {
  const counter = new Map<string, number>()

  posts.value.forEach((post) => {
    post.tags.forEach((tag) => {
      counter.set(tag, (counter.get(tag) || 0) + 1)
    })
  })

  const tags = [...counter.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, 8)
    .map(([tag]) => tag)

  return tags.length ? tags : ['Go', 'Java', '前端', '面经', '简历', '算法']
})

const stats = computed(() => {
  const articleCount = posts.value.filter(post => post.post_type === 'article').length
  const momentCount = posts.value.filter(post => post.post_type === 'moment').length

  return [
    { label: '当前帖子', value: total.value },
    { label: '文章', value: articleCount },
    { label: '说说', value: momentCount },
  ]
})

const fetchPosts = async () => {
  loading.value = true

  try {
    const response = await $api.get<PagePayload<CommunityPostItem>>('/community/posts', {
      page: page.value,
      page_size: pageSize,
      type: activeType.value === 'all' ? undefined : activeType.value,
      keyword: keyword.value.trim() || undefined,
    })

    posts.value = response.data.list || []
    total.value = response.data.total || 0
  } catch {
    posts.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

const handleSearch = async () => {
  page.value = 1
  await fetchPosts()
}

const handleTypeChange = async (type: 'all' | 'article' | 'moment') => {
  activeType.value = type
  page.value = 1
  await fetchPosts()
}

const resetComposer = () => {
  composer.postType = 'moment'
  composer.title = ''
  composer.content = ''
  composer.tags = ''
}

const submitPost = async () => {
  if (!authStore.isLoggedIn) {
    ElMessage.warning('请先登录后再发布内容')
    await navigateTo('/auth/login')
    return
  }

  const content = composer.content.trim()
  const title = composer.title.trim()
  if (!content) {
    ElMessage.warning('请输入内容')
    return
  }

  if (composer.postType === 'article' && !title) {
    ElMessage.warning('文章需要标题')
    return
  }

  posting.value = true

  try {
    const currentPostType = composer.postType
    const tags = composer.tags
      .split(/[,\s，]+/)
      .map(tag => tag.trim())
      .filter(Boolean)

    await $api.post('/community/posts', {
      post_type: currentPostType,
      title,
      content,
      tags,
    })

    ElMessage.success(currentPostType === 'article' ? '文章已发布' : '说说已发布')
    resetComposer()
    page.value = 1

    if (activeType.value !== 'all' && activeType.value !== currentPostType) {
      activeType.value = currentPostType
    }

    await fetchPosts()
  } finally {
    posting.value = false
  }
}

const handlePageChange = async (value: number) => {
  page.value = value
  await fetchPosts()
}

onMounted(() => {
  void fetchPosts()
})
</script>

<template>
  <div class="space-y-8">
    <section class="overflow-hidden rounded-[32px] border border-white/70 bg-[linear-gradient(135deg,rgba(37,99,235,0.12),rgba(255,255,255,0.96)_48%,rgba(14,165,233,0.12))] px-6 py-8 shadow-[0_18px_48px_rgba(15,23,42,0.06)] sm:px-8">
      <div class="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
        <div class="max-w-2xl">
          <p class="text-xs font-semibold uppercase tracking-[0.24em] text-primary-500">Community</p>
          <h2 class="mt-3 text-3xl font-semibold text-secondary-900 sm:text-4xl">像牛客社区一样，登录后就能发文章和说说</h2>
          <p class="mt-4 text-sm leading-7 text-secondary-600 sm:text-base">
            这里先做成后续替换 mock 也能直接接入的真实社区骨架。普通用户登录后即可发布长文和短内容，管理员继续保留后台管理入口。
          </p>
        </div>

        <div class="grid grid-cols-3 gap-3 rounded-3xl bg-white/80 p-4 backdrop-blur">
          <div
            v-for="item in stats"
            :key="item.label"
            class="min-w-[88px] rounded-2xl bg-secondary-50 px-4 py-3 text-center"
          >
            <div class="text-xl font-semibold text-secondary-900">{{ item.value }}</div>
            <div class="mt-1 text-xs text-secondary-500">{{ item.label }}</div>
          </div>
        </div>
      </div>
    </section>

    <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr),320px]">
      <div class="space-y-6">
        <section
          id="composer"
          class="rounded-[28px] border border-white/70 bg-white/95 p-6 shadow-[0_18px_48px_rgba(15,23,42,0.06)]"
        >
          <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 class="text-xl font-semibold text-secondary-900">发帖区</h3>
              <p class="mt-1 text-sm text-secondary-500">
                {{ authStore.isLoggedIn ? '支持发布文章和短说说，后端接口已接通。' : '登录后可直接发布文章和说说。' }}
              </p>
            </div>

            <div class="inline-flex rounded-full border border-secondary-200 bg-secondary-50 p-1">
              <button
                class="rounded-full px-4 py-2 text-sm font-medium transition-colors"
                :class="composer.postType === 'moment' ? 'bg-white text-primary-600 shadow-sm' : 'text-secondary-600'"
                type="button"
                @click="composer.postType = 'moment'"
              >
                说说
              </button>
              <button
                class="rounded-full px-4 py-2 text-sm font-medium transition-colors"
                :class="composer.postType === 'article' ? 'bg-white text-primary-600 shadow-sm' : 'text-secondary-600'"
                type="button"
                @click="composer.postType = 'article'"
              >
                文章
              </button>
            </div>
          </div>

          <div class="mt-5 space-y-4">
            <el-input
              v-if="composer.postType === 'article'"
              v-model="composer.title"
              maxlength="120"
              placeholder="输入文章标题，适合分享面经、经验总结、项目复盘"
              size="large"
            />

            <el-input
              v-model="composer.content"
              type="textarea"
              :rows="composer.postType === 'article' ? 10 : 5"
              maxlength="5000"
              show-word-limit
              placeholder="写点什么吧。比如今日面试体验、刷题心得、项目踩坑、求助问题。"
            />

            <div class="flex flex-col gap-3 lg:flex-row lg:items-center">
              <el-input
                v-model="composer.tags"
                placeholder="标签，多个标签用逗号分隔，如 Go, 面经, 后端"
                size="large"
              />

              <div class="flex items-center gap-3">
                <NuxtLink
                  v-if="!authStore.isLoggedIn"
                  to="/auth/login"
                  class="inline-flex items-center justify-center rounded-full border border-secondary-200 px-5 py-2.5 text-sm font-medium text-secondary-700 transition-colors hover:border-primary-200 hover:text-primary-600"
                >
                  去登录
                </NuxtLink>

                <el-button
                  type="primary"
                  size="large"
                  :loading="posting"
                  class="!h-11 !rounded-full !px-6"
                  @click="submitPost"
                >
                  <el-icon class="mr-1"><Plus /></el-icon>
                  发布内容
                </el-button>
              </div>
            </div>
          </div>
        </section>

        <section class="rounded-[28px] border border-white/70 bg-white/95 p-6 shadow-[0_18px_48px_rgba(15,23,42,0.06)]">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div class="flex flex-wrap items-center gap-2">
              <button
                v-for="tab in [
                  { key: 'all', label: '全部' },
                  { key: 'article', label: '文章' },
                  { key: 'moment', label: '说说' },
                ]"
                :key="tab.key"
                class="rounded-full px-4 py-2 text-sm font-medium transition-colors"
                :class="activeType === tab.key
                  ? 'bg-primary-600 text-white'
                  : 'bg-secondary-50 text-secondary-600 hover:bg-secondary-100'"
                type="button"
                @click="handleTypeChange(tab.key as 'all' | 'article' | 'moment')"
              >
                {{ tab.label }}
              </button>
            </div>

            <div class="flex w-full gap-3 lg:w-[360px]">
              <el-input
                v-model="keyword"
                size="large"
                placeholder="搜索标题或正文"
                @keyup.enter="handleSearch"
              >
                <template #prefix>
                  <el-icon><Search /></el-icon>
                </template>
              </el-input>

              <el-button size="large" @click="handleSearch">搜索</el-button>
            </div>
          </div>

          <div v-loading="loading" class="mt-6 space-y-4">
            <article
              v-for="post in posts"
              :key="post.id"
              class="rounded-[24px] border border-secondary-100 bg-secondary-50/65 p-5 transition-all hover:border-primary-200 hover:bg-white hover:shadow-[0_16px_36px_rgba(37,99,235,0.08)]"
            >
              <div class="flex items-start justify-between gap-4">
                <div class="flex min-w-0 gap-3">
                  <el-avatar :src="post.author.avatar" :size="42">
                    {{ post.author.username.slice(0, 1).toUpperCase() }}
                  </el-avatar>

                  <div class="min-w-0">
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="font-medium text-secondary-900">{{ post.author.username }}</span>
                      <span
                        class="rounded-full px-2.5 py-1 text-xs font-medium"
                        :class="post.post_type === 'article'
                          ? 'bg-blue-100 text-blue-700'
                          : 'bg-emerald-100 text-emerald-700'"
                      >
                        {{ post.post_type === 'article' ? '文章' : '说说' }}
                      </span>
                      <span
                        v-if="post.is_pinned"
                        class="rounded-full bg-amber-100 px-2.5 py-1 text-xs font-medium text-amber-700"
                      >
                        置顶
                      </span>
                      <span
                        v-if="post.is_recommended"
                        class="rounded-full bg-rose-100 px-2.5 py-1 text-xs font-medium text-rose-700"
                      >
                        推荐
                      </span>
                    </div>

                    <div class="mt-1 flex items-center gap-3 text-xs text-secondary-500">
                      <span class="inline-flex items-center gap-1">
                        <el-icon><Clock /></el-icon>
                        {{ post.created_at }}
                      </span>
                    </div>
                  </div>
                </div>

                <NuxtLink
                  :to="`/community/${post.id}`"
                  class="hidden rounded-full border border-secondary-200 px-4 py-2 text-sm font-medium text-secondary-700 transition-colors hover:border-primary-200 hover:text-primary-600 md:inline-flex"
                >
                  查看详情
                </NuxtLink>
              </div>

              <div class="mt-4">
                <NuxtLink
                  :to="`/community/${post.id}`"
                  class="group block"
                >
                  <h3
                    v-if="post.title"
                    class="text-lg font-semibold text-secondary-900 transition-colors group-hover:text-primary-600"
                  >
                    {{ post.title }}
                  </h3>
                  <p
                    class="mt-2 whitespace-pre-line text-sm leading-7 text-secondary-600"
                    :class="post.post_type === 'article' ? 'line-clamp-3' : 'line-clamp-4'"
                  >
                    {{ post.post_type === 'article' ? post.summary : post.content }}
                  </p>
                </NuxtLink>
              </div>

              <div class="mt-4 flex flex-wrap items-center justify-between gap-3">
                <div class="flex flex-wrap gap-2">
                  <span
                    v-for="tag in post.tags"
                    :key="tag"
                    class="rounded-full bg-white px-3 py-1 text-xs text-secondary-500 ring-1 ring-secondary-200"
                  >
                    # {{ tag }}
                  </span>
                </div>

                <div class="flex items-center gap-4 text-xs text-secondary-500">
                  <span class="inline-flex items-center gap-1">
                    <el-icon><View /></el-icon>
                    {{ post.view_count }}
                  </span>
                  <span class="inline-flex items-center gap-1">
                    <el-icon><ChatDotRound /></el-icon>
                    {{ post.comment_count }}
                  </span>
                  <span class="inline-flex items-center gap-1">
                    <el-icon><Document /></el-icon>
                    {{ post.like_count }}
                  </span>
                </div>
              </div>
            </article>

            <div
              v-if="!loading && posts.length === 0"
              class="rounded-[24px] border border-dashed border-secondary-200 bg-secondary-50 px-6 py-16 text-center"
            >
              <el-icon :size="28" class="text-secondary-300">
                <EditPen />
              </el-icon>
              <p class="mt-3 text-base font-medium text-secondary-700">还没有社区内容</p>
              <p class="mt-2 text-sm text-secondary-500">现在就发布第一条文章或说说，后续开发替换 mock 时可以直接复用这套接口。</p>
            </div>
          </div>

          <div class="mt-6 flex justify-center" v-if="total > pageSize">
            <el-pagination
              layout="prev, pager, next"
              :current-page="page"
              :page-size="pageSize"
              :total="total"
              background
              @current-change="handlePageChange"
            />
          </div>
        </section>
      </div>

      <aside class="space-y-5">
        <section class="rounded-[28px] border border-white/70 bg-white/95 p-6 shadow-[0_18px_48px_rgba(15,23,42,0.06)]">
          <h3 class="text-lg font-semibold text-secondary-900">社区使用说明</h3>
          <ul class="mt-4 space-y-3 text-sm leading-7 text-secondary-600">
            <li>文章适合发面经、项目总结、学习复盘。</li>
            <li>说说适合发简短动态、提问、随手记录。</li>
            <li>普通登录用户即可发布，不需要管理员权限。</li>
            <li>当前版本先支持发帖、列表和详情，评论点赞后续再扩展。</li>
          </ul>
        </section>

        <section class="rounded-[28px] border border-white/70 bg-white/95 p-6 shadow-[0_18px_48px_rgba(15,23,42,0.06)]">
          <h3 class="text-lg font-semibold text-secondary-900">热门标签</h3>
          <div class="mt-4 flex flex-wrap gap-2">
            <button
              v-for="tag in hotTags"
              :key="tag"
              class="rounded-full bg-secondary-50 px-3 py-2 text-sm text-secondary-600 transition-colors hover:bg-primary-50 hover:text-primary-600"
              type="button"
              @click="keyword = tag; handleSearch()"
            >
              # {{ tag }}
            </button>
          </div>
        </section>

        <section class="rounded-[28px] border border-primary-100 bg-[linear-gradient(180deg,rgba(239,246,255,0.92),rgba(255,255,255,0.98))] p-6 shadow-[0_18px_48px_rgba(15,23,42,0.06)]">
          <h3 class="text-lg font-semibold text-secondary-900">快速入口</h3>
          <div class="mt-4 space-y-3">
            <NuxtLink
              to="/practice"
              class="flex items-center justify-between rounded-2xl bg-white px-4 py-3 text-sm font-medium text-secondary-700 transition-colors hover:text-primary-600"
            >
              <span>去刷题</span>
              <span>></span>
            </NuxtLink>
            <NuxtLink
              to="/interview"
              class="flex items-center justify-between rounded-2xl bg-white px-4 py-3 text-sm font-medium text-secondary-700 transition-colors hover:text-primary-600"
            >
              <span>去面试模拟</span>
              <span>></span>
            </NuxtLink>
            <NuxtLink
              to="/plan"
              class="flex items-center justify-between rounded-2xl bg-white px-4 py-3 text-sm font-medium text-secondary-700 transition-colors hover:text-primary-600"
            >
              <span>去做计划</span>
              <span>></span>
            </NuxtLink>
          </div>
        </section>
      </aside>
    </div>
  </div>
</template>
