<script setup>
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { authApi } from './api'
import { authState } from './api/authState.js'
import LoginView from './views/LoginView.vue'

const route = useRoute()
const sections = ['stacks', 'quadlets', 'systemd', 'containers', 'images', 'volumes', 'shell']
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
    <header class="container-fluid" style="padding-right: 0;">
      <nav>
        <ul>
          <li><strong>Podtainer</strong></li>
        </ul>
        <ul>
          <li :class="{ active: activeSection === 'stacks' }"><RouterLink to="/stacks">Stacks</RouterLink></li>
          <li :class="{ active: activeSection === 'quadlets' }"><RouterLink to="/quadlets">Quadlets</RouterLink></li>
          <li :class="{ active: activeSection === 'systemd' }"><RouterLink to="/systemd">Systemd</RouterLink></li>
          <li :class="{ active: activeSection === 'containers' }"><RouterLink to="/containers">Containers</RouterLink></li>
          <li :class="{ active: activeSection === 'images' }"><RouterLink to="/images">Images</RouterLink></li>
          <li :class="{ active: activeSection === 'volumes' }"><RouterLink to="/volumes">Volumes</RouterLink></li>
          <li :class="{ active: activeSection === 'shell' }"><RouterLink to="/shell">Shell</RouterLink></li>
        </ul>
        <ul style="margin-left: auto; margin-right: 0;">
          <li style="padding: 0;">
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
</style>
