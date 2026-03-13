const RECENT_APP_ID_KEY = 'simple-abtest:recent-app-id'

export const useRecentApp = () => {
  const getRecentAppId = () => {
    if (typeof window === 'undefined') return null
    const raw = window.localStorage.getItem(RECENT_APP_ID_KEY)
    if (!raw) return null
    const parsed = Number(raw)
    return Number.isInteger(parsed) && parsed > 0 ? parsed : null
  }

  const setRecentAppId = (appId: number | null) => {
    if (typeof window === 'undefined') return
    if (appId == null || appId <= 0) return
    window.localStorage.setItem(RECENT_APP_ID_KEY, String(appId))
  }

  return {
    getRecentAppId,
    setRecentAppId
  }
}
