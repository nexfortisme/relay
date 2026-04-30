<script setup lang="ts">
import { ref } from 'vue'
import PrismLogo from '../components/PrismLogo.vue'

const email = ref('')
const password = ref('')
const remember = ref(false)
const submitting = ref(false)

const features = [
  'Upload PDFs, transcripts, images',
  'Summarize feeds on a schedule',
  'Recall anything you\'ve generated',
]

function handleSubmit() {
  // TODO: wire up to backend auth once available
  submitting.value = true
  setTimeout(() => {
    submitting.value = false
  }, 600)
}

function continueWith(provider: string) {
  // TODO: SSO providers — stub for future integration
  console.warn(`SSO provider not wired up yet: ${provider}`)
}
</script>

<template>
  <div class="login-view">
    <aside class="login-marketing">
      <div class="login-brand">
        <PrismLogo />
        <span>Relay</span>
      </div>

      <div class="login-pitch">
        <h1>Chat with your<br />own knowledge.</h1>
        <p>Notebooks · feeds · scheduled tasks · all of it.</p>
        <div class="login-features">
          <WfChip v-for="feature in features" :key="feature" dot>{{ feature }}</WfChip>
        </div>
      </div>

      <div class="login-footer">© 2026 Relay Labs</div>
    </aside>

    <section class="login-form-wrap">
      <form class="login-form" @submit.prevent="handleSubmit">
        <h2>Sign in</h2>

        <label class="field">
          <span>Email</span>
          <input
            v-model="email"
            type="email"
            placeholder="you@email.com"
            autocomplete="email"
            required
          />
        </label>

        <label class="field">
          <span>Password</span>
          <input
            v-model="password"
            type="password"
            placeholder="••••••••"
            autocomplete="current-password"
            required
          />
        </label>

        <div class="row-between">
          <label class="remember">
            <WfCheckbox v-model="remember" /> Remember me
          </label>
          <a href="#" class="link">Forgot?</a>
        </div>

        <WfBtn primary type="submit" :disabled="submitting" :style="{ width: '100%' }">
          {{ submitting ? 'Signing in…' : 'Continue →' }}
        </WfBtn>

        <div class="divider"><span>or</span></div>

        <WfBtn :style="{ width: '100%' }" @click="continueWith('google')">
          Continue with Google
        </WfBtn>
        <WfBtn :style="{ width: '100%' }" @click="continueWith('github')">
          Continue with GitHub
        </WfBtn>

        <p class="signup">
          No account? <a href="#" class="link">Sign up free →</a>
        </p>
      </form>
    </section>
  </div>
</template>

<style scoped>
.login-view {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr);
  height: 100%;
  background: var(--bg);
  color: var(--text);
}

.login-marketing {
  display: grid;
  grid-template-rows: auto 1fr auto;
  padding: 2rem clamp(1.5rem, 4vw, 3rem);
  background: var(--surface-soft);
  border-right: 1px solid var(--border);
}

.login-brand {
  display: inline-flex;
  align-items: center;
  gap: 0.55rem;
  font-weight: 700;
  font-size: 1.1rem;
}

.login-brand :deep(.prism-logo) {
  --logo-size: 1.45rem;
  --logo-face-w: 0.34rem;
  --logo-face-h: 0.7rem;
}

.login-pitch {
  align-self: center;
  display: grid;
  gap: 0.85rem;
  max-width: 28rem;
}

.login-pitch h1 {
  margin: 0;
  font-size: clamp(1.8rem, 3.4vw, 2.6rem);
  line-height: 1.1;
  font-weight: 700;
}

.login-pitch p {
  margin: 0;
  color: var(--muted);
  font-size: 0.95rem;
}

.login-features {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.5rem;
  margin-top: 0.6rem;
}

.login-footer {
  color: var(--muted);
  font-size: 0.78rem;
}

.login-form-wrap {
  display: grid;
  place-items: center;
  padding: 2rem;
  overflow: auto;
}

.login-form {
  width: 100%;
  max-width: 22rem;
  display: grid;
  gap: 0.85rem;
}

.login-form h2 {
  margin: 0 0 0.25rem;
  font-size: 1.4rem;
  font-weight: 700;
}

.field {
  display: grid;
  gap: 0.3rem;
}

.field span {
  font-size: 0.8rem;
  color: var(--muted);
  font-weight: 600;
}

.field input {
  padding: 0.65rem 0.8rem;
  border-radius: 0.55rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-size: 0.95rem;
}

.field input:focus {
  outline: none;
  border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
}

.row-between {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 0.82rem;
  color: var(--muted);
}

.remember {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  cursor: pointer;
}

.link {
  color: var(--primary);
  text-decoration: none;
}

.link:hover {
  text-decoration: underline;
}

.divider {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  color: var(--muted);
  font-size: 0.78rem;
  margin: 0.2rem 0;
}

.divider::before,
.divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--border);
}

.signup {
  margin: 0.4rem 0 0;
  font-size: 0.85rem;
  color: var(--muted);
  text-align: center;
}

@media (max-width: 720px) {
  .login-view {
    grid-template-columns: 1fr;
    grid-template-rows: auto 1fr;
  }

  .login-marketing {
    border-right: none;
    border-bottom: 1px solid var(--border);
  }
}
</style>
