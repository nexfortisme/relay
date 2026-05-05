<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { RouterLink, useRouter } from 'vue-router'
import AppIcon from './AppIcon.vue'
import PrismLogo from './PrismLogo.vue'
import { useUiStore } from '../stores/uiStore'

const tabs = [
  { to: '/chat', label: 'Chat' },
  { to: '/notebooks', label: 'Notebooks' },
  { to: '/scheduled', label: 'Scheduled' },
  { to: '/my-data', label: 'My Data' },
  { to: '/feeds', label: 'Feeds' },
] as const

const uiStore = useUiStore()
const router = useRouter()
const { isSidebarCollapsed } = storeToRefs(uiStore)
</script>

<template>
  <nav class="page-nav-tabs" aria-label="Sub-application navigation">
    <template v-if="isSidebarCollapsed">
      <button
        class="header-logo-btn"
        title="Go home"
        aria-label="Go home"
        @click="router.push('/home')"
      >
        <PrismLogo />
      </button>
      <button
        class="expand-sidebar-btn"
        title="Expand sidebar"
        aria-label="Expand sidebar"
        @click="uiStore.toggleSidebarCollapsed"
      >
        <AppIcon name="chevron-right" />
      </button>
    </template>
    <RouterLink
      v-for="tab in tabs"
      :key="tab.to"
      :to="tab.to"
      class="page-nav-tab"
      active-class="page-nav-tab--active"
    >
      {{ tab.label }}
    </RouterLink>
  </nav>
</template>

<style scoped>
.page-nav-tabs {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.6rem 1rem;
  border-bottom: 1px solid var(--border);
  background: var(--bg);
  min-width: 0;
  overflow-x: auto;
  overscroll-behavior-x: contain;
  scrollbar-width: none;
}

.page-nav-tabs::-webkit-scrollbar {
  display: none;
}

.page-nav-tab {
  flex: 0 0 auto;
  padding: 0.4rem 0.85rem;
  border-radius: 999px;
  border: 1px solid transparent;
  color: var(--muted);
  font-size: 0.85rem;
  font-weight: 600;
  text-decoration: none;
  cursor: pointer;
}

.page-nav-tab:hover {
  color: var(--text);
  background: var(--surface-hover);
}

.page-nav-tab--active {
  color: var(--text);
  background: var(--surface);
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
}

.expand-sidebar-btn,
.header-logo-btn {
  width: 2rem;
  height: 2rem;
  border-radius: 0.45rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--muted);
  cursor: pointer;
  display: inline-grid;
  place-items: center;
  flex: 0 0 auto;
}

.expand-sidebar-btn:hover,
.header-logo-btn:hover {
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
  color: var(--text);
  background: var(--surface-hover);
}

.header-logo-btn {
  border: 1px solid color-mix(in srgb, var(--primary) 42%, var(--border));
  background: color-mix(in srgb, var(--surface) 45%, var(--primary) 12%);
  color: var(--primary);
  padding: 0;
  margin-right: 0.15rem;
}

.header-logo-btn :deep(.prism-logo) {
  --logo-size: 1.1rem;
  --logo-face-w: 0.26rem;
  --logo-face-h: 0.52rem;
}

@media (max-width: 760px) {
  .page-nav-tabs {
    gap: 0.32rem;
    padding: 0.5rem 0.72rem;
  }

  .page-nav-tab {
    padding: 0.38rem 0.7rem;
    font-size: 0.8rem;
  }
}
</style>
