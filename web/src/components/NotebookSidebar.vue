<script setup lang="ts">
import AppIcon from './AppIcon.vue'
import LogoIcon from './LogoIcon.vue'
import UserMenu from './UserMenu.vue'
import { brandLogoPalette } from '../lib/logoPalette'
import type { Notebook } from '../lib/notebooks'

defineProps<{
  notebooks: Notebook[]
  selectedNotebookId: string | null
  showingFiles: boolean
  isLoading: boolean
}>()

defineEmits<{
  create: []
  home: []
  select: [id: string]
  toggleFiles: []
}>()
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-top">
      <button class="brand-row" title="Home" aria-label="Home" @click="$emit('home')">
        <LogoIcon :size="24" :palette="brandLogoPalette" />
        <span class="brand-label">Relay</span>
      </button>
      <div class="primary-actions">
        <button class="new-nb-btn" @click="$emit('create')">
          <AppIcon name="plus" :size="16" />
          New Notebook
        </button>
        <button
          class="files-btn"
          :class="{ 'files-btn--active': showingFiles }"
          title="Toggle files"
          aria-label="Toggle files"
          @click="$emit('toggleFiles')"
        >
          <AppIcon name="file" :size="17" />
        </button>
      </div>
      <div class="section-label">Notebooks</div>
    </div>

    <div class="notebook-list">
      <div v-if="isLoading" class="list-empty">Loading&hellip;</div>
      <div v-else-if="notebooks.length === 0" class="list-empty">No notebooks yet.</div>
      <button
        v-for="nb in notebooks"
        :key="nb.id"
        class="nb-row"
        :class="{ 'nb-row--active': selectedNotebookId === nb.id }"
        @click="$emit('select', nb.id)"
      >
        <span class="nb-row-name">{{ nb.name }}</span>
        <span v-if="nb.pendingJobs > 0" class="nb-badge">{{ nb.pendingJobs }}</span>
      </button>
    </div>

    <UserMenu />
  </aside>
</template>

<style scoped>
.sidebar {
  border-right: 1px solid var(--border);
  padding: 1rem;
  background: var(--sidebar);
  display: grid;
  grid-template-rows: auto 1fr auto;
  gap: 1rem;
  overflow: hidden;
}

.sidebar-top {
  display: grid;
  gap: 0.65rem;
}

.brand-row {
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
  text-align: left;
}

.brand-label {
  font-weight: 700;
}

.primary-actions {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.45rem;
}

.new-nb-btn {
  width: 100%;
  min-height: 2.55rem;
  padding: 0.65rem 0.85rem;
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
  font-size: 0.875rem;
}

.new-nb-btn:hover {
  background: var(--primary-strong);
}

.files-btn {
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

.files-btn:hover {
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
  color: var(--text);
  background: var(--surface-hover);
}

.files-btn--active {
  border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
  color: var(--primary);
  background: color-mix(in srgb, var(--primary) 10%, var(--surface));
}

.section-label {
  font-size: 0.72rem;
  font-weight: 760;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--muted);
  padding: 0.1rem 0.1rem 0;
}

.notebook-list {
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.list-empty {
  padding: 0.4rem 0.2rem;
  font-size: 0.84rem;
  color: var(--muted);
}

.nb-row {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  width: 100%;
  padding: 0.48rem 0.62rem;
  border: 1px solid transparent;
  border-radius: 0.45rem;
  background: transparent;
  color: var(--muted);
  font-size: 0.87rem;
  text-align: left;
  cursor: pointer;
  line-height: 1.2;
}

.nb-row:hover {
  background: var(--surface);
  color: var(--text);
}

.nb-row--active {
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
  background: var(--selected);
  color: var(--text);
}

.nb-row-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.nb-badge {
  background: var(--primary);
  color: #fff;
  font-size: 0.68rem;
  font-weight: 700;
  border-radius: 999px;
  padding: 0.1rem 0.42rem;
  flex-shrink: 0;
}
</style>
