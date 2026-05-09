<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import AppIcon from '../components/AppIcon.vue'
import AppSidebar from '../components/AppSidebar.vue'
import LogoIcon from '../components/LogoIcon.vue'
import { brandLogoPalette } from '../lib/logoPalette'
import { useUiStore } from '../stores/uiStore'
import { useChatStore } from '../stores/chatStore'
import { useNotebookStore } from '../stores/notebookStore'
import { listFeeds, type Feed } from '../lib/api'

const uiStore = useUiStore()
const chatStore = useChatStore()
const notebookStore = useNotebookStore()
const router = useRouter()
const { isSidebarCollapsed, theme } = storeToRefs(uiStore)
const { notebooks, isLoading: notebooksLoading, error: notebooksError } = storeToRefs(notebookStore)

const launcherDraft = ref('')
const feeds = ref<Feed[]>([])
const feedsLoading = ref(false)

const sortedFeeds = computed(() =>
  [...feeds.value].sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
)
const recentFeeds = computed(() => sortedFeeds.value.slice(0, 3))
const hasMore = computed(() => feeds.value.length > 3)
const sortedNotebooks = computed(() =>
  [...notebooks.value].sort((a, b) => dateMillis(b.updatedAt) - dateMillis(a.updatedAt)),
)
const notebookCountLabel = computed(() => {
  const count = notebooks.value.length
  return `${count} notebook${count === 1 ? '' : 's'}`
})

onMounted(async () => {
  await Promise.all([loadHomeFeeds(), notebookStore.loadNotebooks()])
})

async function loadHomeFeeds() {
  feedsLoading.value = true
  try {
    feeds.value = await listFeeds()
  } finally {
    feedsLoading.value = false
  }
}

function dateMillis(iso: string): number {
  const time = new Date(iso).getTime()
  return Number.isNaN(time) ? 0 : time
}

function formatShortDate(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' }).format(date)
}

function pendingJobsLabel(count: number): string {
  return `${count} indexing ${count === 1 ? 'job' : 'jobs'}`
}

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
          <LogoIcon :size="18" :palette="brandLogoPalette" />
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
            <div v-if="notebooks.length > 0" class="home-card-actions">
              <span class="muted">{{ notebookCountLabel }}</span>
              <button class="see-more-btn" @click="router.push('/notebooks')">Open</button>
            </div>
          </div>
          <div v-if="notebooksLoading" class="muted notebook-loading">Loading…</div>
          <p v-else-if="notebooksError" class="notebook-empty notebook-error">
            {{ notebooksError }}
          </p>
          <ul v-else-if="sortedNotebooks.length > 0" class="notebook-list">
            <li v-for="notebook in sortedNotebooks" :key="notebook.id" class="notebook-list-item">
              <div class="notebook-list-main">
                <span class="notebook-list-title">{{ notebook.name }}</span>
                <span class="notebook-list-description">
                  {{ notebook.description || `Updated ${formatShortDate(notebook.updatedAt)}` }}
                </span>
              </div>
              <span v-if="notebook.pendingJobs > 0" class="notebook-pending">
                {{ pendingJobsLabel(notebook.pendingJobs) }}
              </span>
              <span v-else class="notebook-date">{{ formatShortDate(notebook.updatedAt) }}</span>
            </li>
          </ul>
          <p v-else class="notebook-empty">
            No notebooks yet.
            <button class="notebook-empty-link" @click="router.push('/notebooks')">
              Create one
            </button>
          </p>
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

.home-card-actions {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
  flex-shrink: 0;
}

.muted {
  color: var(--muted);
  font-size: 0.78rem;
}

.notebook-loading {
  font-size: 0.88rem;
}

.notebook-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0;
  max-height: 18rem;
  overflow-y: auto;
}

.notebook-list-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.88rem;
  padding: 0.55rem 0;
  border-bottom: 1px solid var(--border);
}

.notebook-list-item:last-child {
  border-bottom: none;
}

.notebook-list-main {
  min-width: 0;
  display: grid;
  gap: 0.18rem;
}

.notebook-list-title,
.notebook-list-description {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.notebook-list-title {
  color: var(--text);
  font-weight: 650;
}

.notebook-list-description,
.notebook-date {
  color: var(--muted);
  font-size: 0.78rem;
}

.notebook-pending {
  flex-shrink: 0;
  border-radius: 999px;
  background: color-mix(in srgb, var(--primary) 12%, transparent);
  color: var(--primary);
  font-size: 0.72rem;
  font-weight: 650;
  padding: 0.18rem 0.45rem;
  white-space: nowrap;
}

.notebook-empty {
  margin: 0;
  font-size: 0.88rem;
  color: var(--muted);
}

.notebook-error {
  color: var(--danger);
}

.notebook-empty-link {
  background: none;
  border: none;
  padding: 0;
  color: var(--primary);
  cursor: pointer;
  font-size: inherit;
  font-weight: 500;
}

.notebook-empty-link:hover {
  text-decoration: underline;
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

  .notebook-list-item {
    grid-template-columns: 1fr;
    gap: 0.3rem;
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
