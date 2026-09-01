<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { quadletsApi } from '../api'

const props = defineProps({ filename: { type: String, required: true } })
const router = useRouter()

const content = ref('')
const meta = ref(null)
const error = ref('')
const logs = ref('')
const showLogs = ref(false)
const loading = ref(true)
let firstLoad = true

async function load() {
  error.value = ''
  if (firstLoad) loading.value = true
  try {
    const [data, list] = await Promise.all([quadletsApi.get(props.filename), quadletsApi.list()])
    content.value = data.content
    meta.value = list.find((f) => f.filename === props.filename) || null
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
    firstLoad = false
  }
}

async function save() {
  error.value = ''
  try {
    await quadletsApi.write(props.filename, content.value)
  } catch (e) {
    error.value = e.message
  }
}

async function action(fn) {
  error.value = ''
  try {
    await fn(props.filename)
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function remove() {
  if (!confirm(`Delete quadlet file "${props.filename}"? This stops the unit and removes the file.`)) return
  error.value = ''
  try {
    await quadletsApi.delete(props.filename)
    router.push('/quadlets')
  } catch (e) {
    error.value = e.message
  }
}

async function viewLogs() {
  showLogs.value = !showLogs.value
  if (!showLogs.value) return
  try {
    const data = await quadletsApi.logs(props.filename)
    logs.value = data.logs
  } catch (e) {
    logs.value = 'Error: ' + e.message
  }
}

onMounted(load)
</script>

<template>
  <div class="page-header">
    <h1>{{ filename }}</h1>
    <RouterLink to="/quadlets"><button>Back</button></RouterLink>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>
  <div v-if="loading" class="loading"><span class="spinner"></span> Loading…</div>

  <template v-else>
    <div v-if="meta?.stack" class="warn-banner">
      This file is owned by stack "{{ meta.stack }}" — edits here will be overwritten on the stack's next redeploy.
      <RouterLink :to="`/stacks/${encodeURIComponent(meta.stack)}`">View stack</RouterLink>
    </div>

    <div v-if="meta" class="toolbar">
      <span :class="['badge', meta.active]">{{ meta.active }}</span>
      <span class="muted">{{ meta.type }}</span>
    </div>

    <textarea v-model="content" rows="16"></textarea>

    <div class="toolbar" style="margin-top: 1rem;">
      <button class="primary" @click="save">Save</button>
      <button @click="action(quadletsApi.start)">Start</button>
      <button @click="action(quadletsApi.stop)">Stop</button>
      <button @click="action(quadletsApi.restart)">Restart</button>
      <button @click="viewLogs">{{ showLogs ? 'Hide Logs' : 'View Logs' }}</button>
      <button class="danger" @click="remove">Delete</button>
    </div>

    <pre v-if="showLogs" class="logs">{{ logs }}</pre>
  </template>
</template>
