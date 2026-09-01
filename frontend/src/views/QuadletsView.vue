<script setup>
import { ref, onMounted } from 'vue'
import { quadletsApi } from '../api'
import RowTable from '../components/RowTable.vue'

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

  <div v-if="loading" class="loading"><span class="spinner"></span> Loading…</div>
  <template v-else>
    <RowTable>
      <div class="row-table-row row-table-head">
        <div>Filename</div>
        <div>Type</div>
        <div>Stack</div>
        <div>Status</div>
      </div>
      <RouterLink v-for="f in files" :key="f.filename" :to="`/quadlets/${encodeURIComponent(f.filename)}`" class="row-table-row">
        <div>{{ f.filename }}</div>
        <div>{{ f.type }}</div>
        <div class="muted">{{ f.stack || 'external' }}</div>
        <div><span :class="['badge', f.active]">{{ f.active }}</span></div>
      </RouterLink>
    </RowTable>
    <p v-if="!files.length" class="muted">No quadlet files found.</p>
  </template>
</template>
