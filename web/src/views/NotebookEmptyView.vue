<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import AppSidebar from '../components/AppSidebar.vue'
import PageNavTabs from '../components/PageNavTabs.vue'
import { useAppStore } from '../stores/appStore'

const router = useRouter()
const { isSidebarCollapsed } = storeToRefs(useAppStore())
</script>

<template>
  <div class="page" :class="{ 'page--collapsed': isSidebarCollapsed }">
    <AppSidebar v-if="!isSidebarCollapsed" />
    <section class="panel">
      <PageNavTabs />
      <div class="body">
        <header class="header">
          <h1>📒 New notebook</h1>
          <p class="muted">Empty state</p>
        </header>

        <WfBox dashed :pad="20" class="dropzone">
          <h2>Drop files to start</h2>
          <p class="muted">PDFs · markdown · text · images · URLs</p>
          <div class="actions">
            <WfBtn>📎 Choose files</WfBtn>
            <WfBtn>🔗 Paste URL</WfBtn>
            <WfBtn>✎ New blank doc</WfBtn>
          </div>
        </WfBox>

        <p class="muted center">
          or pick from
          <a href="#" class="link" @click.prevent="router.push('/my-data')">My Data</a>
          — files you've already uploaded
        </p>
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
  padding: 2rem clamp(1.25rem, 4vw, 3rem);
  overflow: auto;
  display: grid;
  gap: 1.25rem;
  align-content: start;
  justify-items: stretch;
}

.header h1 {
  margin: 0 0 0.25rem;
  font-size: 1.4rem;
}

.muted {
  color: var(--muted);
  font-size: 0.85rem;
}

.center {
  text-align: center;
}

.dropzone {
  text-align: center;
  display: grid;
  gap: 0.6rem;
  justify-items: center;
  min-height: 12rem;
  align-content: center;
}

.dropzone h2 {
  margin: 0;
  font-size: 1.4rem;
}

.actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
  justify-content: center;
  margin-top: 0.4rem;
}

.link {
  color: var(--primary);
  text-decoration: none;
  font-weight: 600;
}

.link:hover {
  text-decoration: underline;
}
</style>
