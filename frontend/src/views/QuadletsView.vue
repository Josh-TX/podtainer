<script setup>
import { ref, onMounted } from 'vue'
import { quadletsApi } from '../api'

const files = ref([])
const error = ref('')
const loading = ref(true)

async function load() {
  error.value = ''
  loading.value = true
  try {
    files.value = await quadletsApi.list()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <h1>Quadlets</h1>
  <p class="muted">Every quadlet unit file in the search path, including ones not managed by Podtainer.</p>

  <div v-if="error" class="error-banner">{{ error }}</div>

  <p v-if="loading" aria-busy="true">Loading…</p>
  <template v-else>
    <table class="rows">
      <thead>
        <tr>
          <th>Filename</th>
          <th>Type</th>
          <th>Stack</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="f in files" :key="f.filename">
          <td><RouterLink class="row-link" :to="`/quadlets/${encodeURIComponent(f.filename)}`">{{ f.filename }}</RouterLink></td>
          <td>{{ f.type }}</td>
          <td class="muted">{{ f.stack || 'external' }}</td>
          <td><span :class="['badge', f.active]">{{ f.active }}</span></td>
        </tr>
      </tbody>
    </table>
    <p v-if="!files.length" class="muted">No quadlet files found.</p>
  </template>
</template>
