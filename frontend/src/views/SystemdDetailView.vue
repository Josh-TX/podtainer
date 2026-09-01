<script setup>
import { ref, onMounted } from 'vue'
import { systemdApi } from '../api'

const props = defineProps({ name: { type: String, required: true } })

const unit = ref(null)
const error = ref('')
const loading = ref(true)

async function load() {
  error.value = ''
  loading.value = true
  try {
    const units = await systemdApi.list()
    unit.value = units.find((u) => u.name === props.name) || null
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page-header">
    <h1>{{ name }}</h1>
    <RouterLink to="/systemd" role="button" class="secondary">Back</RouterLink>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>
  <p v-else-if="loading" aria-busy="true">Loading…</p>
  <p v-else-if="!unit" class="muted">Unit not found.</p>

  <template v-else>
    <div class="toolbar">
      <span :class="['badge', unit.active]">{{ unit.active }}</span>
      <span class="muted">{{ unit.sub }}</span>
    </div>
    <p><span class="muted">Description:</span> {{ unit.description || '—' }}</p>
    <p><span class="muted">Source path:</span> {{ unit.sourcePath || '—' }}</p>
  </template>
</template>
