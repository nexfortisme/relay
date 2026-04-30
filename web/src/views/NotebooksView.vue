<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import AppSidebar from '../components/AppSidebar.vue'
import PageNavTabs from '../components/PageNavTabs.vue'
import { useAppStore } from '../stores/appStore'

const router = useRouter()
const { isSidebarCollapsed } = storeToRefs(useAppStore())

type Notebook = { name: string; count: string; sub: string }

const books: Notebook[] = [
  { name: 'Cooking', count: '24 docs', sub: 'PDFs · recipes · screenshots' },
  { name: 'Game manuals', count: '9 docs', sub: 'GBA · DS · misc roms' },
  { name: 'Research', count: '47 docs', sub: 'arXiv papers, notes' },
  { name: 'Travel', count: '12 docs', sub: 'itineraries, restaurants' },
  { name: 'Onboarding', count: '5 docs', sub: 'company wiki snapshots' },
  { name: 'Misc', count: '3 docs', sub: '' },
]

function open(name: string) {
  router.push(`/notebooks/${encodeURIComponent(name)}`)
}

function newNotebook() {
  router.push('/notebooks/new')
}
</script>

<template>
  <div class="page" :class="{ 'page--collapsed': isSidebarCollapsed }">
    <AppSidebar v-if="!isSidebarCollapsed" />
    <section class="panel">
      <PageNavTabs />
      <div class="body">
        <header class="header">
          <div>
            <h1>Notebooks</h1>
            <p class="muted">Grouped collections of docs</p>
          </div>
        </header>

        <div class="toolbar">
          <input class="search" placeholder="Search notebooks &amp; documents…" />
          <WfBtn>↕ Sort</WfBtn>
          <WfBtn primary @click="newNotebook">＋ New notebook</WfBtn>
        </div>

        <div class="grid">
          <WfBox v-for="book in books" :key="book.name" :pad="16" class="card">
            <div class="card-head">
              <h3>📒 {{ book.name }}</h3>
              <span class="muted">⋯</span>
            </div>
            <div class="muted small">{{ book.count }}</div>
            <p class="card-sub">{{ book.sub || '—' }}</p>
            <div class="card-actions">
              <WfBtn tiny @click="open(book.name)">Open</WfBtn>
              <WfBtn tiny @click="open(book.name)">Chat with</WfBtn>
            </div>
          </WfBox>
          <WfBox dashed :pad="16" class="card card--new">
            <button class="add-btn" @click="newNotebook">＋ New notebook</button>
          </WfBox>
        </div>
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

.body {
  padding: 1.5rem clamp(1.25rem, 4vw, 3rem);
  overflow: auto;
  display: grid;
  gap: 1.25rem;
  align-content: start;
}

.header h1 {
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
  align-items: center;
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

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 0.85rem;
}

.card {
  display: grid;
  gap: 0.4rem;
  align-content: start;
}

.card-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}

.card-head h3 {
  margin: 0;
  font-size: 1.05rem;
}

.card-sub {
  margin: 0.2rem 0 0.4rem;
  font-size: 0.85rem;
  color: var(--text);
  min-height: 1.4em;
}

.card-actions {
  display: flex;
  gap: 0.4rem;
  margin-top: 0.3rem;
}

.card--new {
  display: grid;
  place-items: center;
  min-height: 8rem;
}

.add-btn {
  border: none;
  background: transparent;
  color: var(--muted);
  font-size: 0.95rem;
  cursor: pointer;
  font-weight: 600;
}

.add-btn:hover {
  color: var(--text);
}
</style>
