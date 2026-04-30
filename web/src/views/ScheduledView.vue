<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import AppSidebar from '../components/AppSidebar.vue'
import PageNavTabs from '../components/PageNavTabs.vue'
import { useAppStore } from '../stores/appStore'

const router = useRouter()
const { isSidebarCollapsed } = storeToRefs(useAppStore())

type TaskState = 'running' | 'idle' | 'paused'
type Task = {
  name: string
  schedule: string
  state: TaskState
  desc: string
  lastRun: string
}

const tasks: Task[] = [
  {
    name: 'Daily news digest',
    schedule: 'every day · 7:00am',
    state: 'running',
    desc: 'Summarizes all unread RSS items, posts to Inbox.',
    lastRun: '2h ago',
  },
  {
    name: 'Weekly notebook recap',
    schedule: 'Sundays · 6pm',
    state: 'idle',
    desc: 'Recaps notes added to "Research" this week.',
    lastRun: '3d ago',
  },
  {
    name: 'Cookbook ingest',
    schedule: 'every 2h',
    state: 'idle',
    desc: 'Watch /cooking dropbox folder, add to Cooking notebook.',
    lastRun: '47m ago',
  },
  {
    name: 'Stock — earnings watch',
    schedule: 'weekdays · 4:15pm',
    state: 'paused',
    desc: 'Pings if AAPL/NVDA report movement > 3%.',
    lastRun: 'Apr 25',
  },
]

const counts = {
  active: tasks.filter((t) => t.state === 'running').length,
  paused: tasks.filter((t) => t.state === 'paused').length,
}

function newTask() {
  router.push('/scheduled/new')
}

function stateIcon(state: TaskState) {
  if (state === 'running') return '●'
  if (state === 'paused') return '⏸'
  return '○'
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
            <h1>Scheduled tasks</h1>
            <p class="muted">Things Relay does for you, automatically</p>
          </div>
        </header>

        <div class="toolbar">
          <WfBtn primary @click="newTask">＋ New task</WfBtn>
          <WfBtn>From template</WfBtn>
          <div class="spacer" />
          <WfChip dot>{{ counts.active }} active</WfChip>
          <WfChip>{{ counts.paused }} paused</WfChip>
        </div>

        <WfBox :pad="0" class="table-wrap">
          <div class="row row--head">
            <span class="cell-state" />
            <span class="cell-name">Name</span>
            <span class="cell-sched">Schedule</span>
            <span class="cell-status">State</span>
            <span class="cell-last">Last run</span>
            <span class="cell-menu" />
          </div>
          <div v-for="task in tasks" :key="task.name" class="row" :class="`row--${task.state}`">
            <span class="cell-state">
              <span class="state-icon" :class="`state-icon--${task.state}`">{{ stateIcon(task.state) }}</span>
            </span>
            <div class="cell-name">
              <div class="task-name">{{ task.name }}</div>
              <div class="task-desc muted">{{ task.desc }}</div>
            </div>
            <span class="cell-sched mono">{{ task.schedule }}</span>
            <span class="cell-status"><WfChip :active="task.state === 'running'">{{ task.state }}</WfChip></span>
            <span class="cell-last muted">{{ task.lastRun }}</span>
            <span class="cell-menu muted">⋯</span>
          </div>
        </WfBox>
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

.toolbar {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.spacer {
  flex: 1;
}

.table-wrap {
  overflow: hidden;
}

.row {
  display: grid;
  grid-template-columns: 2rem minmax(0, 2fr) minmax(0, 1.5fr) 6rem 6rem 2rem;
  gap: 0.6rem;
  align-items: center;
  padding: 0.7rem 0.85rem;
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
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 0.55rem 0.85rem;
}

.task-name {
  font-weight: 600;
}

.task-desc {
  font-size: 0.78rem;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.82rem;
}

.state-icon {
  display: inline-grid;
  place-items: center;
  width: 1.1rem;
  height: 1.1rem;
  border-radius: 999px;
  font-size: 0.75rem;
  color: var(--muted);
}

.state-icon--running {
  color: var(--primary);
  animation: pulse 1.6s ease-in-out infinite;
}

.cell-menu {
  text-align: right;
  cursor: pointer;
}

@keyframes pulse {
  0%, 100% { opacity: 0.45; }
  50% { opacity: 1; }
}
</style>
