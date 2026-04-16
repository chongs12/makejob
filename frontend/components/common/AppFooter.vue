<template>
  <footer class="border-t border-secondary-200/80 bg-white/90 py-10 backdrop-blur">
    <div class="container-custom">
      <div class="flex flex-col gap-6 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <div class="text-lg font-semibold text-secondary-900">MakeJob</div>
          <p class="mt-2 max-w-xl text-sm leading-6 text-secondary-500">
            面向求职训练、模拟面试和经验交流的统一入口。现在已经补齐顶部导航、登录退出和基础社区模块，后续替换 mock 时可以直接沿用这套页面结构与接口。
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-5 text-sm">
          <NuxtLink to="/" class="text-secondary-500 transition-colors hover:text-secondary-900">
            首页
          </NuxtLink>
          <NuxtLink to="/community" class="text-secondary-500 transition-colors hover:text-secondary-900">
            交流社区
          </NuxtLink>
          <NuxtLink to="/practice" class="text-secondary-500 transition-colors hover:text-secondary-900">
            题库练习
          </NuxtLink>
          <NuxtLink to="/interview" class="text-secondary-500 transition-colors hover:text-secondary-900">
            模拟面试
          </NuxtLink>
          <NuxtLink to="/plan" class="text-secondary-500 transition-colors hover:text-secondary-900">
            学习计划
          </NuxtLink>
          <a href="mailto:support@makejob.com" class="text-secondary-500 transition-colors hover:text-secondary-900">
            support@makejob.com
          </a>
        </div>
      </div>

      <div class="mt-8 border-t border-secondary-100 pt-5 text-xs text-secondary-400">
        &copy; {{ new Date().getFullYear() }} MakeJob. All rights reserved.
      </div>
    </div>
  </footer>
</template>
