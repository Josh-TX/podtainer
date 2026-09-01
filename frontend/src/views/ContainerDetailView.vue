<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { containersApi } from '../api'

const props = defineProps({ id: { type: String, required: true } })
const router = useRouter()

const container = ref(null)
const stats = ref(null)
const logs = ref('')
const error = ref('')
const loading = ref(true)
let firstLoad = true

async function load() {
  error.value = ''
  if (firstLoad) loading.value = true
  try {
    const list = await containersApi.list()
    container.value = list.find((c) => c.id === props.id) || null
  } catch (e) {
    error.value = e.message
  }
  try {
    stats.value = await containersApi.stats(props.id)
  } catch (e) {
    stats.value = { error: e.message }
  }
  try {
    const data = await containersApi.logs(props.id)
    logs.value = data.logs
  } catch (e) {
    logs.value = 'Error: ' + e.message
  } finally {
    loading.value = false
    firstLoad = false
  }
}

async function action(fn) {
  error.value = ''
  try {
    await fn(props.id)
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function remove() {
  if (!confirm(`Remove container "${container.value?.names?.[0] || props.id}"?`)) return
  error.value = ''
  try {
    await containersApi.remove(props.id)
    router.push('/containers')
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div class="page-header">
    <h1>{{ container?.names?.[0] || id.slice(0, 12) }}</h1>
    <RouterLink to="/containers"><button>Back</button></RouterLink>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>
  <div v-else-if="loading" class="loading"><span class="spinner"></span> Loading…</div>
  <p v-else-if="!container" class="muted">Container not found.</p>

  <template v-else>
    <div class="toolbar">
      <span :class="['badge', container.state]">{{ container.state }}</span>
      <span class="muted">{{ container.image }}</span>
      <span class="muted">{{ container.status }}</span>
      <span v-if="container.systemdUnit" class="muted">managed by {{ container.systemdUnit }}</span>
    </div>

    <div class="toolbar">
      <button @click="action(containersApi.start)">Start</button>
      <button @click="action(containersApi.stop)">Stop</button>
      <button @click="action(containersApi.restart)">Restart</button>
      <button class="danger" @click="remove">Remove</button>
    </div>

    <div class="muted" style="margin-bottom: 0.5rem;">
      CPU: {{ stats?.CPU ?? '—' }} &nbsp; Mem: {{ stats?.MemUsage ?? '—' }}
    </div>
    <pre class="logs">{{ logs }}</pre>
  </template>
</template>
