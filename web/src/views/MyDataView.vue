<script setup lang="ts">
import { ref } from 'vue'
import { storeToRefs } from 'pinia'
import AppSidebar from '../components/AppSidebar.vue'
import PageNavTabs from '../components/PageNavTabs.vue'
import { useAppStore } from '../stores/appStore'

const { isSidebarCollapsed } = storeToRefs(useAppStore())

type Item = {
  icon: string
  name: string
  kind: string
  note: string
  date: string
}

const sources = [
  { label: 'all', count: 148 },
  { label: 'uploaded', count: 92 },
  { label: 'generated', count: 56 },
] as const
const activeSource = ref<string>('all')

const types = ['📄 docs', '🖼 images', '📝 notes', '🎙 audio', '📊 data', '🌐 urls']
const books = ['Cooking', 'Research', 'Travel', '—unfiled—']
const items: Item[] = [
  { icon: '📄', name: "Monk's Cafe Yelp.pdf", kind: 'PDF', note: 'uploaded · 2.1MB', date: 'Apr 28' },
  { icon: '🖼', name: 'cartoon-desk.png', kind: 'generated', note: 'from "Cartoon desk…" chat', date: 'Apr 25' },
  { icon: '📝', name: 'daily-digest-04-29.md', kind: 'generated', note: 'task: daily news digest', date: 'today' },
  { icon: '🎙', name: 'call-with-PM.m4a', kind: 'audio', note: 'transcribed · 14MB', date: 'Apr 22' },
  { icon: '📄', name: 'EverDrive GBA PRO manual.pdf', kind: 'PDF', note: 'uploaded · 5.4MB', date: 'Apr 18' },
  { icon: '📊', name: 'restaurants-rated.csv', kind: 'generated', note: 'from "5 highest rated…"', date: 'Apr 15' },
  { icon: '🖼', name: "12 photos · Monk's", kind: 'folder', note: 'uploaded', date: 'Apr 12' },
  { icon: '📝', name: 'fermentation notes.txt', kind: 'uploaded', note: 'linked to Cooking', date: 'Apr 9' },
]
</script>

<template>
  <div class="page" :class="{ 'page--collapsed': isSidebarCollapsed }">
    <AppSidebar v-if="!isSidebarCollapsed" />
    <section class="panel">
      <PageNavTabs />
      <div class="layout">
        <aside class="filters">
          <h3>Source</h3>
          <div class="filter-group">
            <label v-for="s in sources" :key="s.label" class="filter">
              <WfCheckbox :on="s.label === activeSource" />
              <span>{{ s.label }} ({{ s.count }})</span>
            </label>
          </div>

          <hr />
          <h3>Type</h3>
          <div class="filter-group">
            <label v-for="t in types" :key="t" class="filter">
              <WfCheckbox /> <span>{{ t }}</span>
            </label>
          </div>

          <hr />
          <h3>In notebook</h3>
          <div class="filter-group">
            <label v-for="b in books" :key="b" class="filter">
              <WfCheckbox /> <span>{{ b }}</span>
            </label>
          </div>
        </aside>

        <main class="main">
          <header class="head">
            <h1>My Data</h1>
            <p class="muted">Everything you've uploaded or generated</p>
          </header>

          <div class="toolbar">
            <input class="search" placeholder="🔎 Search filename, content, tags…" />
            <WfBtn>⊞ Grid</WfBtn>
            <WfBtn primary>↑ Upload</WfBtn>
          </div>

          <WfBox :pad="0" class="table-wrap">
            <div class="row row--head">
              <span class="cell-icon" />
              <span class="cell-name">Name</span>
              <span class="cell-kind">Kind</span>
              <span class="cell-note">Note</span>
              <span class="cell-date">Added</span>
            </div>
            <div v-for="it in items" :key="it.name" class="row">
              <span class="cell-icon">{{ it.icon }}</span>
              <span class="cell-name">{{ it.name }}</span>
              <span class="cell-kind"><WfChip>{{ it.kind }}</WfChip></span>
              <span class="cell-note muted">{{ it.note }}</span>
              <span class="cell-date muted">{{ it.date }}</span>
            </div>
          </WfBox>

          <div class="footnote">
            <span class="muted small">Showing 8 of 148</span>
            <span class="muted small">2.4 GB used · 7.6 GB free</span>
          </div>
        </main>
      </div>
    </section>
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
  grid-template-columns: 200px minmax(0, 1fr);
  height: 100%;
  overflow: hidden;
}

.filters {
  border-right: 1px solid var(--border);
  padding: 1rem 0.85rem;
  overflow: auto;
  display: grid;
  gap: 0.4rem;
  align-content: start;
  background: var(--sidebar);
}

.filters h3 {
  margin: 0;
  font-size: 0.78rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted);
  font-weight: 700;
}

.filter-group {
  display: grid;
  gap: 0.3rem;
}

.filter {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.82rem;
  cursor: pointer;
}

hr {
  border: none;
  border-top: 1px solid var(--border);
  margin: 0.4rem 0;
}

.main {
  padding: 1.5rem clamp(1rem, 3vw, 2rem);
  overflow: auto;
  display: grid;
  gap: 1rem;
  align-content: start;
  min-width: 0;
}

.head h1 {
  margin: 0 0 0.25rem;
  font-size: 1.4rem;
}

.muted {
  color: var(--muted);
  font-size: 0.85rem;
}

.small {
  font-size: 0.78rem;
}

.toolbar {
  display: flex;
  gap: 0.5rem;
}

.search {
  flex: 1;
  padding: 0.6rem 0.85rem;
  border-radius: 0.55rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-size: 0.9rem;
}

.search:focus {
  outline: none;
  border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
}

.table-wrap {
  overflow: hidden;
}

.row {
  display: grid;
  grid-template-columns: 1.75rem minmax(0, 2fr) 6rem minmax(0, 2fr) 5rem;
  gap: 0.6rem;
  align-items: center;
  padding: 0.65rem 0.85rem;
  border-bottom: 1px solid var(--border);
  font-size: 0.88rem;
}

.row:last-child {
  border-bottom: none;
}

.row--head {
  background: var(--surface-soft);
  font-weight: 700;
  color: var(--muted);
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.cell-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.footnote {
  display: flex;
  justify-content: space-between;
}

@media (max-width: 800px) {
  .layout {
    grid-template-columns: 1fr;
  }

  .filters {
    display: none;
  }
}
</style>
