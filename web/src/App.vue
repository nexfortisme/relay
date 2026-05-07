<script setup lang="ts">
import { onMounted, onUnmounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { RouterView, useRouter, useRoute } from 'vue-router'
import SettingsModal from './components/SettingsModal.vue'
import { useUiStore } from './stores/uiStore'
import { useSettingsStore } from './stores/settingsStore'
import { useChatStore } from './stores/chatStore'
import { useConversationStore } from './stores/conversationStore'
import { useAuthStore } from './stores/authStore'

const uiStore = useUiStore()
const settingsStore = useSettingsStore()
const chatStore = useChatStore()
const conversationStore = useConversationStore()
const authStore = useAuthStore()
const router = useRouter()
const route = useRoute()
const { settingsError, settingsForm, settingsSaving, showSettings } = storeToRefs(settingsStore)
const { theme } = storeToRefs(uiStore)
const { isAuthenticated, unauthorizedAt } = storeToRefs(authStore)
const compactSidebarQuery = '(max-width: 760px)'
let compactSidebarMedia: MediaQueryList | null = null

function collapseSidebarForCompactLayout(event: MediaQueryList | MediaQueryListEvent) {
  if (event.matches) {
    uiStore.setSidebarCollapsed(true)
  }
}

onMounted(async () => {
  if (typeof window.matchMedia === 'function') {
    compactSidebarMedia = window.matchMedia(compactSidebarQuery)
    collapseSidebarForCompactLayout(compactSidebarMedia)
    compactSidebarMedia.addEventListener('change', collapseSidebarForCompactLayout)
  }

  // The router's global guard already runs auth.initialize() before the
  // first navigation, but we also want to load the chat data once we know
  // a user is signed in. Doing it here keeps the appStore agnostic of the
  // auth store and avoids a double-load when login redirects in.
  if (isAuthenticated.value && !conversationStore.conversations.length) {
    await chatStore.initializeApp()
  }
})

onUnmounted(() => {
  compactSidebarMedia?.removeEventListener('change', collapseSidebarForCompactLayout)
})

watch(isAuthenticated, async (authed) => {
  if (authed) {
    await chatStore.initializeApp()
  } else {
    chatStore.closeStream()
  }
})

// When the api layer reports a 401 we kick the user back to /login. We use
// a simple monotonic counter (unauthorizedAt) so multiple in-flight requests
// returning 401 trigger the redirect just once.
watch(unauthorizedAt, (value) => {
  if (value === 0) return
  if (route.meta?.public) return
  router.replace({ name: 'login', query: { redirect: route.fullPath } })
})
</script>

<template>
  <main class="layout" :data-theme="theme">
    <RouterView />
    <SettingsModal
      v-if="showSettings"
      v-model:settings="settingsForm"
      :error="settingsError"
      :saving="settingsSaving"
      @close="settingsStore.closeSettings"
      @save="settingsStore.saveSettings"
    />
  </main>
</template>

<style>
html,
body,
#app {
  margin: 0;
  padding: 0;
  width: 100%;
  height: 100%;
  overflow: hidden;
}

*,
*::before,
*::after {
  box-sizing: border-box;
}
</style>

<style scoped>
.layout {
  height: 100dvh;
  overflow: hidden;
  background: var(--bg);
  color: var(--text);
  font-family:
    Inter,
    ui-sans-serif,
    system-ui,
    -apple-system,
    BlinkMacSystemFont,
    'Segoe UI',
    sans-serif;
}

.layout[data-theme='dark'] {
  --bg: #0f1115;
  --sidebar: #151821;
  --surface: #191d27;
  --surface-soft: #202533;
  --surface-hover: #262c3a;
  --selected: #222b3f;
  --text: #f4f7fb;
  --muted: #9aa5b5;
  --border: #2b3240;
  --primary: #3b82f6;
  --primary-strong: #2563eb;
  --danger: #dc2626;
  --danger-strong: #b91c1c;
  --shadow: 0 18px 46px rgba(0, 0, 0, 0.34);
}

.layout[data-theme='light'] {
  --bg: #f6f7f9;
  --sidebar: #ffffff;
  --surface: #ffffff;
  --surface-soft: #f1f3f6;
  --surface-hover: #e9edf2;
  --selected: #eef4ff;
  --text: #111827;
  --muted: #667085;
  --border: #d9dee7;
  --primary: #2563eb;
  --primary-strong: #1d4ed8;
  --danger: #dc2626;
  --danger-strong: #b91c1c;
  --shadow: 0 18px 46px rgba(31, 41, 55, 0.16);
}
</style>
