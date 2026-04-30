<script setup lang="ts">
import type { CSSProperties } from 'vue'

defineProps<{ style?: string | CSSProperties }>()
</script>

<template>
  <span class="wf-tag" :style="style"><slot /></span>
</template>

<style scoped>
.wf-tag {
  display: inline-flex;
  align-items: center;
  padding: 0.15rem 0.5rem;
  border-radius: 0.3rem;
  background: color-mix(in srgb, var(--primary) 14%, var(--surface));
  color: var(--primary);
  border: 1px solid color-mix(in srgb, var(--primary) 35%, var(--border));
  font-size: 0.7rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
</style>
