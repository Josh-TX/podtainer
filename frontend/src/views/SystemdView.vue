<script setup>
import { ref, onMounted } from 'vue'
import { systemdApi } from '../api'
import RowTable from '../components/RowTable.vue'

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

  <div v-if="loading" class="loading"><span class="spinner"></span> Loading…</div>
  <template v-else>
    <RowTable>
      <div class="row-table-row row-table-head">
        <div>Unit</div>
        <div>Active</div>
        <div>Sub</div>
        <div>Description</div>
      </div>
      <RouterLink v-for="u in units" :key="u.name" :to="`/systemd/${encodeURIComponent(u.name)}`" class="row-table-row">
        <div>{{ u.name }}</div>
        <div><span :class="['badge', u.active]">{{ u.active }}</span></div>
        <div class="muted">{{ u.sub }}</div>
        <div class="muted">{{ u.description }}</div>
      </RouterLink>
    </RowTable>
    <p v-if="!units.length" class="muted">No quadlet-origin units found.</p>
  </template>
</template>
