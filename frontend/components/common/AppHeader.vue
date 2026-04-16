<script setup lang="ts">
import { ArrowDown, ChatDotRound, House, Medal, Notebook, Plus, User } from '@element-plus/icons-vue'

const authStore = useAuthStore()
const route = useRoute()
const router = useRouter()

const navLinks = [
  { path: '/', title: '首页' },
  { path: '/community', title: '交流社区' },
  { path: '/practice', title: '题库练习' },
  { path: '/interview', title: '模拟面试' },
  { path: '/plan', title: '学习计划' },
  { path: '/membership', title: '会员中心' },
]

const userDropdownItems = computed(() => {
  const items = [
    { command: '/dashboard', title: '我的面板', icon: User },
    { command: '/community', title: '我的社区', icon: ChatDotRound },
    { command: '/membership', title: '会员中心', icon: Medal },
    { command: '/settings', title: '账号设置', icon: Notebook },
  ]

  if (authStore.isAdmin) {
    items.unshift({ command: '/admin', title: '后台管理', icon: House })
  }

  return items
})

const isActive = (path: string) => {
  if (path === '/') return route.path === '/'
  return route.path === path || route.path.startsWith(`${path}/`)
}

const handleDropdownCommand = async (command: string) => {
  if (command === 'logout') {
    await authStore.logout()
    return
  }

  await router.push(command)
}
</script>

<template>
  <header class="fixed inset-x-0 top-0 z-50 border-b border-white/70 bg-white/92 backdrop-blur-xl">
    <div class="container-custom flex h-16 items-center justify-between gap-4">
      <div class="flex min-w-0 flex-1 items-center gap-6 lg:gap-10">
        <NuxtLink to="/" class="flex items-center gap-3">
          <div class="flex h-10 w-10 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-600 via-primary-500 to-primary-700 text-sm font-bold text-white shadow-[0_10px_24px_rgba(37,99,235,0.28)]">
            M
          </div>
          <div class="min-w-0">
            <div class="text-lg font-semibold text-secondary-900">MakeJob</div>
            <div class="hidden text-xs text-secondary-500 sm:block">AI 求职训练与交流社区</div>
          </div>
        </NuxtLink>

        <nav class="hidden items-center gap-1 rounded-full border border-secondary-200/80 bg-secondary-50/80 p-1 lg:flex">
          <NuxtLink
            v-for="link in navLinks"
            :key="link.path"
            :to="link.path"
            class="rounded-full px-4 py-2 text-sm font-medium transition-colors"
            :class="isActive(link.path)
              ? 'bg-white text-primary-600 shadow-sm'
              : 'text-secondary-600 hover:bg-white hover:text-secondary-900'"
          >
            {{ link.title }}
          </NuxtLink>
        </nav>
      </div>

      <div class="flex items-center gap-2 sm:gap-3">
        <NuxtLink
          v-if="authStore.isLoggedIn"
          to="/community#composer"
          class="hidden items-center justify-center gap-2 rounded-full border border-primary-200 bg-primary-50 px-4 py-2 text-sm font-medium text-primary-700 transition-colors hover:bg-primary-100 md:inline-flex"
        >
          <el-icon><Plus /></el-icon>
          发帖
        </NuxtLink>

        <NuxtLink
          v-if="authStore.isAdmin"
          to="/admin"
          class="hidden items-center justify-center rounded-full bg-secondary-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-secondary-800 md:inline-flex"
        >
          后台管理
        </NuxtLink>

        <template v-if="!authStore.isLoggedIn">
          <NuxtLink
            to="/auth/login"
            class="inline-flex items-center justify-center rounded-full px-3 py-2 text-sm font-medium text-secondary-700 transition-colors hover:bg-secondary-100 hover:text-secondary-900"
          >
            登录
          </NuxtLink>

          <NuxtLink
            to="/auth/register"
            class="inline-flex items-center justify-center rounded-full bg-primary-600 px-5 py-2 text-sm font-semibold text-white shadow-[0_10px_24px_rgba(37,99,235,0.2)] transition-colors hover:bg-primary-700"
          >
            注册
          </NuxtLink>
        </template>

        <template v-else>
          <NuxtLink
            to="/dashboard"
            class="hidden items-center justify-center rounded-full border border-secondary-200 bg-white px-4 py-2 text-sm font-medium text-secondary-700 transition-colors hover:border-primary-200 hover:text-primary-600 md:inline-flex"
          >
            我的面板
          </NuxtLink>

          <el-dropdown trigger="click" @command="handleDropdownCommand">
            <button
              class="flex items-center gap-2 rounded-full border border-secondary-200 bg-white px-2 py-1.5 shadow-sm transition-colors hover:border-primary-200 hover:bg-primary-50"
              type="button"
            >
              <el-avatar :size="32" :src="authStore.avatar">
                {{ (authStore.username || 'U').slice(0, 1).toUpperCase() }}
              </el-avatar>
              <span class="hidden max-w-[120px] truncate text-sm font-medium text-secondary-800 sm:inline">
                {{ authStore.username || '用户' }}
              </span>
              <el-icon class="text-secondary-400">
                <ArrowDown />
              </el-icon>
            </button>

            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item
                  v-for="item in userDropdownItems"
                  :key="item.command"
                  :command="item.command"
                >
                  <div class="flex items-center gap-2">
                    <el-icon :size="16">
                      <component :is="item.icon" />
                    </el-icon>
                    <span>{{ item.title }}</span>
                  </div>
                </el-dropdown-item>

                <el-dropdown-item command="logout" divided>
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </div>
    </div>
  </header>
</template>
