<script setup lang="ts">
import { computed } from 'vue'
import type { CSSProperties } from 'vue'

const props = defineProps<{
  w?: number | string
  h?: number | string
  circle?: boolean
  style?: string | CSSProperties
}>()

const sizeStyle = computed<CSSProperties>(() => {
  const out: CSSProperties = {}
  if (props.w !== undefined) {
    out.width = typeof props.w === 'number' ? `${props.w}px` : props.w
  }
  if (props.h !== undefined) {
    out.height = typeof props.h === 'number' ? `${props.h}px` : props.h
  }
  return out
})
</script>

<template>
  <div class="wf-ph" :class="{ 'wf-ph--circle': circle }" :style="[sizeStyle, style ?? {}]">
    <slot />
  </div>
</template>

<style scoped>
.wf-ph {
  display: inline-grid;
  place-items: center;
  background: var(--surface-soft);
  border: 1px dashed var(--border);
  border-radius: 0.4rem;
  color: var(--muted);
  font-size: 0.8rem;
  text-align: center;
  min-width: 1.2rem;
  min-height: 1.2rem;
}

.wf-ph--circle {
  border-radius: 999px;
  border-style: solid;
  background: color-mix(in srgb, var(--primary) 15%, var(--surface));
  color: var(--primary);
  font-weight: 700;
}
</style>
