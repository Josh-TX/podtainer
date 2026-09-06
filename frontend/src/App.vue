<script setup>
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { authApi } from './api'
import { authState } from './api/authState.js'
import LoginView from './views/LoginView.vue'

const route = useRoute()
const sections = ['stacks', 'quadlets', 'systemd', 'containers', 'images', 'volumes']
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
  <template v-else>
    <header class="container-fluid">
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
        </ul>
        <ul>
          <li><a href="#" @click.prevent="logout">Log out</a></li>
        </ul>
      </nav>
    </header>
    <main class="container-fluid">
      <RouterView />
    </main>
  </template>
</template>
