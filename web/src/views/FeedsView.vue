<script setup lang="ts">
import { ref } from 'vue'
import { storeToRefs } from 'pinia'
import AddFeedModal from '../components/AddFeedModal.vue'
import AppSidebar from '../components/AppSidebar.vue'
import PageNavTabs from '../components/PageNavTabs.vue'
import { useAppStore } from '../stores/appStore'

const { isSidebarCollapsed } = storeToRefs(useAppStore())

type Feed = { name: string; count: number }
type FeedItem = {
  title: string
  meta: string
  preview: string
  active?: boolean
}

const feeds: Feed[] = [
  { name: 'The Verge', count: 12 },
  { name: 'Hacker News', count: 53 },
  { name: 'NYT Cooking', count: 4 },
  { name: 'Stratechery', count: 1 },
  { name: 'Anthropic blog', count: 0 },
  { name: 'Local — Philly Mag', count: 7 },
]

const items: FeedItem[] = [
  { title: 'Anthropic ships Claude 4.5', meta: 'Anthropic · 2h', preview: 'Major model update with…', active: true },
  { title: 'Best gnocchi in Philly', meta: 'NYT Cooking · 4h', preview: 'We tasted 12 spots…' },
  { title: 'Show HN: tiny RSS reader', meta: 'HN · 5h', preview: 'I built this in a weekend…' },
  { title: 'EU passes new AI rules', meta: 'The Verge · 6h', preview: 'Effective in June, the…' },
  { title: 'Ask HN: how do you…', meta: 'HN · 7h', preview: 'Curious how others…' },
]

const showAddModal = ref(false)
</script>

<template>
  <div class="page" :class="{ 'page--collapsed': isSidebarCollapsed }">
    <AppSidebar v-if="!isSidebarCollapsed" />
    <section class="panel">
      <PageNavTabs />
      <div class="layout">
        <aside class="rail">
          <div class="rail-head">
            <input class="mini-search" placeholder="🔎 Filter feeds" />
            <div class="rail-actions">
              <WfBtn tiny primary @click="showAddModal = true">＋ Add feed</WfBtn>
              <WfBtn tiny>OPML</WfBtn>
            </div>
          </div>
          <div class="rail-list">
            <button class="rail-item rail-item--active">
              📥 All unread <span class="badge badge--pill">77</span>
            </button>
            <button class="rail-item">⭐ Starred <span class="muted small ms-auto">4</span></button>
            <button class="rail-item">🤖 Summarized <span class="muted small ms-auto">23</span></button>
            <hr />
            <div class="muted small section-label">FEEDS</div>
            <button v-for="f in feeds" :key="f.name" class="rail-item">
              <WfPlaceholder circle :w="14" :h="14" />
              <span class="rail-name">{{ f.name }}</span>
              <span v-if="f.count > 0" class="badge">{{ f.count }}</span>
            </button>
          </div>
        </aside>

        <aside class="items">
          <header class="items-head">
            <h2>All unread · 77</h2>
            <div class="row">
              <WfBtn tiny>↻</WfBtn>
              <WfBtn tiny>✓ All</WfBtn>
            </div>
          </header>
          <div class="items-list">
            <button
              v-for="(item, i) in items"
              :key="i"
              class="item"
              :class="{ 'item--active': item.active }"
            >
              <div class="item-row">
                <span class="item-title">{{ item.title }}</span>
                <span class="muted small">⭐</span>
              </div>
              <div class="muted small">{{ item.meta }}</div>
              <div class="item-preview">{{ item.preview }}</div>
            </button>
          </div>
        </aside>

        <main class="reader">
          <header class="reader-head">
            <h1>Anthropic ships Claude 4.5</h1>
            <div class="reader-actions">
              <WfBtn tiny>Open ↗</WfBtn>
              <WfBtn tiny>★</WfBtn>
              <WfBtn tiny>📒 Save to…</WfBtn>
            </div>
          </header>
          <div class="muted small">Anthropic blog · Apr 29 · ~6 min read</div>

          <WfBox :pad="16" fill class="summary">
            <h3>🤖 AI summary</h3>
            <ul>
              <li>New flagship "Opus" model with bigger context and improved reasoning.</li>
              <li>Available in API, Claude apps, and AWS Bedrock from today.</li>
              <li>Pricing reduced ~30% for batch workloads.</li>
            </ul>
            <div class="row">
              <WfBtn tiny>🔁 Regenerate</WfBtn>
              <WfBtn tiny>Longer</WfBtn>
              <WfBtn tiny>Bullet points</WfBtn>
              <WfBtn tiny>Just headline</WfBtn>
            </div>
          </WfBox>

          <WfBox :pad="16">
            <div class="muted small">— full article (preview) —</div>
            <WfPlaceholder :h="64" :style="{ width: '100%', marginTop: '0.4rem' }">
              article body
            </WfPlaceholder>
          </WfBox>

          <WfBox dashed :pad="12" class="batch">
            <WfTag>Batch</WfTag>
            <span class="batch-text">Summarize <b>all 53 unread HN items</b> →</span>
            <div class="spacer" />
            <WfBtn tiny primary>Run digest</WfBtn>
          </WfBox>
        </main>
      </div>
    </section>

    <AddFeedModal v-if="showAddModal" @close="showAddModal = false" @add="showAddModal = false" />
  </div>
</template>

<style scoped>
.page {
  display: grid;
  grid-template-columns: 280px 1fr;
  height: 100%;
  overflow: hidden;
  background: var(--bg);
}

.page--collapsed {
  grid-template-columns: 1fr;
}

.panel {
  display: grid;
  grid-template-rows: auto 1fr;
  overflow: hidden;
}

.layout {
  display: grid;
  grid-template-columns: 220px 280px minmax(0, 1fr);
  height: 100%;
  overflow: hidden;
}

.rail,
.items {
  border-right: 1px solid var(--border);
  display: grid;
  grid-template-rows: auto 1fr;
  overflow: hidden;
  background: var(--sidebar);
}

.rail-head {
  padding: 0.6rem;
  display: grid;
  gap: 0.4rem;
  border-bottom: 1px solid var(--border);
}

.mini-search {
  padding: 0.45rem 0.6rem;
  border-radius: 0.4rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-size: 0.82rem;
}

.mini-search:focus {
  outline: none;
  border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
}

.rail-actions {
  display: flex;
  gap: 0.35rem;
}

.rail-actions :deep(.wf-btn) {
  flex: 1;
}

.rail-list {
  overflow: auto;
  padding: 0.35rem;
  display: grid;
  gap: 0.1rem;
  align-content: start;
}

.rail-item {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.4rem 0.55rem;
  border: 1px solid transparent;
  border-radius: 0.4rem;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  text-align: left;
  font-size: 0.85rem;
}

.rail-item:hover {
  background: var(--surface-hover);
  color: var(--text);
}

.rail-item--active {
  background: var(--selected);
  color: var(--text);
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
  font-weight: 600;
}

.rail-name {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.section-label {
  padding: 0.35rem 0.55rem 0.1rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 700;
}

.badge {
  background: color-mix(in srgb, var(--primary) 18%, var(--surface));
  color: var(--primary);
  border-radius: 999px;
  padding: 0.1rem 0.45rem;
  font-size: 0.7rem;
  font-weight: 700;
}

.badge--pill {
  margin-left: auto;
}

.ms-auto {
  margin-left: auto;
}

.muted {
  color: var(--muted);
  font-size: 0.85rem;
}

.small {
  font-size: 0.76rem;
}

hr {
  border: none;
  border-top: 1px solid var(--border);
  margin: 0.4rem 0.55rem;
}

.items-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.65rem 0.85rem;
  border-bottom: 1px solid var(--border);
}

.items-head h2 {
  margin: 0;
  font-size: 0.95rem;
}

.row {
  display: flex;
  gap: 0.35rem;
}

.items-list {
  overflow: auto;
  display: grid;
  gap: 0;
  align-content: start;
}

.item {
  text-align: left;
  background: transparent;
  border: none;
  border-bottom: 1px solid var(--border);
  padding: 0.7rem 0.85rem;
  cursor: pointer;
  color: var(--text);
}

.item:hover {
  background: var(--surface-hover);
}

.item--active {
  background: var(--selected);
}

.item-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.item-title {
  font-weight: 600;
  font-size: 0.88rem;
}

.item-preview {
  font-size: 0.8rem;
  margin-top: 0.2rem;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.reader {
  padding: 1.25rem clamp(1rem, 3vw, 2rem);
  overflow: auto;
  display: grid;
  gap: 0.75rem;
  align-content: start;
  min-width: 0;
}

.reader-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.5rem;
}

.reader-head h1 {
  margin: 0;
  font-size: 1.3rem;
}

.reader-actions {
  display: flex;
  gap: 0.35rem;
}

.summary h3 {
  margin: 0 0 0.4rem;
  font-size: 1rem;
}

.summary ul {
  margin: 0 0 0.55rem;
  padding-left: 1.1rem;
  font-size: 0.88rem;
  line-height: 1.6;
}

.batch {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.batch-text {
  font-size: 0.85rem;
}

.spacer {
  flex: 1;
}

@media (max-width: 1100px) {
  .layout {
    grid-template-columns: 220px minmax(0, 1fr);
  }

  .items {
    display: none;
  }
}

@media (max-width: 800px) {
  .layout {
    grid-template-columns: 1fr;
  }

  .rail {
    display: none;
  }
}
</style>
