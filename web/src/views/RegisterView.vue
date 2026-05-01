<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import PrismLogo from '../components/PrismLogo.vue'
import { useAuthStore } from '../stores/authStore'

const auth = useAuthStore()
const router = useRouter()

const username = ref('')
const password = ref('')
const passwordConfirm = ref('')
const rememberMe = ref(true)
const errorMessage = ref('')
const submitting = ref(false)

const passwordMismatch = computed(
  () => passwordConfirm.value.length > 0 && password.value !== passwordConfirm.value,
)

async function submit() {
  if (submitting.value) return
  errorMessage.value = ''
  if (password.value !== passwordConfirm.value) {
    errorMessage.value = 'Passwords do not match'
    return
  }
  submitting.value = true
  try {
    await auth.register(username.value.trim(), password.value, rememberMe.value)
    router.replace('/')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Registration failed'
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
          <PrismLogo />
          <span>Relay</span>
        </header>
        <div class="auth-pitch-body">
          <h1>Create your<br />Relay account.</h1>
          <p class="auth-pitch-tagline">private notebooks, feeds, and chat — all yours.</p>
          <ul class="auth-pitch-list">
            <li>your data stays in your account</li>
            <li>tweak prompts and models per user</li>
            <li>sign in from anywhere on any device</li>
          </ul>
        </div>
        <footer class="auth-pitch-footer">© 2026 Relay Labs</footer>
      </aside>

      <form class="auth-form" @submit.prevent="submit">
        <h2 class="auth-form-heading">Create account</h2>

        <label class="auth-field">
          <span class="visually-hidden">Username</span>
          <input
            v-model="username"
            type="text"
            placeholder="username"
            autocomplete="username"
            required
            autofocus
            minlength="2"
          />
        </label>

        <label class="auth-field">
          <span class="visually-hidden">Password</span>
          <input
            v-model="password"
            type="password"
            placeholder="password"
            autocomplete="new-password"
            required
            minlength="4"
          />
        </label>

        <label class="auth-field">
          <span class="visually-hidden">Confirm password</span>
          <input
            v-model="passwordConfirm"
            type="password"
            placeholder="confirm password"
            autocomplete="new-password"
            required
            minlength="4"
            :aria-invalid="passwordMismatch"
          />
        </label>

        <div class="auth-row">
          <label class="auth-checkbox">
            <input v-model="rememberMe" type="checkbox" />
            <span>remember me</span>
          </label>
        </div>

        <p v-if="errorMessage || passwordMismatch" class="auth-error">
          {{ errorMessage || 'Passwords do not match' }}
        </p>

        <button type="submit" class="auth-primary" :disabled="submitting || passwordMismatch">
          <span>{{ submitting ? 'Creating...' : 'Create account' }}</span>
          <span v-if="!submitting" aria-hidden="true">→</span>
        </button>

        <p class="auth-alt">
          Already have an account?
          <RouterLink to="/login">Sign in</RouterLink>
        </p>
      </form>
    </section>
  </main>
</template>

<style scoped>
.auth-layout {
  min-height: 100dvh;
  display: grid;
  place-items: center;
  background: var(--bg);
  padding: 2rem;
}

.auth-card {
  display: grid;
  grid-template-columns: 1fr 1fr;
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

.auth-pitch-header :deep(.prism-logo) {
  --logo-size: 1.4rem;
  --logo-face-w: 0.32rem;
  --logo-face-h: 0.66rem;
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

.auth-field input[aria-invalid='true'] {
  border-color: color-mix(in srgb, var(--danger) 60%, var(--border));
}

.auth-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 0.85rem;
  color: var(--muted);
}

.auth-checkbox {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  cursor: pointer;
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
  cursor: not-allowed;
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
  .auth-card {
    grid-template-columns: 1fr;
  }
  .auth-pitch {
    border-right: none;
    border-bottom: 1px solid var(--border);
  }
}
</style>
