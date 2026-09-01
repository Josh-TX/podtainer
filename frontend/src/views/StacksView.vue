<script setup>
import { ref, onMounted } from 'vue'
import { stacksApi } from '../api'

const stacks = ref([])
const error = ref('')
const loading = ref(true)

function aggregateActive(units) {
  if (!units.length) return 'unknown'
  if (units.some((u) => u.active === 'failed')) return 'failed'
  if (units.every((u) => u.active === 'active')) return 'active'
  return 'inactive'
}

function aggregateHealth(units) {
  const healths = units.map((u) => u.health).filter((h) => h && h !== 'none')
  if (healths.includes('unhealthy')) return 'unhealthy'
  if (healths.includes('starting')) return 'starting'
  if (healths.includes('healthy')) return 'healthy'
  return 'none'
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
        <th>Health</th>
        <th>Services</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="s in stacks" :key="s.name">
        <td><RouterLink class="row-link" :to="`/stacks/${encodeURIComponent(s.name)}`">{{ s.name }}</RouterLink></td>
        <td>
          <span v-if="!s.deployed" class="badge notdeployed">Not Deployed</span>
          <span v-else-if="s.drift" class="badge drift">Needs Redeploy</span>
          <span v-else :class="['badge', aggregateActive(s.units)]">{{ aggregateActive(s.units) }}</span>
        </td>
        <td><span :class="['badge', aggregateHealth(s.units)]">{{ aggregateHealth(s.units) }}</span></td>
        <td class="muted">{{ s.units.filter(u => u.filename.endsWith('.container')).length }}</td>
      </tr>
    </tbody>
  </table>
  <p v-if="!loading && !stacks.length" class="muted">No stacks yet.</p>
</template>
