<script setup lang="ts">
/**
 * 应用根组件
 */

// 初始化应用状态
const appStore = useAppStore()
const authStore = useAuthStore()

onMounted(() => {
  appStore.initAppState()
  authStore.initAuth()
})
</script>

<template>
  <div>
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
  </div>
</template>
