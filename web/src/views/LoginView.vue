<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import LogoIcon from '../components/LogoIcon.vue'
import { useAuthStore } from '../stores/authStore'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()

const username = ref('')
const password = ref('')
const rememberMe = ref(false)
const errorMessage = ref('')
const submitting = ref(false)

async function submit() {
  if (submitting.value) return
  errorMessage.value = ''
  submitting.value = true
  try {
    await auth.login(username.value.trim(), password.value, rememberMe.value)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/home'
    router.replace(redirect)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Login failed'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="auth-layout">
    <section class="auth-card">
      <aside class="auth-pitch">
        <header class="auth-pitch-header">
          <LogoIcon :size="22" />
          <span>Relay</span>
        </header>
        <div class="auth-pitch-body">
          <h1>Chat with your<br />own knowledge.</h1>
          <p class="auth-pitch-tagline">notebooks · feeds · scheduled tasks · all of it.</p>
          <ul class="auth-pitch-list">
            <li>upload PDFs, transcripts, images</li>
            <li>summarize feeds on a schedule</li>
            <li>recall anything you've generated</li>
          </ul>
        </div>
        <footer class="auth-pitch-footer">© 2026 Relay Labs</footer>
      </aside>

      <form class="auth-form" @submit.prevent="submit">
        <h2 class="auth-form-heading">Sign in</h2>

        <label class="auth-field">
          <span class="visually-hidden">Email or username</span>
          <input
            v-model="username"
            type="text"
            placeholder="you@email.com"
            autocomplete="username"
            required
            autofocus
          />
        </label>

        <label class="auth-field">
          <span class="visually-hidden">Password</span>
          <input
            v-model="password"
            type="password"
            placeholder="password"
            autocomplete="current-password"
            required
          />
        </label>

        <div class="auth-row">
          <label class="auth-checkbox">
            <input v-model="rememberMe" type="checkbox" />
            <span>remember me</span>
          </label>
          <span class="auth-forgot" aria-disabled="true" title="Coming soon">forgot?</span>
        </div>

        <p v-if="errorMessage" class="auth-error">{{ errorMessage }}</p>

        <button type="submit" class="auth-primary" :disabled="submitting">
          <span>{{ submitting ? 'Signing in...' : 'Continue' }}</span>
          <span v-if="!submitting" aria-hidden="true">→</span>
        </button>

        <button type="button" class="auth-sso" disabled title="Not available yet">
          Continue with Google
        </button>
        <button type="button" class="auth-sso" disabled title="Not available yet">
          Continue with GitHub
        </button>

        <p class="auth-alt">
          New here?
          <RouterLink to="/register">Create an account</RouterLink>
        </p>
      </form>
    </section>
  </main>
</template>

<style scoped>
.auth-layout {
  height: 100%;
  min-height: 100dvh;
  display: grid;
  place-items: center;
  background: var(--bg);
  padding: 2rem;
  overflow: auto;
  box-sizing: border-box;
}

.auth-card {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0;
  width: min(960px, 100%);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 1rem;
  overflow: hidden;
  box-shadow: var(--shadow);
}

.auth-pitch {
  display: grid;
  grid-template-rows: auto 1fr auto;
  padding: 2rem;
  background: color-mix(in srgb, var(--primary) 6%, var(--surface-soft));
  border-right: 1px solid var(--border);
}

.auth-pitch-header {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 700;
}

.auth-pitch-body {
  align-self: end;
}

.auth-pitch-body h1 {
  font-size: 1.85rem;
  line-height: 1.15;
  margin: 0 0 0.6rem;
}

.auth-pitch-tagline {
  margin: 0 0 1.25rem;
  color: var(--muted);
  font-size: 0.9rem;
}

.auth-pitch-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.5rem;
}

.auth-pitch-list li {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 0.55rem 0.95rem;
  font-size: 0.85rem;
  color: var(--text);
  position: relative;
  padding-left: 1.6rem;
}

.auth-pitch-list li::before {
  content: '';
  position: absolute;
  left: 0.7rem;
  top: 50%;
  transform: translateY(-50%);
  width: 0.4rem;
  height: 0.4rem;
  background: var(--text);
  border-radius: 50%;
}

.auth-pitch-footer {
  font-size: 0.75rem;
  color: var(--muted);
  margin-top: 1.5rem;
}

.auth-form {
  display: grid;
  align-content: center;
  gap: 0.75rem;
  padding: 2.5rem 2rem;
}

.auth-form-heading {
  text-align: center;
  margin: 0 0 0.5rem;
  font-size: 1.4rem;
}

.auth-field {
  display: block;
}

.auth-field input {
  width: 100%;
  padding: 0.75rem 1rem;
  border-radius: 0.5rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-size: 0.95rem;
  font-family: inherit;
  box-sizing: border-box;
}

.auth-field input:focus {
  outline: none;
  border-color: color-mix(in srgb, var(--primary) 60%, var(--border));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent);
}

.auth-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 0.85rem;
  color: var(--muted);
  margin-bottom: 0.25rem;
}

.auth-checkbox {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  cursor: pointer;
}

.auth-forgot {
  text-decoration: underline;
  text-decoration-style: wavy;
  text-underline-offset: 0.2rem;
  cursor: not-allowed;
  opacity: 0.65;
}

.auth-primary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 0.85rem 1rem;
  border-radius: 0.5rem;
  border: 1px solid color-mix(in srgb, var(--primary) 50%, transparent);
  background: var(--primary);
  color: #fff;
  font-weight: 650;
  cursor: pointer;
  font-family: inherit;
  font-size: 0.95rem;
}

.auth-primary:hover:not(:disabled) {
  background: var(--primary-strong);
}

.auth-primary:disabled {
  opacity: 0.7;
  cursor: progress;
}

.auth-sso {
  padding: 0.75rem 1rem;
  border-radius: 0.5rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-weight: 600;
  cursor: not-allowed;
  font-family: inherit;
  font-size: 0.9rem;
  opacity: 0.65;
}

.auth-error {
  margin: 0;
  padding: 0.55rem 0.75rem;
  background: color-mix(in srgb, var(--danger) 12%, var(--surface));
  border: 1px solid color-mix(in srgb, var(--danger) 35%, var(--border));
  border-radius: 0.4rem;
  color: var(--danger);
  font-size: 0.85rem;
}

.auth-alt {
  text-align: center;
  margin: 0.5rem 0 0;
  font-size: 0.85rem;
  color: var(--muted);
}

.auth-alt :deep(a) {
  color: var(--primary);
  text-decoration: none;
  font-weight: 600;
}

.auth-alt :deep(a:hover) {
  text-decoration: underline;
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: -1px;
  padding: 0;
  border: 0;
  clip: rect(0 0 0 0);
  overflow: hidden;
  white-space: nowrap;
}

@media (max-width: 720px) {
  .auth-layout {
    place-items: start center;
    padding: 1rem;
  }

  .auth-card {
    grid-template-columns: 1fr;
    border-radius: 0.8rem;
  }

  .auth-pitch {
    border-right: none;
    border-bottom: 1px solid var(--border);
    padding: 1.25rem;
    gap: 1.5rem;
  }

  .auth-pitch-body {
    align-self: start;
  }

  .auth-pitch-body h1 {
    font-size: 1.42rem;
  }

  .auth-form {
    padding: 1.5rem 1.25rem;
  }
}
</style>
