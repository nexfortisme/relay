<!--
  LoaderPrism.vue
  ──────────────────────────────────────────────────────────────────
  Variant B: rotating prism with breathing facets.

  Four triangular facets radiate from a center point. The whole
  cluster rotates continuously while it breathes (scales) in/out;
  each facet also extends outward and grows as it breathes.

  Two transforms on the same node fight each other, so:
    - The OUTER element owns the rotation.
    - An INNER wrapper owns the breathing scale.
    - Each FACET owns its own breathing translate + grow.

  All timings, sizes, and colors are exposed as CSS custom
  properties so the component is fully tweakable from the outside,
  e.g.:

      <LoaderPrism
        :size="96"
        :spin-duration="2.4"
        :breath-duration="1.2"
        color="#0e7490"
      />

  Or override per-instance with style="--prism-spin-duration: 4s":

      <LoaderPrism style="--prism-spin-duration: 5s" />
-->

<template>
  <div
    class="loader-prism"
    role="status"
    aria-label="Loading"
    :style="cssVars"
  >
    <div class="loader-prism__inner">
      <div class="loader-prism__face loader-prism__face--1"></div>
      <div class="loader-prism__face loader-prism__face--2"></div>
      <div class="loader-prism__face loader-prism__face--3"></div>
      <div class="loader-prism__face loader-prism__face--4"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps({
  // Overall footprint of the loader (px). Facet size scales with this.
  size: { type: Number, default: 80 },

  // Seconds for one full 360° rotation.
  spinDuration: { type: Number, default: 3.2 },

  // Seconds for one full breathing cycle (in → out → in).
  breathDuration: { type: Number, default: 1.6 },

  // Facet color (any CSS color).
  color: { type: String, default: '#14201f' },

  // How far each facet's tip pushes outward from center at peak (px).
  breathDistance: { type: Number, default: 14 },

  // Facet base width / triangle height at rest (px).
  faceWidth: { type: Number, default: 16 },
  faceHeight: { type: Number, default: 22 },

  // Facet height at peak breath (px). Should be > faceHeight.
  faceHeightPeak: { type: Number, default: 34 },

  // Cluster scale at trough / peak of breath.
  clusterScaleMin: { type: Number, default: 0.82 },
  clusterScaleMax: { type: Number, default: 1.08 },

  // Facet opacity at trough of breath (peak is 1).
  opacityMin: { type: Number, default: 0.55 },
});

// Pipe props into CSS custom properties so the stylesheet can read them.
const cssVars = computed(() => ({
  '--prism-size':            `${props.size}px`,
  '--prism-spin-duration':   `${props.spinDuration}s`,
  '--prism-breath-duration': `${props.breathDuration}s`,
  '--prism-color':           props.color,
  '--prism-breath-distance': `${props.breathDistance}px`,
  '--prism-face-w':          `${props.faceWidth}px`,
  '--prism-face-h':          `${props.faceHeight}px`,
  '--prism-face-h-peak':     `${props.faceHeightPeak}px`,
  '--prism-cluster-min':     props.clusterScaleMin,
  '--prism-cluster-max':     props.clusterScaleMax,
  '--prism-opacity-min':     props.opacityMin,
}));
</script>

<style scoped>
.loader-prism {
  width: var(--prism-size);
  height: var(--prism-size);
  position: relative;
  /* Outer: rotation only */
  animation: loader-prism-spin var(--prism-spin-duration) linear infinite;
}

.loader-prism__inner {
  position: absolute;
  inset: 0;
  /* Inner: breathing scale only */
  animation: loader-prism-scale var(--prism-breath-duration) ease-in-out infinite;
}

.loader-prism__face {
  position: absolute;
  width: 0;
  height: 0;
  top: 50%;
  left: 50%;
  border-left:  var(--prism-face-w) solid transparent;
  border-right: var(--prism-face-w) solid transparent;
  border-bottom: var(--prism-face-h) solid var(--prism-color);
  transform-origin: 50% 100%;
  /* Facet: outward push + grow + opacity */
  animation: loader-prism-face var(--prism-breath-duration) ease-in-out infinite;
}

/* Each facet sits at a 90° offset around the center.
   --rot is consumed by the keyframe to keep the rotation. */
.loader-prism__face--1 { --rot:   0deg; }
.loader-prism__face--2 { --rot:  90deg; }
.loader-prism__face--3 { --rot: 180deg; }
.loader-prism__face--4 { --rot: 270deg; }

@keyframes loader-prism-spin {
  to { transform: rotate(360deg); }
}

@keyframes loader-prism-scale {
  0%, 100% { transform: scale(var(--prism-cluster-min)); }
  50%      { transform: scale(var(--prism-cluster-max)); }
}

@keyframes loader-prism-face {
  0%, 100% {
    transform: translate(-50%, -100%) rotate(var(--rot)) translateY(0);
    border-bottom-width: var(--prism-face-h);
    opacity: var(--prism-opacity-min);
  }
  50% {
    transform: translate(-50%, -100%) rotate(var(--rot)) translateY(calc(var(--prism-breath-distance) * -1));
    border-bottom-width: var(--prism-face-h-peak);
    opacity: 1;
  }
}

@media (prefers-reduced-motion: reduce) {
  .loader-prism,
  .loader-prism__inner,
  .loader-prism__face {
    animation-duration: 0.001ms !important;
    animation-iteration-count: 1 !important;
  }
}
</style>
