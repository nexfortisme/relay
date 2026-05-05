import { defineStore } from 'pinia'
import { ref } from 'vue'

const compactSidebarQuery = '(max-width: 760px)'
const themeStorageKey = 'relay.theme'

function getStoredTheme(): 'dark' | 'light' {
  try {
    const stored = window.localStorage.getItem(themeStorageKey)
    return stored === 'light' ? 'light' : 'dark'
  } catch {
    return 'dark'
  }
}

function storeTheme(value: 'dark' | 'light') {
  try {
    window.localStorage.setItem(themeStorageKey, value)
  } catch {
    // Ignore storage write failures (private mode, blocked storage, etc).
  }
}

function shouldCollapseSidebarInitially(): boolean {
  try {
    return window.matchMedia(compactSidebarQuery).matches
  } catch {
    return false
  }
}

export const useUiStore = defineStore('ui', () => {
  const isSidebarCollapsed = ref(shouldCollapseSidebarInitially())
  const theme = ref<'dark' | 'light'>(getStoredTheme())

  function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    storeTheme(theme.value)
  }

  function toggleSidebarCollapsed() {
    isSidebarCollapsed.value = !isSidebarCollapsed.value
  }

  function setSidebarCollapsed(collapsed: boolean) {
    isSidebarCollapsed.value = collapsed
  }

  return {
    isSidebarCollapsed,
    theme,
    toggleTheme,
    toggleSidebarCollapsed,
    setSidebarCollapsed,
  }
})
