<script setup>
import { ref, onMounted } from 'vue'
import { volumesApi } from '../api'
import { timeAgo, fullDate } from '../utils/format'

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
