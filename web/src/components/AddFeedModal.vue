<script setup lang="ts">
const emit = defineEmits<{ close: []; add: [string] }>()

const suggestions = ['Hacker News', 'Stratechery', 'NYT Cooking', 'TLDR', 'Pitchfork', 'Philly Mag']
</script>

<template>
  <div class="backdrop" role="dialog" aria-modal="true" @click.self="emit('close')">
    <WfBox :pad="20" class="modal">
      <header class="head">
        <div>
          <h2>＋ Add a feed</h2>
          <p class="muted">Paste a URL, OPML file, or pick from suggestions</p>
        </div>
        <button class="close" aria-label="Close" @click="emit('close')">✕</button>
      </header>
      <hr />

      <div class="row">
        <input
          class="url-input"
          placeholder="https://www.theverge.com/rss/index.xml"
          value="https://www.theverge.com/rss/index.xml"
        />
        <WfBtn primary>Detect</WfBtn>
      </div>

      <WfBox fill :pad="12" class="detected">
        <WfPlaceholder circle :w="28" :h="28">R</WfPlaceholder>
        <div class="detected-text">
          <div class="title">The Verge — All Posts</div>
          <div class="muted small">~30 items / day · last update 2h ago</div>
        </div>
        <WfTag>RSS 2.0 ✓</WfTag>
      </WfBox>

      <h3>Settings</h3>
      <div class="settings-row">
        <span class="muted small">Fetch every</span>
        <input class="input-tiny" value="30 min" />
        <label class="opt"><WfCheckbox on /> Auto-summarize</label>
        <label class="opt"><WfCheckbox /> Save to notebook</label>
        <input class="input-tiny wide" placeholder="— pick notebook —" />
      </div>

      <hr class="dashed" />
      <p class="muted small">Or pick from suggestions</p>
      <div class="suggestions">
        <WfChip v-for="s in suggestions" :key="s">＋ {{ s }}</WfChip>
      </div>

      <footer class="actions">
        <WfBtn ghost @click="emit('close')">Cancel</WfBtn>
        <WfBtn primary @click="emit('add', 'The Verge')">Add feed</WfBtn>
      </footer>
    </WfBox>
  </div>
</template>

<style scoped>
.backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: grid;
  place-items: center;
  padding: 1rem;
  z-index: 50;
}

.modal {
  width: min(36rem, 100%);
  max-height: 90vh;
  overflow: auto;
  display: grid;
  gap: 0.65rem;
  box-shadow: var(--shadow);
}

.head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 0.5rem;
}

.head h2 {
  margin: 0 0 0.25rem;
  font-size: 1.15rem;
}

.muted {
  color: var(--muted);
  font-size: 0.85rem;
}

.small {
  font-size: 0.78rem;
}

.close {
  border: none;
  background: transparent;
  color: var(--muted);
  font-size: 1.05rem;
  cursor: pointer;
  width: 1.6rem;
  height: 1.6rem;
  border-radius: 0.4rem;
}

.close:hover {
  background: var(--surface-hover);
  color: var(--text);
}

hr {
  border: none;
  border-top: 1px solid var(--border);
  margin: 0.2rem 0;
}

hr.dashed {
  border-top: 1px dashed var(--border);
  margin: 0.4rem 0;
}

.row {
  display: flex;
  gap: 0.5rem;
}

.url-input,
.input-tiny {
  padding: 0.55rem 0.75rem;
  border-radius: 0.5rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-size: 0.88rem;
}

.url-input {
  flex: 1;
  min-width: 0;
}

.url-input:focus,
.input-tiny:focus {
  outline: none;
  border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
}

.input-tiny {
  width: 6rem;
  font-size: 0.82rem;
  padding: 0.35rem 0.55rem;
}

.input-tiny.wide {
  width: 9rem;
}

.detected {
  display: flex;
  align-items: center;
  gap: 0.65rem;
}

.detected-text {
  flex: 1;
  min-width: 0;
}

.title {
  font-weight: 700;
  font-size: 0.9rem;
}

h3 {
  margin: 0.4rem 0 0.1rem;
  font-size: 0.9rem;
}

.settings-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.55rem;
  font-size: 0.82rem;
}

.opt {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  cursor: pointer;
}

.suggestions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.actions {
  display: flex;
  justify-content: space-between;
  margin-top: 0.4rem;
}
</style>
