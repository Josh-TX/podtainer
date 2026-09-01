<script setup>
import { ref, onMounted } from 'vue'
import { containersApi } from '../api'

const containers = ref([])
const error = ref('')
const expanded = ref(null)
const stats = ref(null)
const logs = ref('')

async function load() {
  error.value = ''
  try {
    containers.value = await containersApi.list()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)

async function toggleExpand(c) {
  if (expanded.value === c.id) {
    expanded.value = null
    return
  }
  expanded.value = c.id
  stats.value = null
  logs.value = ''
  try {
    stats.value = await containersApi.stats(c.id)
  } catch (e) {
    stats.value = { error: e.message }
  }
  try {
    const data = await containersApi.logs(c.id)
    logs.value = data.logs
  } catch (e) {
    logs.value = 'Error: ' + e.message
  }
}

async function action(fn, id) {
  error.value = ''
  try {
    await fn(id)
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function remove(c) {
  if (!confirm(`Remove container "${c.names?.[0] || c.id}"?`)) return
  await action(containersApi.remove, c.id)
  if (expanded.value === c.id) expanded.value = null
}
</script>

<template>
  <h1>Containers</h1>
  <p class="muted">Every container visible to podman ps -a, not just quadlet-managed ones.</p>

  <div v-if="error" class="error-banner">{{ error }}</div>

  <table>
    <thead>
      <tr>
        <th>Name</th>
        <th>Image</th>
        <th>State</th>
        <th>Status</th>
        <th>Managed by</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
      <template v-for="c in containers" :key="c.id">
        <tr>
          <td>{{ c.names?.[0] || c.id.slice(0, 12) }}</td>
          <td class="muted">{{ c.image }}</td>
          <td><span :class="['badge', c.state]">{{ c.state }}</span></td>
          <td class="muted">{{ c.status }}</td>
          <td class="muted">{{ c.systemdUnit || '—' }}</td>
          <td class="toolbar" style="margin:0;">
            <button @click="action(containersApi.start, c.id)">Start</button>
            <button @click="action(containersApi.stop, c.id)">Stop</button>
            <button @click="action(containersApi.restart, c.id)">Restart</button>
            <button @click="toggleExpand(c)">{{ expanded === c.id ? 'Close' : 'Details' }}</button>
            <button class="danger" @click="remove(c)">Remove</button>
          </td>
        </tr>
        <tr v-if="expanded === c.id">
          <td colspan="6">
            <div class="muted" style="margin-bottom: 0.5rem;">
              CPU: {{ stats?.CPU ?? '—' }} &nbsp; Mem: {{ stats?.MemUsage ?? '—' }}
            </div>
            <pre class="logs">{{ logs }}</pre>
          </td>
        </tr>
      </template>
      <tr v-if="!containers.length">
        <td colspan="6" class="muted">No containers found.</td>
      </tr>
    </tbody>
  </table>
</template>
