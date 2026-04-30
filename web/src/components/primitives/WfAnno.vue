<script setup lang="ts">
import type { CSSProperties } from 'vue'

defineProps<{ style?: string | CSSProperties }>()
</script>

<template>
  <div class="wf-anno" :style="style"><slot /></div>
</template>

<style scoped>
.wf-anno {
  position: absolute;
  font-size: 0.72rem;
  color: var(--muted);
  font-style: italic;
  pointer-events: none;
}
</style>
