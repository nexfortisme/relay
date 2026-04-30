<script setup lang="ts">
import type { CSSProperties } from 'vue'

withDefaults(
  defineProps<{
    accent?: boolean
    dashed?: boolean
    thin?: boolean
    fill?: boolean
    pad?: 0 | 8 | 12 | 16 | 20
    style?: string | CSSProperties
  }>(),
  { pad: 12 },
)
</script>

<template>
  <div
    class="wf-box"
    :class="{
      'wf-box--accent': accent,
      'wf-box--dashed': dashed,
      'wf-box--thin': thin,
      'wf-box--fill': fill,
      [`wf-box--pad-${pad}`]: true,
    }"
    :style="style"
  >
    <slot />
  </div>
</template>

<style scoped>
.wf-box {
  border: 1px solid var(--border);
  background: var(--surface);
  border-radius: 0.65rem;
  display: block;
}

.wf-box--pad-0 {
  padding: 0;
}
.wf-box--pad-8 {
  padding: 0.5rem;
}
.wf-box--pad-12 {
  padding: 0.75rem;
}
.wf-box--pad-16 {
  padding: 1rem;
}
.wf-box--pad-20 {
  padding: 1.25rem;
}

.wf-box--accent {
  border-color: color-mix(in srgb, var(--primary) 35%, var(--border));
  background: color-mix(in srgb, var(--surface) 85%, var(--primary) 6%);
}

.wf-box--dashed {
  border-style: dashed;
  background: var(--surface-soft);
}

.wf-box--thin {
  background: var(--surface-soft);
}

.wf-box--fill {
  background: var(--surface-soft);
}
</style>
