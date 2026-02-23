import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getApps, getApp } from '@/api'
import type { Application } from '@/api/types'

export const useAppStore = defineStore('app', () => {
  const apps = ref<Application[]>([])
  const currentApp = ref<Application | null>(null)
  const loading = ref(false)

  const fetchApps = async () => {
    loading.value = true
    try {
      const res = await getApps()
      apps.value = res.data || []
      // If currentApp is set, update it from list or fetch detail
      if (currentApp.value) {
        const found = apps.value.find(a => a.id === currentApp.value?.id)
        if (found) {
          // Refresh detail
          await fetchAppDetail(found.id)
        } else {
            currentApp.value = null
        }
      } else if (apps.value.length > 0) {
        // Default select first app? Or let user select.
        // currentApp.value = apps.value[0]
        // await fetchAppDetail(apps.value[0].id)
      }
    } finally {
      loading.value = false
    }
  }

  const fetchAppDetail = async (id: number) => {
    loading.value = true
    try {
      const res = await getApp(id)
      currentApp.value = res.data
    } finally {
      loading.value = false
    }
  }

  const setApp = (app: Application) => {
    currentApp.value = app
    fetchAppDetail(app.id)
  }

  return {
    apps,
    currentApp,
    loading,
    fetchApps,
    fetchAppDetail,
    setApp
  }
})
