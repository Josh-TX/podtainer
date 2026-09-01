<script setup>
import { ref, onMounted } from 'vue'
import { stacksApi } from '../api'
import RowTable from '../components/RowTable.vue'

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
    <RouterLink to="/stacks/new"><button class="primary">New Stack</button></RouterLink>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>
  <div v-if="loading" class="loading"><span class="spinner"></span> Loading…</div>

  <RowTable v-else>
    <div class="row-table-row row-table-head">
      <div>Name</div>
      <div>Status</div>
      <div>Health</div>
      <div>Services</div>
    </div>
    <RouterLink v-for="s in stacks" :key="s.name" :to="`/stacks/${encodeURIComponent(s.name)}`" class="row-table-row">
      <div>{{ s.name }}</div>
      <div>
        <span v-if="!s.deployed" class="badge notdeployed">Not Deployed</span>
        <span v-else-if="s.drift" class="badge drift">Needs Redeploy</span>
        <span v-else :class="['badge', aggregateActive(s.units)]">{{ aggregateActive(s.units) }}</span>
      </div>
      <div><span :class="['badge', aggregateHealth(s.units)]">{{ aggregateHealth(s.units) }}</span></div>
      <div class="muted">{{ s.units.filter(u => u.filename.endsWith('.container')).length }}</div>
    </RouterLink>
  </RowTable>
  <p v-if="!loading && !stacks.length" class="muted">No stacks yet.</p>
</template>
