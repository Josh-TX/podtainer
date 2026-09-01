<script setup>
import { ref, onMounted } from 'vue'
import { containersApi } from '../api'
import RowTable from '../components/RowTable.vue'

const containers = ref([])
const error = ref('')
const loading = ref(true)

async function load() {
  error.value = ''
  loading.value = true
  try {
    containers.value = await containersApi.list()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <h1>Containers</h1>
  <p class="muted">Every container visible to podman ps -a, not just quadlet-managed ones.</p>

  <div v-if="error" class="error-banner">{{ error }}</div>

  <div v-if="loading" class="loading"><span class="spinner"></span> Loading…</div>
  <template v-else>
    <RowTable>
      <div class="row-table-row row-table-head">
        <div>Name</div>
        <div>Image</div>
        <div>State</div>
        <div>Status</div>
        <div>Managed by</div>
      </div>
      <RouterLink v-for="c in containers" :key="c.id" :to="`/containers/${encodeURIComponent(c.id)}`" class="row-table-row">
        <div>{{ c.names?.[0] || c.id.slice(0, 12) }}</div>
        <div class="muted">{{ c.image }}</div>
        <div><span :class="['badge', c.state]">{{ c.state }}</span></div>
        <div class="muted">{{ c.status }}</div>
        <div class="muted">{{ c.systemdUnit || '—' }}</div>
      </RouterLink>
    </RowTable>
    <p v-if="!containers.length" class="muted">No containers found.</p>
  </template>
</template>
