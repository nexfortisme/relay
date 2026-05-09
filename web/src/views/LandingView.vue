<script setup lang="ts">
import AppIcon from '../components/AppIcon.vue'
import LogoIcon from '../components/LogoIcon.vue'
import screenshotChatSrc from '../assets/landing/relay-chat.png'
import screenshotFeedsSrc from '../assets/landing/relay-feeds.png'
import screenshotHomeSrc from '../assets/landing/relay-home.png'

const capabilities = [
  {
    icon: 'sparkles',
    title: 'Streaming chat with model detail',
    description:
      'Watch answers arrive token by token, inspect reasoning streams when models expose them, and keep model and token context visible in the header.',
  },
  {
    icon: 'book',
    title: 'Notebooks with working memory',
    description:
      'Create focused notebooks with their own system and skill prompts, then chat inside the source collection that belongs to the job.',
  },
  {
    icon: 'file',
    title: 'File indexing and CSV tools',
    description:
      'Upload PDFs, documents, images, and CSVs, track processing progress, render PDF pages, and query or update table rows from chat.',
  },
  {
    icon: 'send',
    title: 'Feeds become usable context',
    description:
      'Subscribe to RSS or Atom feeds, backfill items, mark unread or starred, and summarize articles on demand or automatically.',
  },
  {
    icon: 'paperclip',
    title: 'Attachments stay with messages',
    description:
      'Drop files directly into a chat, keep optimistic uploads responsive, preview images, and download persisted attachments later.',
  },
  {
    icon: 'settings',
    title: 'Private accounts and settings',
    description:
      'Use cookie-based sessions, per-user model settings, optional API keys, and custom prompts without turning the app into admin theater.',
  },
] as const

const screenshots = [
  {
    src: screenshotHomeSrc,
    title: 'Start from the dashboard',
    description:
      'Launch a chat, jump into feeds, or pick up recent work from a dashboard built for repeat use.',
  },
  {
    src: screenshotChatSrc,
    title: 'Work inside live chat',
    description:
      'Stream answers, include attachments, inspect thinking, requeue useful prompts, and see which model contributed to the thread.',
  },
  {
    src: screenshotFeedsSrc,
    title: 'Turn feeds into context',
    description:
      'Read incoming items, summarize or expand them with the model, and carry the useful parts into notebooks and chats.',
  },
] as const

const notebookHighlights = [
  'Per-notebook prompts',
  'PDF page rendering',
  'LLM image descriptions',
  'CSV query and edit tools',
] as const

const workflow = ['Collect', 'Index', 'Converse', 'Act'] as const
</script>

<template>
  <div class="landing-view">
    <header class="landing-nav">
      <RouterLink class="brand-link" to="/" aria-label="Relay landing page">
        <LogoIcon :size="26" />
        <span>Relay</span>
      </RouterLink>

      <nav class="nav-links" aria-label="Landing page sections">
        <a href="#capabilities">Capabilities</a>
        <a href="#notebooks">Notebooks</a>
        <a href="#screenshots">Screenshots</a>
        <a href="#workflow">Workflow</a>
        <a href="#about">About</a>
      </nav>

      <div class="nav-actions">
        <RouterLink class="nav-link-button" to="/login">Sign in</RouterLink>
        <RouterLink class="nav-primary" to="/register">Get started</RouterLink>
      </div>
    </header>

    <main>
      <section class="hero-section" aria-labelledby="landing-title">
        <div class="hero-facets" aria-hidden="true">
          <span class="facet facet--one" />
          <span class="facet facet--two" />
          <span class="facet facet--three" />
          <span class="facet facet--four" />
        </div>

        <div class="hero-screens" aria-hidden="true">
          <img class="hero-screen hero-screen--main" :src="screenshotHomeSrc" alt="" />
          <img class="hero-screen hero-screen--chat" :src="screenshotChatSrc" alt="" />
          <img class="hero-screen hero-screen--feeds" :src="screenshotFeedsSrc" alt="" />
        </div>

        <div class="hero-copy">
          <div class="hero-mark">
            <LogoIcon :size="28" />
            <span>AI chat for your own working context</span>
          </div>
          <h1 id="landing-title">Relay</h1>
          <p class="hero-lede">
            A private AI workspace for chats, notebooks, feeds, and the files that shape your
            work. Relay keeps the context close, the interface calm, and the next action easy to
            find.
          </p>
          <div class="hero-actions">
            <RouterLink class="primary-action" to="/register">Create account</RouterLink>
            <RouterLink class="secondary-action" to="/login">Sign in</RouterLink>
          </div>
          <ul class="hero-points" aria-label="Relay highlights">
            <li>Streaming responses</li>
            <li>Notebook RAG</li>
            <li>Feed summaries</li>
            <li>CSV tools</li>
            <li>PDF indexing</li>
          </ul>
        </div>
      </section>

      <section id="capabilities" class="capability-section" aria-labelledby="capabilities-title">
        <div class="section-heading">
          <span class="section-kicker">Faceted by design</span>
          <h2 id="capabilities-title">Everything can become conversation context.</h2>
          <p>
            Relay is built around the way useful work accumulates: a question, a source file, a
            feed item, a table, a follow-up, then another question with better context.
          </p>
        </div>

        <div class="capability-grid">
          <article
            v-for="capability in capabilities"
            :key="capability.title"
            class="capability-card"
          >
            <span class="capability-icon">
              <AppIcon :name="capability.icon" :size="22" />
            </span>
            <h3>{{ capability.title }}</h3>
            <p>{{ capability.description }}</p>
          </article>
        </div>
      </section>

      <section id="notebooks" class="notebook-section" aria-labelledby="notebooks-title">
        <div class="notebook-inner">
          <div class="notebook-copy">
            <span class="section-kicker">New in notebooks</span>
            <h2 id="notebooks-title">Source collections that can actually do work.</h2>
            <p>
              Notebooks now have their own files, chats, prompts, processing jobs, and scoped tools.
              They can search indexed documents, expose tables to the model, and preserve the
              context of a project without mixing it into every chat.
            </p>
          </div>

          <div class="notebook-panel" aria-label="Notebook feature highlights">
            <div class="notebook-panel-header">
              <span>Notebook workspace</span>
              <strong>Ready</strong>
            </div>
            <ul>
              <li v-for="highlight in notebookHighlights" :key="highlight">
                <AppIcon name="check" :size="15" />
                <span>{{ highlight }}</span>
              </li>
            </ul>
          </div>
        </div>
      </section>

      <section id="screenshots" class="screenshot-section" aria-labelledby="screenshots-title">
        <div class="section-heading section-heading--dark">
          <span class="section-kicker">Application screenshots</span>
          <h2 id="screenshots-title">The app surface stays close to the work.</h2>
          <p>
            Home, chat, and feed views share the same prism mark, restrained controls, and
            high-contrast working surface so moving between source material and conversation feels
            continuous.
          </p>
        </div>

        <div class="screenshot-grid">
          <figure
            v-for="screenshot in screenshots"
            :key="screenshot.src"
            class="screenshot-card"
          >
            <img :src="screenshot.src" :alt="`${screenshot.title} screenshot`" />
            <figcaption>
              <strong>{{ screenshot.title }}</strong>
              <span>{{ screenshot.description }}</span>
            </figcaption>
          </figure>
        </div>
      </section>

      <section id="workflow" class="workflow-section" aria-labelledby="workflow-title">
        <div class="workflow-inner">
          <div class="workflow-copy">
            <span class="section-kicker">From signal to answer</span>
            <h2 id="workflow-title">A simple loop for context-rich AI work.</h2>
            <p>
              Relay gives each surface a job: collect source material, index it into the right
              workspace, talk through it with a model, and act on the answer with tools when the
              next step is concrete.
            </p>
          </div>

          <ol class="workflow-list">
            <li v-for="(step, index) in workflow" :key="step">
              <span>{{ String(index + 1).padStart(2, '0') }}</span>
              <strong>{{ step }}</strong>
            </li>
          </ol>
        </div>
      </section>

      <section id="about" class="about-section" aria-labelledby="about-title">
        <div class="about-inner">
          <h2 id="about-title">About</h2>
          <p>
            Relay is open source. Report issues, suggest improvements, or explore the code on
            <a
              href="https://github.com/nexfortisme/relay"
              target="_blank"
              rel="noopener noreferrer"
            >GitHub</a>.
          </p>
        </div>
      </section>
    </main>

    <footer class="landing-footer">
      <RouterLink class="footer-brand" to="/">
        <LogoIcon :size="26" />
        <span>Relay</span>
      </RouterLink>
      <span>Built for private, context-rich AI work.</span>
      <RouterLink to="/login">Sign in</RouterLink>
    </footer>
  </div>
</template>

<style scoped>
.landing-view {
  height: 100%;
  overflow: auto;
  background: #0f1115;
  color: #f4f7fb;
  scroll-behavior: smooth;
}

.landing-nav {
  position: sticky;
  top: 0;
  z-index: 20;
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 1.5rem;
  min-height: 4.5rem;
  padding: 0 2rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(15, 17, 21, 0.86);
  backdrop-filter: blur(18px);
}

.brand-link,
.footer-brand {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
  color: inherit;
  font-weight: 760;
  text-decoration: none;
}

.nav-links {
  display: flex;
  justify-content: center;
  gap: 1.35rem;
}

.nav-links a,
.nav-link-button,
.landing-footer a {
  color: #b7c0ce;
  font-size: 0.9rem;
  font-weight: 650;
  text-decoration: none;
}

.nav-links a:hover,
.nav-link-button:hover,
.landing-footer a:hover {
  color: #f4f7fb;
}

.nav-actions {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.65rem;
}

.nav-primary,
.primary-action,
.secondary-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.5rem;
  border-radius: 8px;
  padding: 0 1rem;
  font-size: 0.9rem;
  font-weight: 760;
  text-decoration: none;
}

.nav-primary,
.primary-action {
  border: 1px solid rgba(96, 165, 250, 0.62);
  background: #2563eb;
  color: #ffffff;
}

.nav-primary:hover,
.primary-action:hover {
  background: #1d4ed8;
}

.secondary-action {
  border: 1px solid rgba(148, 163, 184, 0.28);
  background: rgba(25, 29, 39, 0.88);
  color: #f4f7fb;
}

.secondary-action:hover {
  background: #202533;
}

.hero-section {
  position: relative;
  min-height: 82dvh;
  isolation: isolate;
  display: grid;
  align-items: center;
  overflow: hidden;
  padding: 4rem 2rem 4.75rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.18);
}

.hero-section::after {
  content: '';
  position: absolute;
  inset: auto 0 0;
  height: 12rem;
  z-index: -1;
  background: linear-gradient(180deg, rgba(15, 17, 21, 0), #0f1115 76%);
}

.hero-section::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: -1;
  background: linear-gradient(90deg, #0f1115 0%, rgba(15, 17, 21, 0.92) 36%, transparent 68%);
}

.hero-facets {
  position: absolute;
  inset: 0;
  z-index: -3;
  overflow: hidden;
  background:
    linear-gradient(120deg, rgba(37, 99, 235, 0.2), transparent 36%),
    linear-gradient(250deg, rgba(245, 158, 11, 0.12), transparent 32%),
    #0f1115;
}

.facet {
  position: absolute;
  display: block;
  clip-path: polygon(50% 0, 100% 50%, 50% 100%, 0 50%);
  opacity: 0.72;
}

.facet--one {
  width: 34rem;
  height: 34rem;
  right: 7%;
  top: 10%;
  background: linear-gradient(135deg, rgba(96, 165, 250, 0.3), rgba(34, 211, 238, 0.08));
}

.facet--two {
  width: 20rem;
  height: 20rem;
  right: 30%;
  bottom: 5%;
  background: linear-gradient(135deg, rgba(246, 163, 93, 0.24), rgba(96, 165, 250, 0.08));
}

.facet--three {
  width: 24rem;
  height: 24rem;
  left: -7rem;
  top: 15%;
  background: linear-gradient(135deg, rgba(96, 165, 250, 0.18), rgba(245, 158, 11, 0.05));
}

.facet--four {
  width: 16rem;
  height: 16rem;
  left: 22%;
  bottom: -5rem;
  background: linear-gradient(135deg, rgba(34, 211, 238, 0.2), rgba(246, 163, 93, 0.08));
}

.hero-screens {
  position: absolute;
  inset: 0;
  z-index: -2;
  pointer-events: none;
}

.hero-screen {
  position: absolute;
  display: block;
  border: 1px solid rgba(148, 163, 184, 0.3);
  border-radius: 8px;
  box-shadow: 0 32px 80px rgba(0, 0, 0, 0.38);
}

.hero-screen--main {
  width: min(54rem, 58%);
  right: 4%;
  top: 15%;
}

.hero-screen--chat {
  width: min(35rem, 36%);
  right: 28%;
  bottom: 10%;
}

.hero-screen--feeds {
  width: min(26rem, 28%);
  right: -4%;
  bottom: 5%;
}

.hero-copy {
  width: min(42rem, 100%);
  margin-left: max(0rem, calc((100% - 1180px) / 2));
  position: relative;
  z-index: 1;
}

.hero-mark {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
  margin-bottom: 1.2rem;
  color: #d6dce7;
  font-size: 0.94rem;
  font-weight: 720;
}

.hero-copy h1 {
  margin: 0;
  font-size: 5.3rem;
  line-height: 0.92;
  font-weight: 820;
}

.hero-lede {
  max-width: 38rem;
  margin: 1.25rem 0 0;
  color: #d6dce7;
  font-size: 1.18rem;
  line-height: 1.65;
}

.hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin-top: 1.7rem;
}

.primary-action,
.secondary-action {
  min-height: 2.9rem;
  padding-inline: 1.2rem;
}

.hero-points {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
  list-style: none;
  margin: 1.5rem 0 0;
  padding: 0;
}

.hero-points li {
  border: 1px solid rgba(148, 163, 184, 0.24);
  border-radius: 999px;
  background: rgba(17, 24, 39, 0.72);
  color: #d6dce7;
  padding: 0.45rem 0.72rem;
  font-size: 0.84rem;
  font-weight: 680;
}

.capability-section,
.notebook-section,
.screenshot-section,
.workflow-section {
  padding: 5rem 2rem;
}

.capability-section {
  background: #f6f7f9;
  color: #111827;
}

.notebook-section {
  background: #ffffff;
  color: #111827;
}

.screenshot-section {
  background:
    linear-gradient(135deg, rgba(37, 99, 235, 0.16), transparent 42%),
    linear-gradient(235deg, rgba(245, 158, 11, 0.1), transparent 36%),
    #111827;
}

.workflow-inner {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(20rem, 1.1fr);
  gap: 3rem;
  align-items: center;
  width: min(1180px, 100%);
  margin-inline: auto;
}

.workflow-section {
  background: #f6f7f9;
  color: #111827;
}

.section-heading,
.notebook-inner,
.landing-footer {
  width: min(1180px, 100%);
  margin-inline: auto;
}

.section-heading {
  display: grid;
  gap: 0.75rem;
  margin-bottom: 2rem;
}

.section-heading h2,
.workflow-copy h2 {
  max-width: 48rem;
  margin: 0;
  font-size: 2.35rem;
  line-height: 1.08;
}

.section-heading p,
.workflow-copy p {
  max-width: 48rem;
  margin: 0;
  color: #5f6b7a;
  font-size: 1rem;
  line-height: 1.7;
}

.section-heading--dark {
  color: #f4f7fb;
}

.section-heading--dark p {
  color: #b7c0ce;
}

.section-kicker {
  color: #2563eb;
  font-size: 0.78rem;
  font-weight: 820;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.section-heading--dark .section-kicker {
  color: #7dd3fc;
}

.capability-grid {
  width: min(1180px, 100%);
  margin-inline: auto;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1rem;
}

.capability-card {
  display: grid;
  align-content: start;
  gap: 0.75rem;
  min-height: 13.5rem;
  padding: 1.25rem;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  background: #ffffff;
}

.capability-icon {
  display: inline-grid;
  place-items: center;
  width: 2.6rem;
  height: 2.6rem;
  border: 1px solid #c9d7ef;
  border-radius: 8px;
  color: #2563eb;
  background: #eef4ff;
}

.capability-card h3 {
  margin: 0;
  font-size: 1.05rem;
}

.capability-card p {
  margin: 0;
  color: #5f6b7a;
  font-size: 0.95rem;
  line-height: 1.6;
}

.notebook-inner {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(21rem, 0.58fr);
  align-items: center;
  gap: 3rem;
}

.notebook-copy {
  display: grid;
  gap: 0.85rem;
}

.notebook-copy h2 {
  max-width: 48rem;
  margin: 0;
  font-size: 2.35rem;
  line-height: 1.08;
}

.notebook-copy p {
  max-width: 48rem;
  margin: 0;
  color: #5f6b7a;
  font-size: 1rem;
  line-height: 1.7;
}

.notebook-panel {
  overflow: hidden;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  background:
    linear-gradient(135deg, rgba(37, 99, 235, 0.08), transparent 54%),
    #f6f7f9;
}

.notebook-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.1rem;
  border-bottom: 1px solid #d9dee7;
}

.notebook-panel-header span {
  color: #5f6b7a;
  font-size: 0.82rem;
  font-weight: 760;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.notebook-panel-header strong {
  border: 1px solid rgba(34, 197, 94, 0.26);
  border-radius: 999px;
  background: rgba(34, 197, 94, 0.1);
  color: #166534;
  padding: 0.28rem 0.56rem;
  font-size: 0.78rem;
}

.notebook-panel ul {
  display: grid;
  gap: 0.7rem;
  margin: 0;
  padding: 1.1rem;
  list-style: none;
}

.notebook-panel li {
  display: flex;
  align-items: center;
  gap: 0.62rem;
  min-height: 2.5rem;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  background: #ffffff;
  color: #111827;
  padding: 0.7rem 0.78rem;
  font-size: 0.93rem;
  font-weight: 680;
}

.notebook-panel li .app-icon {
  color: #2563eb;
}

.screenshot-grid {
  width: min(1180px, 100%);
  margin-inline: auto;
  display: grid;
  gap: 1.2rem;
}

.screenshot-card {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(18rem, 0.65fr);
  align-items: stretch;
  gap: 0;
  margin: 0;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: 8px;
  background: #0f1115;
}

.screenshot-card img {
  width: 100%;
  height: 100%;
  min-height: 18rem;
  display: block;
  object-fit: cover;
}

.screenshot-card figcaption {
  display: grid;
  align-content: center;
  gap: 0.75rem;
  padding: 1.5rem;
  border-left: 1px solid rgba(148, 163, 184, 0.22);
}

.screenshot-card strong {
  font-size: 1.3rem;
}

.screenshot-card span {
  color: #b7c0ce;
  line-height: 1.65;
}

.workflow-copy {
  display: grid;
  gap: 0.8rem;
}

.workflow-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.8rem;
}

.workflow-list li {
  position: relative;
  min-height: 9.8rem;
  padding: 1rem;
  overflow: hidden;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  background:
    linear-gradient(135deg, rgba(37, 99, 235, 0.1), transparent 50%),
    #f6f7f9;
}

.workflow-list li::after {
  content: '';
  position: absolute;
  right: -2.4rem;
  bottom: -2.4rem;
  width: 6rem;
  height: 6rem;
  clip-path: polygon(50% 0, 100% 50%, 50% 100%, 0 50%);
  background: linear-gradient(135deg, rgba(96, 165, 250, 0.28), rgba(245, 158, 11, 0.16));
}

.workflow-list span {
  color: #2563eb;
  font-size: 0.82rem;
  font-weight: 820;
}

.workflow-list strong {
  display: block;
  margin-top: 3.4rem;
  font-size: 1.1rem;
}

.about-section {
  padding: 3rem 2rem 1rem;
  border-top: 1px solid rgba(148, 163, 184, 0.18);
}

.about-inner {
  width: min(1180px, 100%);
  margin-inline: auto;
  display: grid;
  gap: 0.65rem;
}

.about-section h2 {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 760;
  color: #f4f7fb;
}

.about-section p {
  margin: 0;
  max-width: 42rem;
  color: #b7c0ce;
  font-size: 0.95rem;
  line-height: 1.65;
}

.about-section a {
  color: #7dd3fc;
  font-weight: 650;
  text-decoration: none;
}

.about-section a:hover {
  color: #f4f7fb;
  text-decoration: underline;
}

.landing-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.5rem 2rem 2rem;
  color: #b7c0ce;
}

@media (max-width: 1050px) {
  .nav-links {
    display: none;
  }

  .landing-nav {
    grid-template-columns: auto auto;
    justify-content: space-between;
  }

  .hero-section {
    min-height: auto;
    padding-top: 3rem;
  }

  .hero-screens {
    position: relative;
    inset: auto;
    z-index: 0;
    display: grid;
    grid-template-columns: 1fr;
    gap: 0.8rem;
    margin-top: 2.2rem;
  }

  .hero-screen {
    position: static;
    width: 100%;
  }

  .hero-screen--chat,
  .hero-screen--feeds {
    display: none;
  }

  .hero-copy {
    margin-left: 0;
  }

  .capability-grid,
  .workflow-list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .notebook-inner {
    grid-template-columns: 1fr;
  }

  .screenshot-card {
    grid-template-columns: 1fr;
  }

  .screenshot-card figcaption {
    border-left: 0;
    border-top: 1px solid rgba(148, 163, 184, 0.22);
  }

  .workflow-inner {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 680px) {
  .landing-nav {
    min-height: auto;
    padding: 0.8rem 1rem;
  }

  .nav-actions {
    gap: 0.45rem;
  }

  .nav-link-button {
    display: none;
  }

  .nav-primary {
    min-height: 2.3rem;
    padding-inline: 0.85rem;
  }

  .hero-section,
  .capability-section,
  .notebook-section,
  .screenshot-section,
  .workflow-section,
  .about-section {
    padding-inline: 1rem;
  }

  .hero-copy h1 {
    font-size: 3.6rem;
  }

  .hero-lede {
    font-size: 1rem;
  }

  .section-heading h2,
  .notebook-copy h2,
  .workflow-copy h2 {
    font-size: 1.75rem;
  }

  .capability-grid,
  .workflow-list {
    grid-template-columns: 1fr;
  }

  .capability-card {
    min-height: auto;
  }

  .screenshot-card img {
    min-height: 12rem;
  }

  .landing-footer {
    align-items: flex-start;
    flex-direction: column;
    padding-inline: 1rem;
  }
}
</style>
