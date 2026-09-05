<script setup>
import { ref, onMounted } from 'vue'
import { volumesApi } from '../api'

const volumes = ref([])
const error = ref('')
const loading = ref(true)

async function load() {
  error.value = ''
  loading.value = true
  try {
    volumes.value = await volumesApi.list()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function timeAgo(dateStr) {
  const sec = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000)
  if (sec < 60) return 'just now'
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min} minute${min === 1 ? '' : 's'} ago`
  const hr = Math.floor(min / 60)
  if (hr < 48) return `${hr} hour${hr === 1 ? '' : 's'} ago`
  const day = Math.floor(hr / 24)
  if (day < 90) return `${day} day${day === 1 ? '' : 's'} ago`
  return new Date(dateStr).toLocaleDateString()
}

function fullDate(dateStr) {
  return new Date(dateStr).toLocaleString()
}

onMounted(load)
</script>

<template>
  <h1>Volumes</h1>
  <p class="muted">Every podman volume on the host, not just ones created by a stack.</p>

  <div v-if="error" class="error-banner">{{ error }}</div>

  <p v-if="loading" aria-busy="true">Loading…</p>
  <template v-else>
    <table class="rows">
      <thead>
        <tr>
          <th>Name</th>
          <th>Driver</th>
          <th>Created</th>
          <th>Mountpoint</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="v in volumes" :key="v.name">
          <td><RouterLink class="row-link truncate-rtl" :to="`/volumes/${encodeURIComponent(v.name)}`">{{ v.name }}</RouterLink></td>
          <td class="muted">{{ v.driver }}</td>
          <td class="muted no-wrap" :title="fullDate(v.createdAt)">{{ timeAgo(v.createdAt) }}</td>
          <td class="muted"><span class="truncate-rtl">{{ v.mountpoint }}</span></td>
        </tr>
      </tbody>
    </table>
    <p v-if="!volumes.length" class="muted">No volumes found.</p>
  </template>
</template>

<style scoped>
table.rows {
  table-layout: fixed;
  width: 100%;
}
table.rows th:nth-child(2) {
  width: 5.5rem;
}
table.rows th:nth-child(3) {
  width: 7.5rem;
}
.no-wrap {
  white-space: nowrap;
}
.truncate-rtl {
  display: block;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  direction: rtl;
  text-align: left;
}
</style>
