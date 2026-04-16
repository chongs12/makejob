/**
 * API插件
 * 将useApi注入到NuxtApp中，方便全局使用
 */

import { useApi } from '~/composables/useApi'

export default defineNuxtPlugin(() => {
  const api = useApi()
  
  return {
    provide: {
      api
    }
  }
})
