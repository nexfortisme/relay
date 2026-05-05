<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { RouterLink, useRouter } from 'vue-router'
import AppIcon from './AppIcon.vue'
import PrismLogo from './PrismLogo.vue'
import UserMenu from './UserMenu.vue'
import { useAppStore } from '../stores/appStore'

const appStore = useAppStore()
const router = useRouter()
const { conversations } = storeToRefs(appStore)
const activeConversations = computed(() =>
  conversations.value.filter((conversation) => !conversation.archived),
)

type NavItem = {
  to: string
  label: string
  icon: 'sparkles' | 'file' | 'clock' | 'archive' | 'send'
  enabled: boolean
}

const navItems: NavItem[] = [
  { to: '/', label: 'Home', icon: 'sparkles', enabled: true },
  { to: '/chat', label: 'Chat', icon: 'sparkles', enabled: true },
  { to: '/notebooks', label: 'Notebooks', icon: 'file', enabled: false },
  { to: '/scheduled', label: 'Scheduled', icon: 'clock', enabled: false },
  { to: '/my-data', label: 'My Data', icon: 'archive', enabled: false },
  { to: '/feeds', label: 'Feeds', icon: 'send', enabled: true },
]

async function startNewChat() {
  await appStore.goHome()
  router.push('/chat')
}

function selectConversation(conversationId: string) {
  appStore.selectConversation(conversationId)
  router.push('/chat')
}
</script>

<template>
  <aside class="home-sidebar">
    <div class="home-sidebar-header">
      <button class="home-logo" @click="router.push('/')">
        <PrismLogo />
        <span>Relay</span>
      </button>
      <div class="primary-row">
        <button class="primary-btn" @click="startNewChat">
          <AppIcon name="plus" :size="16" />
          New Chat
        </button>
        <button
          class="collapse-btn"
          title="Collapse sidebar"
          aria-label="Collapse sidebar"
          @click="appStore.toggleSidebarCollapsed"
        >
          <AppIcon name="chevron-left" />
        </button>
      </div>
    </div>

    <nav class="home-nav" aria-label="Sub-applications">
      <template v-for="item in navItems" :key="item.to">
        <RouterLink
          v-if="item.enabled"
          :to="item.to"
          class="home-nav-item"
          active-class="home-nav-item--active"
        >
          <AppIcon :name="item.icon" :size="16" />
          <span>{{ item.label }}</span>
        </RouterLink>
        <button v-else class="home-nav-item home-nav-item--disabled" disabled>
          <AppIcon :name="item.icon" :size="16" />
          <span>{{ item.label }}</span>
          <span class="badge">soon</span>
        </button>
      </template>
    </nav>

    <div class="recent-section">
      <div class="recent-title">Recent chats</div>
      <button
        v-for="conversation in activeConversations.slice(0, 5)"
        :key="conversation.id"
        class="recent-item"
        @click="selectConversation(conversation.id)"
      >
        {{ conversation.title }}
      </button>
      <p v-if="activeConversations.length === 0" class="recent-empty">
        No chats yet — start one with the launcher.
      </p>
    </div>

    <UserMenu />
  </aside>
</template>

<style scoped>
.home-sidebar {
  border-right: 1px solid var(--border);
  background: var(--sidebar);
  padding: 1rem;
  display: grid;
  grid-template-rows: auto 1fr auto auto;
  gap: 1rem;
  overflow: hidden;
}

.home-sidebar-header {
  display: grid;
  gap: 0.6rem;
}

.home-logo {
  display: inline-flex;
  align-items: center;
  gap: 0.55rem;
  border: none;
  background: transparent;
  color: var(--text);
  font-weight: 700;
  font-size: 1rem;
  cursor: pointer;
  padding: 0.25rem 0.1rem;
}

.home-logo :deep(.prism-logo) {
  --logo-size: 1.45rem;
  --logo-face-w: 0.34rem;
  --logo-face-h: 0.7rem;
}

.primary-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.45rem;
}

.collapse-btn {
  min-width: 2.55rem;
  min-height: 2.55rem;
  border-radius: 0.5rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--muted);
  cursor: pointer;
  display: inline-grid;
  place-items: center;
  padding: 0;
}

.collapse-btn:hover {
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
  color: var(--text);
  background: var(--surface-hover);
}

.primary-btn {
  width: 100%;
  min-height: 2.55rem;
  border-radius: 0.5rem;
  border: 1px solid color-mix(in srgb, var(--primary) 45%, transparent);
  background: var(--primary);
  color: #fff;
  font-weight: 650;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
}

.primary-btn:hover {
  background: var(--primary-strong);
}

.home-nav {
  display: grid;
  gap: 0.25rem;
  align-content: start;
}

.home-nav-item {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.55rem 0.7rem;
  border-radius: 0.45rem;
  text-decoration: none;
  border: 1px solid transparent;
  background: transparent;
  color: var(--muted);
  font-size: 0.9rem;
  cursor: pointer;
  text-align: left;
  width: 100%;
}

.home-nav-item:hover:not(:disabled) {
  background: var(--surface-hover);
  color: var(--text);
}

.home-nav-item--active {
  background: var(--selected);
  color: var(--text);
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
}

.home-nav-item--disabled {
  cursor: not-allowed;
  opacity: 0.65;
}

.badge {
  margin-left: auto;
  font-size: 0.65rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding: 0.1rem 0.4rem;
  border-radius: 999px;
  background: var(--surface-soft);
  color: var(--muted);
}

.recent-section {
  display: grid;
  gap: 0.2rem;
  align-content: end;
}

.recent-title {
  padding: 0.35rem 0.55rem 0.15rem;
  font-size: 0.72rem;
  color: var(--muted);
  text-transform: uppercase;
  font-weight: 700;
}

.recent-item {
  text-align: left;
  padding: 0.45rem 0.6rem;
  border-radius: 0.4rem;
  border: 1px solid transparent;
  background: transparent;
  color: var(--muted);
  font-size: 0.85rem;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.recent-item:hover {
  background: var(--surface-hover);
  color: var(--text);
}

.recent-empty {
  padding: 0.4rem 0.6rem;
  font-size: 0.8rem;
  color: var(--muted);
  margin: 0;
}

</style>
