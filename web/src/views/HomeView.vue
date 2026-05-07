<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import AppIcon from '../components/AppIcon.vue'
import AppSidebar from '../components/AppSidebar.vue'
import PrismLogo from '../components/PrismLogo.vue'
import { useUiStore } from '../stores/uiStore'
import { useChatStore } from '../stores/chatStore'
import { listFeeds, type Feed } from '../lib/api'

const uiStore = useUiStore()
const chatStore = useChatStore()
const router = useRouter()
const { isSidebarCollapsed, theme } = storeToRefs(uiStore)

const launcherDraft = ref('')
const feeds = ref<Feed[]>([])
const feedsLoading = ref(false)

const sortedFeeds = computed(() =>
  [...feeds.value].sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
)
const recentFeeds = computed(() => sortedFeeds.value.slice(0, 3))
const hasMore = computed(() => feeds.value.length > 3)

onMounted(async () => {
  feedsLoading.value = true
  try {
    feeds.value = await listFeeds()
  } finally {
    feedsLoading.value = false
  }
})

async function startNewChat() {
  const trimmed = launcherDraft.value.trim()
  if (trimmed) {
    await chatStore.handleCreateConversation()
    chatStore.setDraft(launcherDraft.value)
  } else {
    await chatStore.goHome()
  }
  router.push('/chat')
}
</script>

<template>
  <div
    class="home-view"
    :class="{ 'home-view--sidebar-collapsed': isSidebarCollapsed }"
    :data-theme="theme"
  >
    <AppSidebar v-if="!isSidebarCollapsed" />
    <button
      v-if="!isSidebarCollapsed"
      class="mobile-sidebar-backdrop"
      type="button"
      aria-label="Close sidebar"
      @click="uiStore.toggleSidebarCollapsed"
    />

    <main class="home-main">
      <div v-if="isSidebarCollapsed" class="home-collapsed-controls">
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
      </div>

      <header class="home-header">
        <div class="home-title">Relay</div>
        <div class="home-subtitle">dashboard</div>
      </header>

      <section class="launcher">
        <div class="launcher-prompt">
          <h2>What do you want to do?</h2>
          <p>Type a question, or pick a starting place.</p>
        </div>
        <div class="launcher-actions">
          <button class="launcher-tile" disabled title="Coming soon">
            <AppIcon name="file" :size="20" />
            <span>Notebook</span>
          </button>
          <button class="launcher-tile" @click="router.push('/feeds')">
            <AppIcon name="send" :size="20" />
            <span>Feeds</span>
          </button>
          <button class="launcher-tile" disabled title="Coming soon">
            <AppIcon name="clock" :size="20" />
            <span>Task</span>
          </button>
          <button class="launcher-tile launcher-tile--primary" @click="startNewChat">
            <AppIcon name="plus" :size="20" />
            <span>New chat</span>
          </button>
        </div>
        <form class="launcher-input-row" @submit.prevent="startNewChat">
          <input
            v-model="launcherDraft"
            class="launcher-input"
            type="text"
            placeholder="Summarize the new RSS items from this morning..."
          />
          <button type="submit" class="launcher-submit">
            <AppIcon name="send" :size="16" />
          </button>
        </form>
      </section>

      <div class="home-grid">
        <section class="home-card">
          <div class="home-card-header">
            <h3>Notebooks</h3>
            <span class="muted">placeholder</span>
          </div>
          <div class="placeholder-grid">
            <div class="placeholder-card">Cooking</div>
            <div class="placeholder-card">Game manuals</div>
            <div class="placeholder-card">Research papers</div>
            <div class="placeholder-card">Travel</div>
          </div>
        </section>
        <section class="home-card">
          <div class="home-card-header">
            <h3>Today's feed digest</h3>
            <button v-if="hasMore" class="see-more-btn" @click="router.push('/feeds')">
              See more
            </button>
          </div>
          <div v-if="feedsLoading" class="muted feed-loading">Loading…</div>
          <ul v-else-if="recentFeeds.length > 0" class="feed-list">
            <li
              v-for="feed in recentFeeds"
              :key="feed.id"
              class="feed-list-item"
              @click="router.push({ name: 'feeds', query: { feedId: feed.id } })"
            >
              <span class="feed-list-title">{{ feed.title }}</span>
              <span :class="feed.unreadCount > 0 ? 'feed-unread-count' : 'muted'">
                {{ feed.unreadCount }} new
              </span>
            </li>
          </ul>
          <p v-else class="feed-empty">
            No feeds yet.
            <button class="feed-empty-link" @click="router.push('/feeds')">Add one</button>
          </p>
        </section>
      </div>
    </main>
  </div>
</template>

<style scoped>
.home-view {
  display: grid;
  grid-template-columns: 280px 1fr;
  height: 100%;
  overflow: hidden;
  background: var(--bg);
  position: relative;
}

.home-view.home-view--sidebar-collapsed {
  grid-template-columns: 1fr;
}

.home-main {
  overflow: auto;
  padding: 1.75rem clamp(1.25rem, 4vw, 3rem);
  display: grid;
  gap: 1.5rem;
  align-content: start;
}

.home-collapsed-controls {
  display: flex;
  gap: 0.4rem;
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
}

.header-logo-btn :deep(.prism-logo) {
  --logo-size: 1.1rem;
  --logo-face-w: 0.26rem;
  --logo-face-h: 0.52rem;
}

.home-header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}

.home-title {
  font-size: 1.4rem;
  font-weight: 700;
}

.home-subtitle {
  color: var(--muted);
  font-size: 0.85rem;
}

.launcher {
  border: 1px solid color-mix(in srgb, var(--primary) 30%, var(--border));
  background: color-mix(in srgb, var(--surface) 85%, var(--primary) 6%);
  border-radius: 0.85rem;
  padding: 1.5rem;
  display: grid;
  gap: 1rem;
}

.launcher-prompt h2 {
  margin: 0 0 0.25rem;
  font-size: 1.2rem;
}

.launcher-prompt p {
  margin: 0;
  color: var(--muted);
  font-size: 0.9rem;
}

.launcher-actions {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 0.65rem;
}

.launcher-tile {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
  padding: 1rem 0.75rem;
  border-radius: 0.65rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-weight: 600;
  cursor: pointer;
  font-size: 0.9rem;
}

.launcher-tile:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.launcher-tile:not(:disabled):hover {
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
  background: var(--surface-hover);
}

.launcher-tile--primary {
  background: var(--primary);
  color: #fff;
  border-color: color-mix(in srgb, var(--primary) 60%, transparent);
}

.launcher-tile--primary:not(:disabled):hover {
  background: var(--primary-strong);
}

.launcher-input-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 0.5rem;
}

.launcher-input {
  padding: 0.75rem 0.9rem;
  border-radius: 0.55rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-size: 0.95rem;
}

.launcher-input:focus {
  outline: none;
  border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
}

.launcher-submit {
  width: 2.6rem;
  border-radius: 0.55rem;
  border: 1px solid color-mix(in srgb, var(--primary) 45%, transparent);
  background: var(--primary);
  color: #fff;
  cursor: pointer;
  display: inline-grid;
  place-items: center;
}

.home-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 280px), 1fr));
  gap: 1rem;
}

.home-card {
  border: 1px solid var(--border);
  background: var(--surface);
  border-radius: 0.75rem;
  padding: 1rem 1.1rem;
  display: grid;
  gap: 0.75rem;
  align-content: start;
}

.home-card-header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}

.home-card-header h3 {
  margin: 0;
  font-size: 1rem;
}

.muted {
  color: var(--muted);
  font-size: 0.78rem;
}

.placeholder-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 0.5rem;
}

.placeholder-card {
  border: 1px dashed var(--border);
  border-radius: 0.5rem;
  padding: 0.85rem 0.7rem;
  font-size: 0.85rem;
  color: var(--muted);
  background: var(--surface-soft);
}

.placeholder-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.4rem;
}

.placeholder-list li {
  display: flex;
  justify-content: space-between;
  font-size: 0.88rem;
  padding: 0.4rem 0;
  border-bottom: 1px solid var(--border);
}

.placeholder-list li:last-child {
  border-bottom: none;
}

.see-more-btn {
  background: none;
  border: none;
  padding: 0;
  color: var(--primary);
  font-size: 0.78rem;
  cursor: pointer;
  font-weight: 500;
}

.see-more-btn:hover {
  text-decoration: underline;
}

.feed-loading {
  font-size: 0.88rem;
}

.feed-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.4rem;
}

.feed-list-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.88rem;
  padding: 0.4rem 0;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
}

.feed-list-item:last-child {
  border-bottom: none;
}

.feed-list-item:hover .feed-list-title {
  color: var(--primary);
}

.feed-list-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
  flex: 1;
}

.feed-unread-count {
  flex-shrink: 0;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--primary);
  margin-left: 0.5rem;
}

.feed-empty {
  margin: 0;
  font-size: 0.88rem;
  color: var(--muted);
}

.feed-empty-link {
  background: none;
  border: none;
  padding: 0;
  color: var(--primary);
  cursor: pointer;
  font-size: inherit;
  font-weight: 500;
}

.feed-empty-link:hover {
  text-decoration: underline;
}

.mobile-sidebar-backdrop {
  display: none;
}

@media (max-width: 760px) {
  .home-view {
    grid-template-columns: 1fr;
  }

  .home-view:not(.home-view--sidebar-collapsed) :deep(.home-sidebar) {
    position: absolute;
    inset: 0 auto 0 0;
    width: min(20rem, 86vw);
    z-index: 40;
    box-sizing: border-box;
    box-shadow: var(--shadow);
  }

  .mobile-sidebar-backdrop {
    position: absolute;
    inset: 0;
    z-index: 30;
    display: block;
    border: 0;
    background: rgba(4, 9, 20, 0.52);
    cursor: pointer;
  }

  .home-main {
    min-width: 0;
    padding: 1rem clamp(0.85rem, 4vw, 1.25rem);
    gap: 1rem;
  }

  .home-header {
    align-items: flex-start;
    gap: 0.5rem;
  }

  .launcher {
    padding: 1rem;
    border-radius: 0.7rem;
  }

  .launcher-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .launcher-tile {
    min-width: 0;
    padding: 0.85rem 0.55rem;
  }

  .placeholder-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 420px) {
  .home-header {
    display: grid;
  }

  .launcher-actions {
    grid-template-columns: 1fr;
  }
}
</style>
