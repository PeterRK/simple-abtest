import { computed, ref } from 'vue'

export interface SessionInfo {
  uid: number
  token: string
  name: string
}

const SESSION_KEY = 'simple-abtest:session'

const session = ref<SessionInfo | null>(null)
const authDialogVisible = ref(false)

const parseSession = (raw: string | null): SessionInfo | null => {
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as SessionInfo
    if (!parsed || !Number.isInteger(parsed.uid) || parsed.uid <= 0 || !parsed.token || !parsed.name) return null
    return parsed
  } catch {
    return null
  }
}

export const loadSession = () => {
  if (typeof window === 'undefined') return
  session.value = parseSession(window.localStorage.getItem(SESSION_KEY))
  authDialogVisible.value = !session.value
}

export const saveSession = (next: SessionInfo) => {
  session.value = next
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(SESSION_KEY, JSON.stringify(next))
  }
  authDialogVisible.value = false
}

export const clearSession = () => {
  session.value = null
  if (typeof window !== 'undefined') {
    window.localStorage.removeItem(SESSION_KEY)
  }
  authDialogVisible.value = true
}

export const openAuthDialog = () => {
  authDialogVisible.value = true
}

export const closeAuthDialog = () => {
  if (session.value) authDialogVisible.value = false
}

export const useAuth = () => ({
  session,
  authDialogVisible,
  isLoggedIn: computed(() => !!session.value),
  loadSession,
  saveSession,
  clearSession,
  openAuthDialog,
  closeAuthDialog
})
