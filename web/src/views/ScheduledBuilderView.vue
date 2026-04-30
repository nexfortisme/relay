<script setup lang="ts">
import { ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import AppSidebar from '../components/AppSidebar.vue'
import PageNavTabs from '../components/PageNavTabs.vue'
import { useAppStore } from '../stores/appStore'

const router = useRouter()
const { isSidebarCollapsed } = storeToRefs(useAppStore())

const triggers = [
  'Every day',
  'Every hour',
  'Weekly',
  'On RSS update',
  'On file drop',
  'On webhook',
] as const
type Trigger = (typeof triggers)[number]
const activeTrigger = ref<Trigger>('Every day')

type Step = { icon: string; title: string; sub: string }
const steps: Step[] = [
  { icon: '🟢', title: 'Fetch new items', sub: 'from feeds: The Verge, NYT, HN' },
  { icon: '📒', title: 'Use notebook context', sub: 'Cooking + Research' },
  { icon: '🤖', title: 'Summarize with prompt', sub: '"…in 3 bullets…"' },
  { icon: '📤', title: 'Save to', sub: 'My Data → Daily Digests' },
  { icon: '📧', title: 'Notify me', sub: 'email' },
]

function back() {
  router.push('/scheduled')
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
            <h1>New scheduled task</h1>
            <p class="muted">Step builder</p>
          </div>
          <WfBtn ghost @click="back">← Back</WfBtn>
        </header>

        <div class="grid">
          <div class="builder">
            <WfBox :pad="16">
              <h2>1. Trigger</h2>
              <hr />
              <div class="trigger-row">
                <WfBtn
                  v-for="t in triggers"
                  :key="t"
                  tiny
                  :primary="t === activeTrigger"
                  @click="activeTrigger = t"
                >
                  {{ t }}
                </WfBtn>
              </div>
              <div class="when">
                <span class="muted small">at</span>
                <input class="input-tiny" value="07:00" />
                <span class="muted small">timezone</span>
                <input class="input-tiny wide" value="America/New York" />
              </div>
            </WfBox>

            <WfBox :pad="16">
              <h2>2. Steps</h2>
              <hr />
              <div class="steps">
                <WfBox v-for="step in steps" :key="step.title" thin :pad="8" class="step">
                  <span class="step-icon">{{ step.icon }}</span>
                  <div class="step-body">
                    <div class="step-title">{{ step.title }}</div>
                    <div class="step-sub muted">{{ step.sub }}</div>
                  </div>
                  <span class="step-handle muted">⋮⋮</span>
                </WfBox>
                <WfBox dashed :pad="8" class="step-add">＋ Add step</WfBox>
              </div>
            </WfBox>
          </div>

          <aside class="preview">
            <WfBox :pad="16" fill>
              <h2>Preview</h2>
              <hr />
              <pre class="tree">EVERY day at 07:00
└─ FETCH new from 3 feeds
   └─ WITH context: Cooking + Research
      └─ SUMMARIZE → 3 bullets
         └─ SAVE to My Data
            └─ EMAIL me</pre>
              <hr class="dashed" />
              <h3>Next 3 runs</h3>
              <p class="muted small">Apr 30 · 07:00<br />May 1 · 07:00<br />May 2 · 07:00</p>
              <div class="actions">
                <WfBtn>🧪 Dry-run</WfBtn>
                <WfBtn primary>Save &amp; activate</WfBtn>
              </div>
            </WfBox>
          </aside>
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

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
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

.grid {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr);
  gap: 1rem;
}

@media (max-width: 900px) {
  .grid {
    grid-template-columns: 1fr;
  }
}

.builder {
  display: grid;
  gap: 1rem;
  align-content: start;
}

h2 {
  margin: 0;
  font-size: 1.05rem;
}

h3 {
  margin: 0;
  font-size: 0.95rem;
}

hr {
  border: none;
  border-top: 1px solid var(--border);
  margin: 0.6rem 0;
}

hr.dashed {
  border-top: 1px dashed var(--border);
  margin: 0.85rem 0;
}

.trigger-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.when {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-top: 0.85rem;
}

.input-tiny {
  padding: 0.35rem 0.55rem;
  border-radius: 0.4rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-size: 0.82rem;
  width: 5rem;
}

.input-tiny.wide {
  width: 9rem;
}

.input-tiny:focus {
  outline: none;
  border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
}

.steps {
  display: grid;
  gap: 0.5rem;
}

.step {
  display: grid;
  grid-template-columns: 1.5rem 1fr auto;
  gap: 0.65rem;
  align-items: center;
}

.step-icon {
  font-size: 1rem;
}

.step-title {
  font-weight: 600;
  font-size: 0.9rem;
}

.step-sub {
  font-size: 0.78rem;
}

.step-handle {
  font-size: 0.85rem;
  cursor: grab;
}

.step-add {
  text-align: center;
  color: var(--muted);
  font-size: 0.85rem;
}

.preview {
  align-self: start;
}

.tree {
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.82rem;
  line-height: 1.6;
  white-space: pre;
  color: var(--text);
}

.actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.85rem;
}
</style>
