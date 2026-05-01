<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { RouterView, useRouter, useRoute } from 'vue-router'
import SettingsModal from './components/SettingsModal.vue'
import { useAppStore } from './stores/appStore'
import { useAuthStore } from './stores/authStore'

const appStore = useAppStore()
const authStore = useAuthStore()
const router = useRouter()
const route = useRoute()
const { settingsError, settingsForm, settingsSaving, showSettings, theme } =
  storeToRefs(appStore)
const { isAuthenticated, unauthorizedAt } = storeToRefs(authStore)

onMounted(async () => {
  // The router's global guard already runs auth.initialize() before the
  // first navigation, but we also want to load the chat data once we know
  // a user is signed in. Doing it here keeps the appStore agnostic of the
  // auth store and avoids a double-load when login redirects in.
  if (isAuthenticated.value && !appStore.conversations.length) {
    await appStore.initializeApp()
  }
})

watch(isAuthenticated, async (authed) => {
  if (authed) {
    await appStore.initializeApp()
  } else {
    appStore.closeStream()
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
      @close="appStore.closeSettings"
      @save="appStore.saveSettings"
    />
  </main>
</template>

<style scoped>
:global(html, body, #app) {
  margin: 0;
  height: 100%;
  overflow: hidden;
}

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
