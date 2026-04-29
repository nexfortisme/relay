<script setup lang="ts">
import { storeToRefs } from 'pinia'
import AppSidebar from '../components/AppSidebar.vue'
import PageNavTabs from '../components/PageNavTabs.vue'
import { useAppStore } from '../stores/appStore'

const { isSidebarCollapsed } = storeToRefs(useAppStore())
</script>

<template>
  <div
    class="placeholder-view"
    :class="{ 'placeholder-view--sidebar-collapsed': isSidebarCollapsed }"
  >
    <AppSidebar v-if="!isSidebarCollapsed" />
    <section class="placeholder-panel">
      <PageNavTabs />
      <div class="placeholder-body">
        <h1>Scheduled</h1>
        <p>Recurring tasks and prompts will live here.</p>
      </div>
    </section>
  </div>
</template>

<style scoped>
.placeholder-view {
  display: grid;
  grid-template-columns: 280px 1fr;
  height: 100%;
  overflow: hidden;
  background: var(--bg);
}

.placeholder-view.placeholder-view--sidebar-collapsed {
  grid-template-columns: 1fr;
}

.placeholder-panel {
  display: grid;
  grid-template-rows: auto 1fr;
  overflow: hidden;
}

.placeholder-body {
  padding: 3rem clamp(1.5rem, 5vw, 4rem);
  overflow: auto;
}

.placeholder-body h1 {
  margin: 0 0 0.5rem;
}

.placeholder-body p {
  color: var(--muted);
  max-width: 48ch;
}
</style>
