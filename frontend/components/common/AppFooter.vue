<script setup lang="ts">
/**
 * 页脚组件
 * 简洁版权信息
 */
</script>

<template>
  <footer class="py-6 border-t border-secondary-200 bg-white">
    <div class="container-custom">
      <div class="flex flex-col md:flex-row items-center justify-between gap-4">
        <!-- 版权信息 -->
        <div class="text-sm text-secondary-500">
          <p>&copy; {{ new Date().getFullYear() }} MakeJob. All rights reserved.</p>
        </div>
        
        <!-- 链接 -->
        <div class="flex items-center gap-6 text-sm">
          <NuxtLink to="/" class="text-secondary-500 hover:text-secondary-700 transition-colors">
            关于我们
          </NuxtLink>
          <NuxtLink to="/" class="text-secondary-500 hover:text-secondary-700 transition-colors">
            使用条款
          </NuxtLink>
          <NuxtLink to="/" class="text-secondary-500 hover:text-secondary-700 transition-colors">
            隐私政策
          </NuxtLink>
          <a 
            href="mailto:support@makejob.com" 
            class="text-secondary-500 hover:text-secondary-700 transition-colors"
          >
            联系我们
          </a>
        </div>
      </div>
    </div>
  </footer>
</template>
