<script setup>
import { ref, onMounted } from 'vue'
import { systemdApi } from '../api'
import { unitBadgeClass, restartingLabel, isRestarting } from '../unitBadge'

const props = defineProps({ name: { type: String, required: true } })

const unit = ref(null)
const content = ref('')
const error = ref('')
const loading = ref(true)
const logs = ref('')
const showLogs = ref(false)

async function load() {
  error.value = ''
  loading.value = true
  try {
    const units = await systemdApi.list()
    unit.value = units.find((u) => u.name === props.name) || null
    if (unit.value) {
      const data = await systemdApi.content(props.name)
      content.value = data.content
    }
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function viewLogs() {
  showLogs.value = !showLogs.value
  if (!showLogs.value) return
  try {
    const data = await systemdApi.logs(props.name)
    logs.value = data.logs
  } catch (e) {
    logs.value = 'Error: ' + e.message
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
      <span :class="unitBadgeClass(unit)">{{ unit.active }}</span>
      <span class="muted">{{ unit.sub }}</span>
      <span v-if="isRestarting(unit)" class="muted">{{ restartingLabel(unit) }}</span>
    </div>
    <p><span class="muted">Description:</span> {{ unit.description || '—' }}</p>
    <p><span class="muted">Source path:</span> {{ unit.sourcePath || '—' }}</p>

    <label>
      Unit file
      <pre class="logs">{{ content }}</pre>
    </label>

    <button class="secondary" @click="viewLogs">{{ showLogs ? 'Hide Logs' : 'View Logs' }}</button>
    <pre v-if="showLogs" class="logs">{{ logs }}</pre>
  </template>
</template>
