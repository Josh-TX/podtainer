<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { stacksApi } from '../api'

const props = defineProps({ isNew: { type: Boolean, default: false } })
const route = useRoute()
const router = useRouter()

const name = ref(props.isNew ? '' : route.params.name)
const content = ref(props.isNew ? 'services:\n  web:\n    image: docker.io/library/nginx:latest\n    ports:\n      - "8080:80"\n' : '')
const status = ref(null)
const error = ref('')
const busy = ref(false)
const logsByService = ref({})
const loading = ref(!props.isNew)
let firstLoad = true

async function load() {
  if (props.isNew) return
  error.value = ''
  if (firstLoad) loading.value = true
  try {
    const data = await stacksApi.get(name.value)
    content.value = data.content
    status.value = data.status
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
    firstLoad = false
  }
}

async function saveAndDeploy(force = false) {
  error.value = ''
  busy.value = true
  try {
    await stacksApi.deploy(name.value, content.value, force)
    if (props.isNew) {
      router.push(`/stacks/${name.value}`)
    } else {
      await load()
    }
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function pullAndRestart() {
  error.value = ''
  busy.value = true
  try {
    await stacksApi.pull(name.value)
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function remove() {
  if (!confirm(`Delete stack "${name.value}"? This stops and removes its units and compose file. Named volumes are preserved.`)) return
  error.value = ''
  busy.value = true
  try {
    await stacksApi.delete(name.value)
    router.push('/stacks')
  } catch (e) {
    error.value = e.message
    busy.value = false
  }
}

function serviceFromFilename(filename) {
  // "<stack>-<service>.container" -> "<service>"
  const base = filename.replace(/\.container$/, '')
  return base.slice(name.value.length + 1)
}

async function toggleLogs(service) {
  if (logsByService.value[service] !== undefined) {
    delete logsByService.value[service]
    return
  }
  try {
    const { logs } = await stacksApi.serviceLogs(name.value, service)
    logsByService.value = { ...logsByService.value, [service]: logs }
  } catch (e) {
    logsByService.value = { ...logsByService.value, [service]: 'Error: ' + e.message }
  }
}

const containerUnits = computed(() => (status.value?.units || []).filter((u) => u.filename.endsWith('.container')))

onMounted(load)
</script>

<template>
  <div class="page-header">
    <h1>{{ isNew ? 'New Stack' : name }}</h1>
    <RouterLink to="/stacks" role="button" class="secondary">Back</RouterLink>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>
  <p v-if="loading" aria-busy="true">Loading…</p>

  <template v-else>
    <label>
      Stack name
      <input v-if="isNew" v-model="name" placeholder="mystack" />
    </label>

    <div v-if="!isNew && status" class="toolbar">
      <span v-if="!status.deployed" class="badge notdeployed">Not Deployed</span>
      <span v-else-if="status.drift" class="badge drift">Needs Redeploy</span>
    </div>

    <label>
      docker-compose.yml
      <textarea v-model="content" rows="16"></textarea>
    </label>

    <div class="toolbar">
      <button :disabled="busy || !name" @click="saveAndDeploy(false)">Save &amp; Deploy</button>
      <button v-if="!isNew" class="secondary" :disabled="busy" @click="saveAndDeploy(true)">Force Redeploy</button>
      <button v-if="!isNew" class="secondary" :disabled="busy" @click="pullAndRestart">Pull &amp; Restart</button>
      <button v-if="!isNew" class="danger" :disabled="busy" @click="remove">Delete</button>
    </div>

    <template v-if="!isNew && status">
      <h2>Services</h2>
      <table>
        <thead>
          <tr>
            <th>Service</th>
            <th>Status</th>
            <th>Health</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in containerUnits" :key="u.filename">
            <td>{{ serviceFromFilename(u.filename) }}</td>
            <td><span :class="['badge', u.active]">{{ u.active }}</span></td>
            <td><span :class="['badge', u.health]">{{ u.health }}</span></td>
            <td><button class="secondary" @click="toggleLogs(serviceFromFilename(u.filename))">
              {{ logsByService[serviceFromFilename(u.filename)] !== undefined ? 'Hide Logs' : 'View Logs' }}
            </button></td>
          </tr>
        </tbody>
      </table>
      <template v-for="(logs, service) in logsByService" :key="service">
        <p class="muted">{{ service }}</p>
        <pre class="logs">{{ logs }}</pre>
      </template>
    </template>
  </template>
</template>
