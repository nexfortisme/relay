<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import greetingsMarkdown from "../resources/greetings.md?raw";
import { selectGreeting } from "../lib/greetings";
import LogoIcon from "./LogoIcon.vue";
import { brandLogoPalette } from "../lib/logoPalette";

const now = ref(new Date());
let clockTimer: ReturnType<typeof setInterval> | null = null;

const greeting = computed(() => selectGreeting(greetingsMarkdown, now.value));

const mqCompact = ref(false);
let mq: MediaQueryList | null = null;

function syncMq() {
  if (typeof window.matchMedia !== "function") {
    mqCompact.value = false;
    return;
  }
  mqCompact.value = window.matchMedia("(max-width: 760px)").matches;
}

onMounted(() => {
  if (typeof window.matchMedia === "function") {
    mq = window.matchMedia("(max-width: 760px)");
    syncMq();
    mq.addEventListener("change", syncMq);
  }
  clockTimer = setInterval(() => {
    now.value = new Date();
  }, 60_000);
});

onUnmounted(() => {
  mq?.removeEventListener("change", syncMq);
  if (clockTimer) {
    clearInterval(clockTimer);
  }
});

const logoSize = computed(() => (mqCompact.value ? 38 : 50));
</script>

<template>
  <div class="empty-chat-greeting" aria-live="polite">
    <LogoIcon :size="logoSize" :palette="brandLogoPalette" />
    <p>{{ greeting }}</p>
  </div>
</template>

<style scoped>
.empty-chat-greeting {
  position: absolute;
  left: 1.35rem;
  right: 1.35rem;
  bottom: 7.8rem;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 0.85rem;
  pointer-events: none;
  color: color-mix(in srgb, var(--text) 90%, var(--muted));
}

.empty-chat-greeting--with-files {
  bottom: 10.2rem;
}

.empty-chat-greeting p {
  min-width: 0;
  margin: 0;
  font-family: Georgia, "Times New Roman", serif;
  font-size: clamp(1.8rem, 4.5vw, 2.5rem);
  line-height: 1.02;
  text-align: left;
  overflow-wrap: anywhere;
}

@media (max-width: 760px) {
  .empty-chat-greeting {
    left: 0.78rem;
    right: 0.78rem;
    bottom: 7.4rem;
    gap: 0.55rem;
  }

  .empty-chat-greeting--with-files {
    bottom: 10rem;
  }

  .empty-chat-greeting p {
    font-size: 1.2rem;
  }
}
</style>
