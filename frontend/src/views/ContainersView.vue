<script setup>
import { ref, onMounted } from 'vue'
import { containersApi } from '../api'

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

  <p v-if="loading" aria-busy="true">Loading…</p>
  <template v-else>
    <table class="rows">
      <thead>
        <tr>
          <th>Name</th>
          <th>Image</th>
          <th>State</th>
          <th>Status</th>
          <th>Managed by</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="c in containers" :key="c.id">
          <td><RouterLink class="row-link" :to="`/containers/${encodeURIComponent(c.id)}`">{{ c.names?.[0] || c.id.slice(0, 12) }}</RouterLink></td>
          <td class="muted">{{ c.image }}</td>
          <td><span :class="['badge', c.state]">{{ c.state }}</span></td>
          <td class="muted">{{ c.status }}</td>
          <td class="muted">{{ c.systemdUnit || '—' }}</td>
        </tr>
      </tbody>
    </table>
    <p v-if="!containers.length" class="muted">No containers found.</p>
  </template>
</template>
