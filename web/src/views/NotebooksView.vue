<script setup lang="ts">
import { storeToRefs } from 'pinia'
import AppSidebar from '../components/AppSidebar.vue'
import PageNavTabs from '../components/PageNavTabs.vue'
import { useUiStore } from '../stores/uiStore'

const uiStore = useUiStore()
const { isSidebarCollapsed } = storeToRefs(uiStore)
</script>

<template>
  <div
    class="placeholder-view"
    :class="{ 'placeholder-view--sidebar-collapsed': isSidebarCollapsed }"
  >
    <AppSidebar v-if="!isSidebarCollapsed" />
    <button
      v-if="!isSidebarCollapsed"
      class="mobile-sidebar-backdrop"
      type="button"
      aria-label="Close sidebar"
      @click="uiStore.toggleSidebarCollapsed"
    />
    <section class="placeholder-panel">
      <PageNavTabs />
      <div class="placeholder-body">
        <h1>Notebooks</h1>
        <p>This sub-application isn't built yet. It'll let you organize documents into searchable collections.</p>
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
  position: relative;
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

.mobile-sidebar-backdrop {
  display: none;
}

@media (max-width: 760px) {
  .placeholder-view {
    grid-template-columns: 1fr;
  }

  .placeholder-view:not(.placeholder-view--sidebar-collapsed) :deep(.home-sidebar) {
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

  .placeholder-panel {
    min-width: 0;
  }

  .placeholder-body {
    padding: 2rem clamp(1rem, 5vw, 1.5rem);
  }
}
</style>
