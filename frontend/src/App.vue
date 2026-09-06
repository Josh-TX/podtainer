<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const sections = ['stacks', 'quadlets', 'systemd', 'containers', 'images', 'volumes']
const activeSection = computed(() => sections.find(s => route.path.startsWith(`/${s}`)))
</script>

<template>
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
    </nav>
  </header>
  <main class="container-fluid">
    <RouterView />
  </main>
</template>
