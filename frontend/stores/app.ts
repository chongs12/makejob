/**
 * 应用全局状态管理
 * 管理UI状态、全局配置等
 */

import { defineStore } from 'pinia'

// 应用状态类型
interface AppState {
  // 侧边栏折叠状态
  sidebarCollapsed: boolean
  // 全局加载状态
  globalLoading: boolean
  // 当前活跃行业
  currentIndustry: string
  // 页面标题
  pageTitle: string
  // 主题模式
  theme: 'light' | 'dark'
  // 消息通知
  notifications: Notification[]
}

// 通知类型
interface Notification {
  id: string
  type: 'success' | 'warning' | 'error' | 'info'
  title: string
  message?: string
  duration?: number
}

export const useAppStore = defineStore('app', {
  state: (): AppState => ({
    sidebarCollapsed: false,
    globalLoading: false,
    currentIndustry: 'go',
    pageTitle: '',
    theme: 'light',
    notifications: [],
  }),

  getters: {
    /**
     * 侧边栏是否折叠
     */
    isSidebarCollapsed: (state): boolean => {
      return state.sidebarCollapsed
    },

    /**
     * 是否为深色主题
     */
    isDarkTheme: (state): boolean => {
      return state.theme === 'dark'
    },
  },

  actions: {
    /**
     * 切换侧边栏折叠状态
     */
    toggleSidebar() {
      this.sidebarCollapsed = !this.sidebarCollapsed
      
      // 持久化到localStorage
      if (process.client) {
        localStorage.setItem('sidebarCollapsed', String(this.sidebarCollapsed))
      }
    },

    /**
     * 设置侧边栏折叠状态
     */
    setSidebarCollapsed(collapsed: boolean) {
      this.sidebarCollapsed = collapsed
      
      if (process.client) {
        localStorage.setItem('sidebarCollapsed', String(collapsed))
      }
    },

    /**
     * 设置全局加载状态
     */
    setGlobalLoading(loading: boolean) {
      this.globalLoading = loading
    },

    /**
     * 设置当前行业
     */
    setCurrentIndustry(industry: string) {
      this.currentIndustry = industry
      
      // 持久化到localStorage
      if (process.client) {
        localStorage.setItem('currentIndustry', industry)
      }
    },

    /**
     * 设置页面标题
     */
    setPageTitle(title: string) {
      this.pageTitle = title
      
      // 同步更新document title
      if (process.client) {
        document.title = title ? `${title} - MakeJob` : 'MakeJob'
      }
    },

    /**
     * 切换主题
     */
    toggleTheme() {
      this.theme = this.theme === 'light' ? 'dark' : 'light'
      
      if (process.client) {
        localStorage.setItem('theme', this.theme)
        this.applyTheme()
      }
    },

    /**
     * 设置主题
     */
    setTheme(theme: 'light' | 'dark') {
      this.theme = theme
      
      if (process.client) {
        localStorage.setItem('theme', theme)
        this.applyTheme()
      }
    },

    /**
     * 应用主题到DOM
     */
    applyTheme() {
      if (process.client) {
        const html = document.documentElement
        if (this.theme === 'dark') {
          html.classList.add('dark')
        } else {
          html.classList.remove('dark')
        }
      }
    },

    /**
     * 添加通知
     */
    addNotification(notification: Omit<Notification, 'id'>) {
      const id = Math.random().toString(36).substr(2, 9)
      this.notifications.push({
        ...notification,
        id,
        duration: notification.duration || 3000,
      })

      // 自动移除
      setTimeout(() => {
        this.removeNotification(id)
      }, notification.duration || 3000)
    },

    /**
     * 移除通知
     */
    removeNotification(id: string) {
      const index = this.notifications.findIndex(n => n.id === id)
      if (index > -1) {
        this.notifications.splice(index, 1)
      }
    },

    /**
     * 初始化应用状态
     * 从localStorage读取持久化数据
     */
    initAppState() {
      if (process.client) {
        // 读取侧边栏状态
        const sidebarCollapsed = localStorage.getItem('sidebarCollapsed')
        if (sidebarCollapsed !== null) {
          this.sidebarCollapsed = sidebarCollapsed === 'true'
        }

        // 读取当前行业
        const currentIndustry = localStorage.getItem('currentIndustry')
        if (currentIndustry) {
          this.currentIndustry = currentIndustry
        }

        // 读取主题
        const theme = localStorage.getItem('theme') as 'light' | 'dark' | null
        if (theme) {
          this.theme = theme
          this.applyTheme()
        }
      }
    },
  },
})
