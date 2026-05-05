<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import AppIcon from './AppIcon.vue'
import PrismAvatar from './PrismAvatar.vue'
import { useSettingsStore } from '../stores/settingsStore'
import { useAuthStore } from '../stores/authStore'
import { useUiStore } from '../stores/uiStore'

// Bottom-of-sidebar profile pill with a popover menu (Settings, Sign out).
// Lives in its own component so every sidebar in the app can drop it in
// without copying the click-outside / flyout machinery.
const settingsStore = useSettingsStore()
const authStore = useAuthStore()
const uiStore = useUiStore()
const router = useRouter()
const { user } = storeToRefs(authStore)
const { theme } = storeToRefs(uiStore)

const open = ref(false)
const root = ref<HTMLElement | null>(null)

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

function onDocumentClick(event: MouseEvent) {
  if (!open.value) return
  if (root.value && !root.value.contains(event.target as Node)) {
    close()
  }
}

document.addEventListener('mousedown', onDocumentClick)
onBeforeUnmount(() => document.removeEventListener('mousedown', onDocumentClick))

function openSettings() {
  close()
  settingsStore.openSettings()
}

async function signOut() {
  close()
  await authStore.logout()
  router.replace('/login')
}

function toggleTheme() {
  uiStore.toggleTheme()
  close()
}
</script>

<template>
  <div ref="root" class="user-menu-root">
    <transition name="user-menu">
      <div v-if="open" class="user-menu" role="menu">
        <button class="user-menu-item" role="menuitem" @click="openSettings">
          <AppIcon name="settings" :size="16" />
          <span>Settings</span>
        </button>
        <button class="user-menu-item" role="menuitem" @click="toggleTheme">
          <AppIcon :name="theme === 'dark' ? 'sun' : 'moon'" :size="16" />
          <span>{{ theme === 'dark' ? 'Light mode' : 'Dark mode' }}</span>
        </button>
        <button class="user-menu-item" role="menuitem" @click="signOut">
          <AppIcon name="archive" :size="16" />
          <span>Sign out</span>
        </button>
      </div>
    </transition>
    <button
      type="button"
      class="user-pill"
      :class="{ 'user-pill--open': open }"
      :aria-expanded="open"
      aria-haspopup="menu"
      @click="toggle"
    >
      <PrismAvatar :size="32" />
      <span class="user-pill-text">
        <span class="user-pill-name">{{ user?.username ?? 'guest' }}</span>
        <span class="user-pill-sub">Account</span>
      </span>
    </button>
  </div>
</template>

<style scoped>
.user-menu-root {
  position: relative;
  border-top: 1px dashed var(--border);
  padding-top: 0.65rem;
  margin-top: 0.25rem;
}

.user-pill {
  width: 100%;
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.45rem 0.55rem;
  border-radius: 0.55rem;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text);
  cursor: pointer;
  text-align: left;
}

.user-pill:hover,
.user-pill--open {
  border-color: var(--border);
  background: var(--surface-hover);
}

.user-pill-text {
  display: grid;
  line-height: 1.15;
}

.user-pill-name {
  font-weight: 650;
  font-size: 0.92rem;
}

.user-pill-sub {
  color: var(--muted);
  font-size: 0.75rem;
}

.user-menu {
  position: absolute;
  bottom: calc(100% + 0.4rem);
  left: 0;
  right: 0;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 0.55rem;
  box-shadow: var(--shadow);
  padding: 0.3rem;
  display: grid;
  gap: 0.15rem;
  z-index: 20;
}

.user-menu-item {
  display: inline-flex;
  align-items: center;
  gap: 0.55rem;
  padding: 0.55rem 0.7rem;
  border-radius: 0.4rem;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text);
  cursor: pointer;
  text-align: left;
  font-size: 0.9rem;
}

.user-menu-item:hover {
  background: var(--surface-hover);
}

.user-menu-enter-active,
.user-menu-leave-active {
  transition:
    opacity 90ms ease,
    transform 90ms ease;
}

.user-menu-enter-from,
.user-menu-leave-to {
  opacity: 0;
  transform: translateY(4px);
}
</style>
