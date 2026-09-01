<script setup>
import { ref, onMounted } from 'vue'
import { systemdApi } from '../api'

const units = ref([])
const error = ref('')

async function load() {
  error.value = ''
  try {
    units.value = await systemdApi.list()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <h1>Systemd</h1>
  <p class="muted">Every systemd --user unit generated from a quadlet file, regardless of origin.</p>

  <div v-if="error" class="error-banner">{{ error }}</div>

  <table>
    <thead>
      <tr>
        <th>Unit</th>
        <th>Active</th>
        <th>Sub</th>
        <th>Description</th>
        <th>Source</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="u in units" :key="u.name">
        <td>{{ u.name }}</td>
        <td><span :class="['badge', u.active]">{{ u.active }}</span></td>
        <td class="muted">{{ u.sub }}</td>
        <td class="muted">{{ u.description }}</td>
        <td class="muted">{{ u.sourcePath }}</td>
      </tr>
      <tr v-if="!units.length">
        <td colspan="5" class="muted">No quadlet-origin units found.</td>
      </tr>
    </tbody>
  </table>
</template>
