<script setup lang="ts">
import type { CSSProperties } from 'vue'

defineProps<{
  dot?: boolean
  active?: boolean
  style?: string | CSSProperties
}>()
</script>

<template>
  <span
    class="wf-chip"
    :class="{ 'wf-chip--dot': dot, 'wf-chip--active': active }"
    :style="style"
  >
    <span v-if="dot" class="wf-chip__dot" />
    <slot />
  </span>
</template>

<style scoped>
.wf-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.2rem 0.6rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--muted);
  font-size: 0.75rem;
  font-weight: 600;
  white-space: nowrap;
}

.wf-chip--active {
  color: var(--text);
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
  background: var(--selected);
}

.wf-chip__dot {
  width: 0.4rem;
  height: 0.4rem;
  border-radius: 999px;
  background: var(--primary);
}
</style>
