<!--
  LogoLoader.vue — animated "assemble & dissolve" loader (V6 mark).

  Props:
    size      Number  rendered px (square). Default 64.
    duration  String  CSS time (e.g. "2.4s"). Default "2.4s".
    palette   Object  optional color overrides { a, b, c, core }
    label     String  aria-label. Default "Loading".
-->
<script setup lang="ts">
import { computed } from 'vue'

type Tone = 'a' | 'b' | 'c'

type Tri = {
  left: number
  top: number
  r: number
  s?: number
  tone: Tone
}

const TRIS: Tri[] = [
  { left: 60, top: 4, r: -10, tone: 'a' },
  { left: 82, top: 12, r: 35, tone: 'b' },
  { left: 96, top: 38, r: 75, tone: 'a' },
  { left: 98, top: 64, r: 110, tone: 'b' },
  { left: 84, top: 90, r: 150, tone: 'a' },
  { left: 60, top: 102, r: 180, tone: 'c' },
  { left: 36, top: 96, r: 215, tone: 'a' },
  { left: 16, top: 76, r: 250, tone: 'b' },
  { left: 8, top: 50, r: 285, tone: 'a' },
  { left: 18, top: 24, r: 320, tone: 'b' },
  { left: 38, top: 8, r: -30, tone: 'c' },
  { left: 50, top: 34, r: 15, s: 0.7, tone: 'c' },
  { left: 76, top: 56, r: 165, s: 0.7, tone: 'c' },
  { left: 44, top: 70, r: 245, s: 0.7, tone: 'c' },
]

type LogoPalette = {
  a: string
  b: string
  c: string
  core: string
}

const DEFAULT_PALETTE: LogoPalette = {
  a: '#8aa9d9',
  b: '#6f8fc4',
  c: '#b9cef0',
  core: '#0e1726',
}

const CENTER = 70
const FLY_DISTANCE = 1.6

const props = withDefaults(
  defineProps<{
    size?: number
    duration?: string
    palette?: Partial<LogoPalette>
    label?: string
  }>(),
  {
    size: 64,
    duration: '2.4s',
    palette: () => ({}),
    label: 'Loading',
  },
)

const palette = computed(() => ({ ...DEFAULT_PALETTE, ...props.palette }))
const scale = computed(() => props.size / 140)

const rootStyle = computed(() => ({
  '--logo-size': props.size + 'px',
  '--logo-core': palette.value.core,
  '--loader-duration': props.duration,
}))

const durationSec = computed(() => {
  const v = String(props.duration).trim()
  if (v.endsWith('ms')) return parseFloat(v) / 1000
  return parseFloat(v) || 2.4
})

function posStyle(t: Tri, i: number) {
  const k = scale.value
  const cx = t.left + 11
  const cy = t.top + 15
  const dx = (cx - CENTER) * FLY_DISTANCE * k
  const dy = (cy - CENTER) * FLY_DISTANCE * k
  const delay = (i * (durationSec.value / TRIS.length / 2)).toFixed(3) + 's'

  return {
    left: t.left * k + 'px',
    top: t.top * k + 'px',
    width: 22 * k + 'px',
    height: 30 * k + 'px',
    '--r': t.r + 'deg',
    '--s': t.s ?? 1,
    '--c': palette.value[t.tone],
    '--ox': dx.toFixed(1) + 'px',
    '--oy': dy.toFixed(1) + 'px',
    animationDelay: delay,
  }
}
</script>

<template>
  <div class="v6-loader" :style="rootStyle" role="status" :aria-label="label">
    <div v-for="(t, i) in TRIS" :key="i" class="pos" :style="posStyle(t, i)">
      <div class="tri" />
    </div>
    <div class="core" />
    <span class="sr-only">{{ label }}</span>
  </div>
</template>

<style scoped>
.v6-loader {
  width: var(--logo-size);
  height: var(--logo-size);
  position: relative;
  display: inline-block;
}
.pos {
  position: absolute;
  transform-origin: 50% 50%;
  animation: v6-assemble var(--loader-duration, 2.4s) cubic-bezier(0.5, 0, 0.3, 1) infinite;
}
.tri {
  position: absolute;
  inset: 0;
  background: var(--c);
  clip-path: polygon(50% 0, 100% 100%, 0 100%);
  transform: rotate(var(--r, 0deg)) scale(var(--s, 1));
  transform-origin: 50% 50%;
}
.core {
  position: absolute;
  left: 50%;
  top: 50%;
  width: calc(var(--logo-size) * 0.085);
  height: calc(var(--logo-size) * 0.085);
  border-radius: 50%;
  background: var(--logo-core);
  transform: translate(-50%, -50%);
  z-index: 2;
  animation: v6-core var(--loader-duration, 2.4s) ease-in-out infinite;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

@keyframes v6-assemble {
  0% {
    transform: translate(var(--ox, 0px), var(--oy, 0px)) scale(0);
    opacity: 0;
  }
  35% {
    transform: translate(0, 0) scale(1);
    opacity: 1;
  }
  70% {
    transform: translate(0, 0) scale(1);
    opacity: 1;
  }
  100% {
    transform: translate(calc(var(--ox, 0px) * -0.6), calc(var(--oy, 0px) * -0.6)) scale(0);
    opacity: 0;
  }
}
@keyframes v6-core {
  0%,
  100% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(0);
  }
  35%,
  70% {
    opacity: 1;
    transform: translate(-50%, -50%) scale(1);
  }
}

@media (prefers-reduced-motion: reduce) {
  .pos,
  .core {
    animation: none;
  }
  .pos {
    transform: none;
  }
  .core {
    opacity: 1;
    transform: translate(-50%, -50%);
  }
}
</style>
