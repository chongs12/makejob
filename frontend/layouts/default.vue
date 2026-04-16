<script setup lang="ts">
const appStore = useAppStore()
const authStore = useAuthStore()
const route = useRoute()

const isLandingPage = computed(() => route.path === '/')
const pageTitle = computed(() => {
  if (appStore.pageTitle) return appStore.pageTitle
  if (typeof route.meta.title === 'string' && route.meta.title) return route.meta.title
  return 'MakeJob'
})

onMounted(() => {
  appStore.initAppState()
  authStore.initAuth()
})
</script>

<template>
  <div class="min-h-screen bg-[#f6f8fc] text-secondary-900">
    <div class="pointer-events-none fixed inset-x-0 top-0 h-[380px] bg-[radial-gradient(circle_at_top,_rgba(37,99,235,0.12),_transparent_58%),linear-gradient(180deg,_rgba(255,255,255,0.96),_rgba(246,248,252,0.72))]" />

    <div class="relative">
      <CommonAppHeader />

      <main class="pt-20 pb-12">
        <slot v-if="isLandingPage" />

        <template v-else>
          <div class="container-custom">
            <div class="mb-6 flex flex-col gap-4 rounded-3xl border border-white/70 bg-white/90 px-6 py-5 shadow-[0_18px_48px_rgba(15,23,42,0.06)] backdrop-blur sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p class="text-xs font-semibold uppercase tracking-[0.24em] text-primary-500">MakeJob</p>
                <h1 class="mt-2 text-2xl font-semibold text-secondary-900">{{ pageTitle }}</h1>
              </div>

              <div class="flex items-center gap-3">
                <NuxtLink
                  to="/"
                  class="inline-flex items-center justify-center rounded-full border border-secondary-200 bg-secondary-50 px-4 py-2 text-sm font-medium text-secondary-700 transition-colors hover:border-primary-200 hover:bg-primary-50 hover:text-primary-600"
                >
                  返回首页
                </NuxtLink>

                <NuxtLink
                  v-if="authStore.isAdmin"
                  to="/admin"
                  class="inline-flex items-center justify-center rounded-full border border-primary-200 bg-primary-50 px-4 py-2 text-sm font-medium text-primary-700 transition-colors hover:bg-primary-100"
                >
                  后台管理
                </NuxtLink>
              </div>
            </div>

            <slot />
          </div>
        </template>
      </main>

      <CommonAppFooter />
    </div>
  </div>
</template>
