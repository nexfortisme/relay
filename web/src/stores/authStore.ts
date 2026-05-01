import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import {
  authLogin,
  authLogout,
  authMe,
  authRefresh,
  authRegister,
  setUnauthorizedHandler,
  type AuthUser,
} from '../lib/api'

// Tracks the currently authenticated user. The store is the single point of
// truth for the rest of the app — components never call /auth endpoints
// directly so we can centralize redirects, error formatting, and the
// "transparent refresh on 401" handling in one place.
export const useAuthStore = defineStore('auth', () => {
  const user = ref<AuthUser | null>(null)
  const initialized = ref(false)
  const unauthorizedAt = ref(0)

  const isAuthenticated = computed(() => user.value !== null)

  async function initialize(): Promise<void> {
    if (initialized.value) {
      return
    }
    try {
      // /auth/me sees the access cookie if it's still valid; the server
      // transparently rotates it via the refresh cookie inside the auth
      // middleware, so a single call covers "warm session" and "expired
      // access but valid refresh" without a separate refresh call here.
      user.value = await authMe()
    } catch {
      user.value = null
    } finally {
      initialized.value = true
    }
  }

  async function login(username: string, password: string, rememberMe: boolean): Promise<void> {
    user.value = await authLogin(username, password, rememberMe)
    initialized.value = true
  }

  async function register(
    username: string,
    password: string,
    rememberMe: boolean,
  ): Promise<void> {
    user.value = await authRegister(username, password, rememberMe)
    initialized.value = true
  }

  async function logout(): Promise<void> {
    try {
      await authLogout()
    } finally {
      user.value = null
    }
  }

  async function tryRefresh(): Promise<boolean> {
    try {
      const refreshed = await authRefresh()
      if (refreshed) {
        user.value = refreshed
        return true
      }
    } catch {
      // fall through to clear
    }
    user.value = null
    return false
  }

  function markUnauthorized(): void {
    user.value = null
    unauthorizedAt.value = Date.now()
  }

  // Wire the api client to notify the store on 401. This runs during store
  // construction, which is fine because the store is created once at app
  // start (Pinia memoizes definitions).
  setUnauthorizedHandler(markUnauthorized)

  return {
    user,
    initialized,
    unauthorizedAt,
    isAuthenticated,
    initialize,
    login,
    register,
    logout,
    tryRefresh,
    markUnauthorized,
  }
})
