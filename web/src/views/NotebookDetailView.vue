<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute } from 'vue-router'
import AppSidebar from '../components/AppSidebar.vue'
import PageNavTabs from '../components/PageNavTabs.vue'
import { useAppStore } from '../stores/appStore'

const route = useRoute()
const { isSidebarCollapsed } = storeToRefs(useAppStore())

const notebookName = computed(() => decodeURIComponent(String(route.params.id ?? 'Notebook')))

type Doc = { file: string; type: string }
const docs: Doc[] = [
  { file: "Monk's Cafe Yelp.pdf", type: 'PDF' },
  { file: 'vegan cheesesteak recipe.md', type: 'MD' },
  { file: 'fermentation notes', type: 'TXT' },
  { file: 'restaurant photos (12)', type: 'IMG' },
  { file: 'Bon Appétit · gnocchi', type: 'URL' },
  { file: 'stovetop tips.md', type: 'MD' },
]

const sources = [
  { n: 1, title: "Monk's Yelp.pdf", quote: '"…the best vegan cheesesteak I\'ve ever had."' },
  { n: 2, title: "Monk's Yelp.pdf", quote: '"Mussels & frites — perfectly briny."' },
  { n: 3, title: "Monk's Yelp.pdf", quote: '"Bruges Burger — comfort done right."' },
]
</script>

<template>
  <div class="page" :class="{ 'page--collapsed': isSidebarCollapsed }">
    <AppSidebar v-if="!isSidebarCollapsed" />
    <section class="panel">
      <PageNavTabs />
      <div class="detail">
        <aside class="docs">
          <div class="docs-head">
            <input class="mini-search" placeholder="🔎 in this notebook" />
            <WfBtn tiny>＋</WfBtn>
          </div>
          <div class="docs-list">
            <button
              v-for="(doc, i) in docs"
              :key="doc.file"
              class="doc-row"
              :class="{ 'doc-row--active': i === 0 }"
            >
              <span class="doc-type">{{ doc.type }}</span>
              <span class="doc-name">{{ doc.file }}</span>
            </button>
          </div>
        </aside>

        <main class="thread">
          <header class="thread-head">
            <div>
              <h1>📒 {{ notebookName }}</h1>
              <p class="muted">{{ docs.length }} docs · last edited today</p>
            </div>
          </header>

          <div class="messages">
            <WfBox :pad="12" class="msg msg--user">
              <div class="muted small">You</div>
              <div>What's the consensus best dish at Monk's?</div>
            </WfBox>
            <WfBox :pad="12" fill class="msg msg--ai">
              <div class="muted small">Relay · 4 sources</div>
              <p>
                Reviewers consistently call out the <b>Seitan Cheesesteak</b>
                <WfChip>1</WfChip> and <b>Gent Mussels &amp; Frites</b>
                <WfChip>2</WfChip>. The <i>Bruges Burger</i>
                <WfChip>3</WfChip> is also a popular pick.
              </p>
            </WfBox>
          </div>

          <footer class="composer">
            <WfBox :pad="8" class="composer-row">
              <WfBtn tiny>📎</WfBtn>
              <input class="composer-input" placeholder="Ask within this notebook…" />
              <WfBtn tiny primary>Send</WfBtn>
            </WfBox>
          </footer>
        </main>

        <aside class="sources">
          <h3>Sources used</h3>
          <WfBox v-for="src in sources" :key="src.n" thin :pad="8" class="source-card">
            <div class="src-title">[{{ src.n }}] {{ src.title }}</div>
            <div class="src-quote muted">{{ src.quote }}</div>
          </WfBox>
          <div class="suggested">
            <div class="muted small">Suggested</div>
            <div class="chips">
              <WfChip>Summarize this notebook</WfChip>
              <WfChip>Compare to Vedge</WfChip>
            </div>
          </div>
        </aside>
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

.detail {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr) 240px;
  height: 100%;
  overflow: hidden;
}

.docs {
  border-right: 1px solid var(--border);
  display: grid;
  grid-template-rows: auto 1fr;
  overflow: hidden;
  background: var(--sidebar);
}

.docs-head {
  display: flex;
  gap: 0.4rem;
  padding: 0.55rem;
  border-bottom: 1px solid var(--border);
}

.mini-search {
  flex: 1;
  min-width: 0;
  padding: 0.4rem 0.55rem;
  border-radius: 0.4rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-size: 0.8rem;
}

.mini-search:focus {
  outline: none;
  border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
}

.docs-list {
  overflow: auto;
  padding: 0.35rem;
  display: grid;
  gap: 0.15rem;
  align-content: start;
}

.doc-row {
  display: grid;
  grid-template-columns: 2rem 1fr;
  gap: 0.4rem;
  align-items: center;
  padding: 0.4rem 0.5rem;
  border: 1px solid transparent;
  border-radius: 0.4rem;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  text-align: left;
  font-size: 0.82rem;
}

.doc-row:hover {
  background: var(--surface-hover);
  color: var(--text);
}

.doc-row--active {
  background: var(--selected);
  color: var(--text);
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
}

.doc-type {
  font-size: 0.7rem;
  font-weight: 700;
  color: var(--primary);
}

.doc-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.thread {
  display: grid;
  grid-template-rows: auto 1fr auto;
  overflow: hidden;
  min-width: 0;
}

.thread-head {
  padding: 0.85rem 1.1rem;
  border-bottom: 1px solid var(--border);
}

.thread-head h1 {
  margin: 0 0 0.15rem;
  font-size: 1.1rem;
}

.muted {
  color: var(--muted);
  font-size: 0.85rem;
}

.small {
  font-size: 0.78rem;
}

.messages {
  overflow: auto;
  padding: 1rem 1.1rem;
  display: grid;
  gap: 0.65rem;
  align-content: start;
}

.msg p {
  margin: 0.3rem 0 0;
  line-height: 1.55;
  font-size: 0.92rem;
}

.composer {
  padding: 0.75rem 1.1rem;
  border-top: 1px solid var(--border);
}

.composer-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.composer-input {
  flex: 1;
  border: none;
  background: transparent;
  color: var(--text);
  font-size: 0.9rem;
  outline: none;
}

.composer-input::placeholder {
  color: var(--muted);
}

.sources {
  border-left: 1px solid var(--border);
  padding: 0.85rem;
  overflow: auto;
  display: grid;
  gap: 0.55rem;
  align-content: start;
  background: var(--sidebar);
}

.sources h3 {
  margin: 0 0 0.25rem;
  font-size: 0.95rem;
}

.source-card {
  display: grid;
  gap: 0.2rem;
}

.src-title {
  font-weight: 700;
  font-size: 0.78rem;
}

.src-quote {
  font-size: 0.78rem;
  font-style: italic;
}

.suggested {
  display: grid;
  gap: 0.35rem;
  margin-top: 0.4rem;
  padding-top: 0.4rem;
  border-top: 1px dashed var(--border);
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

@media (max-width: 1024px) {
  .detail {
    grid-template-columns: 1fr;
    grid-template-rows: auto 1fr;
  }

  .docs,
  .sources {
    display: none;
  }
}
</style>
