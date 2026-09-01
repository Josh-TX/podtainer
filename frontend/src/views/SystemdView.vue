<script setup>
import { ref, onMounted } from 'vue'
import { systemdApi } from '../api'

const units = ref([])
const error = ref('')
const loading = ref(true)

async function load() {
  error.value = ''
  loading.value = true
  try {
    units.value = await systemdApi.list()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <h1>Systemd</h1>
  <p class="muted">Every systemd --user unit generated from a quadlet file, regardless of origin.</p>

  <div v-if="error" class="error-banner">{{ error }}</div>

  <p v-if="loading" aria-busy="true">Loading…</p>
  <template v-else>
    <table class="rows">
      <thead>
        <tr>
          <th>Unit</th>
          <th>Active</th>
          <th>Sub</th>
          <th>Description</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="u in units" :key="u.name">
          <td><RouterLink class="row-link" :to="`/systemd/${encodeURIComponent(u.name)}`">{{ u.name }}</RouterLink></td>
          <td><span :class="['badge', u.active]">{{ u.active }}</span></td>
          <td class="muted">{{ u.sub }}</td>
          <td class="muted">{{ u.description }}</td>
        </tr>
      </tbody>
    </table>
    <p v-if="!units.length" class="muted">No quadlet-origin units found.</p>
  </template>
</template>
