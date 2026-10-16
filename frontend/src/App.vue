<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { authApi } from './api'
import { authState } from './api/authState.js'
import LoginView from './views/LoginView.vue'

const route = useRoute()
const menuOpen = ref(false)
watch(() => route.path, () => { menuOpen.value = false })
const sections = ['stacks', 'quadlets', 'systemd', 'containers', 'images', 'volumes', 'shell', 'files']
const activeSection = computed(() => sections.find(s => route.path.startsWith(`/${s}`)))

async function refreshAuth() {
  const s = await authApi.status()
  authState.needsSetup = s.needsSetup
  authState.authenticated = s.authenticated
  authState.checked = true
}

async function logout() {
  await authApi.logout()
  authState.authenticated = false
}

onMounted(refreshAuth)
</script>

<template>
  <template v-if="!authState.checked"></template>
  <LoginView v-else-if="!authState.authenticated" />
  <div v-else class="app-shell">
    <div v-if="menuOpen" class="nav-backdrop" @click="menuOpen = false"></div>
    <header class="container-fluid" style="padding-right: 0;">
      <nav>
        <ul>
          <li><strong>Podtainer</strong></li>
          <li v-if="activeSection" class="nav-current">{{ activeSection }}</li>
        </ul>
        <ul class="nav-links" :class="{ open: menuOpen }">
          <li :class="{ active: activeSection === 'stacks' }"><RouterLink to="/stacks">Stacks</RouterLink></li>
          <li :class="{ active: activeSection === 'quadlets' }"><RouterLink to="/quadlets">Quadlets</RouterLink></li>
          <li :class="{ active: activeSection === 'systemd' }"><RouterLink to="/systemd">Systemd</RouterLink></li>
          <li :class="{ active: activeSection === 'containers' }"><RouterLink to="/containers">Containers</RouterLink></li>
          <li :class="{ active: activeSection === 'images' }"><RouterLink to="/images">Images</RouterLink></li>
          <li :class="{ active: activeSection === 'volumes' }"><RouterLink to="/volumes">Volumes</RouterLink></li>
          <li :class="{ active: activeSection === 'shell' }"><RouterLink to="/shell">Shell</RouterLink></li>
          <li :class="{ active: activeSection === 'files' }"><RouterLink to="/files">Files</RouterLink></li>
          <li class="nav-logout"><a href="#" @click.prevent="logout">Log out</a></li>
        </ul>
        <ul class="nav-right" style="margin-left: auto; margin-right: 0;">
          <li class="nav-toggle" style="padding: 0;">
            <button class="outline secondary" aria-label="Toggle menu" :aria-expanded="menuOpen" @click="menuOpen = !menuOpen">☰</button>
          </li>
          <li class="nav-user" style="padding: 0;">
            <details class="dropdown">
              <summary aria-label="User menu" style="border: none; padding: 0.5rem 0.75rem;">⋮</summary>
              <ul dir="rtl">
                <li><a href="#" @click.prevent="logout">Log out</a></li>
              </ul>
            </details>
          </li>
        </ul>
      </nav>
    </header>
    <main class="container-fluid">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.app-shell > header {
  flex: none;
  position: relative;
  z-index: 20;
}
.nav-backdrop {
  position: fixed;
  inset: 0;
  z-index: 10;
}
.app-shell > main {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
nav details.dropdown > summary::after {
  display: none;
}
nav details.dropdown > summary {
  background: none;
}
nav details.dropdown > summary:focus,
nav details.dropdown > summary:focus-visible {
  box-shadow: none;
}
nav details.dropdown > summary:hover,
nav details.dropdown[open] > summary {
  background-color: var(--pico-dropdown-hover-background-color);
}
.nav-toggle, .nav-logout, .nav-current { display: none; }
.nav-current { text-transform: capitalize; color: var(--pico-muted-color); }
.nav-toggle button {
  margin: 0;
  border: none;
  padding: 0.25rem 0.75rem;
  font-size: 1.35rem;
  line-height: 1;
  width: auto;
}
@media (max-width: 900px) {
  nav { flex-wrap: wrap; }
  .nav-toggle, .nav-logout { display: block; list-style: none; }
  .nav-user { display: none; }
  .nav-current { display: block; }
  .nav-right { display: flex; align-items: center; }
  .nav-links {
    display: flex;
    visibility: hidden;
    transform: translateY(-100%);
    clip-path: inset(100% -24px -24px -24px);
    transition: transform 150ms, clip-path 150ms, visibility 150ms;
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    flex-direction: column;
    margin: 0;
    padding: 0.25rem var(--pico-spacing);
    background: var(--pico-code-background-color);
    border-bottom: var(--pico-border-width) solid var(--pico-table-border-color);
    box-shadow: 0 8px 16px rgba(0, 0, 0, 0.25);
  }
  .nav-links.open { visibility: visible; transform: none; clip-path: inset(0 -24px -24px -24px); }
  .nav-links li, .nav-links a { width: 100%; }
  .nav-links li.active::after {
    top: 0;
    right: auto;
    width: 3px;
    height: auto;
  }
}
</style>
