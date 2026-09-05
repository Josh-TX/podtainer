<script setup>
import { ref, onMounted } from 'vue'
import { stacksApi } from '../api'

const stacks = ref([])
const error = ref('')
const loading = ref(true)

function healthRatio(containers) {
  const total = containers.length
  const success = containers.filter((c) => c.state === 'running' && (c.health === 'healthy' || c.health === 'none')).length
  return { success, total }
}

function healthClass({ success, total }) {
  if (total === 0 || success === 0) return 'unhealthy'
  if (success === total) return 'healthy'
  return 'starting'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    stacks.value = await stacksApi.list()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page-header">
    <h1>Stacks</h1>
    <RouterLink to="/stacks/new" role="button">New Stack</RouterLink>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>
  <p v-if="loading" aria-busy="true">Loading…</p>

  <table v-else class="rows">
    <thead>
      <tr>
        <th>Name</th>
        <th>Status</th>
        <th>Services</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="s in stacks" :key="s.name">
        <td><RouterLink class="row-link" :to="`/stacks/${encodeURIComponent(s.name)}`">{{ s.name }}</RouterLink></td>
        <td>
          <span v-if="!s.deployed" class="badge notdeployed">Not Deployed</span>
          <span v-else :class="['badge', healthClass(healthRatio(s.podmanContainers))]">{{ healthRatio(s.podmanContainers).success }}/{{ healthRatio(s.podmanContainers).total }}</span>
          <span v-if="s.drift" class="drift-icon" data-tooltip="Deployed quadlet units differ from the compose file — redeploy to apply changes">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="m18.84 12.25 1.72-1.71a5 5 0 0 0-.12-7.07 5.006 5.006 0 0 0-6.95 0l-1.72 1.71" />
              <path d="m5.17 11.75-1.71 1.71a5.004 5.004 0 0 0 .12 7.07 5.006 5.006 0 0 0 6.95 0l1.71-1.71" />
              <line x1="8" x2="8" y1="2" y2="5" />
              <line x1="2" x2="5" y1="8" y2="8" />
              <line x1="16" x2="16" y1="19" y2="22" />
              <line x1="19" x2="22" y1="16" y2="16" />
            </svg>
          </span>
        </td>
        <td class="muted">{{ s.podmanContainers.length }}</td>
      </tr>
    </tbody>
  </table>
  <p v-if="!loading && !stacks.length" class="muted">No stacks yet.</p>
</template>
