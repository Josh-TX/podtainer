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
          <span v-else-if="s.drift" class="badge drift">Needs Redeploy</span>
          <span v-else :class="['badge', healthClass(healthRatio(s.podmanContainers))]">{{ healthRatio(s.podmanContainers).success }}/{{ healthRatio(s.podmanContainers).total }}</span>
        </td>
        <td class="muted">{{ s.podmanContainers.length }}</td>
      </tr>
    </tbody>
  </table>
  <p v-if="!loading && !stacks.length" class="muted">No stacks yet.</p>
</template>
