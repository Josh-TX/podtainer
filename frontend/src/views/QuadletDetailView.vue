<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { quadletsApi, systemdApi, containersApi } from '../api'

const props = defineProps({ filename: { type: String, required: true } })
const router = useRouter()

const content = ref('')
const meta = ref(null)
const systemdUnit = ref(null)
const container = ref(null)
const error = ref('')
const logs = ref('')
const showLogs = ref(false)
const loading = ref(true)
const busy = ref(false)
let firstLoad = true

const unitName = computed(() => {
  if (!meta.value) return ''
  const ext = '.' + meta.value.filename.split('.').pop()
  const base = meta.value.filename.slice(0, -ext.length)
  switch (meta.value.type) {
    case 'container':
      return base + '.service'
    case 'volume':
      return base + '-volume.service'
    case 'network':
      return base + '-network.service'
    default:
      return meta.value.filename
  }
})

async function load() {
  error.value = ''
  if (firstLoad) loading.value = true
  try {
    const [data, list, units, containers] = await Promise.all([
      quadletsApi.get(props.filename),
      quadletsApi.list(),
      systemdApi.list(),
      containersApi.list(),
    ])
    content.value = data.content
    meta.value = list.find((f) => f.filename === props.filename) || null
    systemdUnit.value = units.find((u) => u.name === unitName.value) || null
    container.value = containers.find((c) => c.systemdUnit === unitName.value) || null
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
  busy.value = true
  try {
    await fn(props.filename)
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function remove() {
  if (!confirm(`Delete quadlet file "${props.filename}"? This stops the unit and removes the file.`)) return
  error.value = ''
  busy.value = true
  try {
    await quadletsApi.delete(props.filename)
    router.push('/quadlets')
  } catch (e) {
    error.value = e.message
    busy.value = false
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
    <div class="toolbar" style="margin-bottom: 0">
      <button :disabled="busy" @click="save">Save</button>
      <details class="dropdown">
        <summary role="button" class="secondary">More Options</summary>
        <ul>
          <li><a href="#" @click.prevent="!busy && action(quadletsApi.start)">Start</a></li>
          <li><a href="#" @click.prevent="!busy && action(quadletsApi.stop)">Stop</a></li>
          <li><a href="#" @click.prevent="!busy && action(quadletsApi.restart)">Restart</a></li>
          <li><a href="#" @click.prevent="viewLogs">{{ showLogs ? 'Hide Logs' : 'View Logs' }}</a></li>
          <li><a href="#" class="danger-link" @click.prevent="!busy && remove()">Delete</a></li>
        </ul>
      </details>
    </div>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>
  <p v-if="loading" aria-busy="true">Loading…</p>

  <template v-else>
    <div class="stack-columns">
      <div class="col">
        <label>
          Unit file
          <textarea v-model="content" rows="16"></textarea>
        </label>

        <pre v-if="showLogs" class="logs">{{ logs }}</pre>
      </div>

      <div class="col">
        <article v-if="meta?.stack">
          <h2>Stack</h2>
          <p>
            <RouterLink :to="`/stacks/${encodeURIComponent(meta.stack)}`">{{ meta.stack }}</RouterLink>
          </p>
        </article>

        <article>
          <h2>Systemd Unit</h2>
          <p v-if="!systemdUnit" class="muted">Not found.</p>
          <template v-else>
            <RouterLink :to="`/systemd/${encodeURIComponent(systemdUnit.name)}`">{{ systemdUnit.name }}</RouterLink>
            <div class="toolbar">
              <span :class="['badge', systemdUnit.active]">{{ systemdUnit.active }}</span>
              <span class="muted">{{ systemdUnit.sub }}</span>
            </div>
            <p><span class="muted">Description:</span> {{ systemdUnit.description || '—' }}</p>
          </template>
        </article>

        <article v-if="meta?.type === 'container'">
          <h2>Podman Container</h2>
          <p v-if="!container" class="muted">No running container.</p>
          <template v-else>
            <RouterLink :to="`/containers/${encodeURIComponent(container.id)}`">{{ container.names[0] }}</RouterLink>
            <div class="toolbar">
              <span :class="['badge', container.state]">{{ container.state }}</span>
              <span class="muted">{{ container.status }}</span>
            </div>
          </template>
        </article>
      </div>
    </div>
  </template>
</template>
