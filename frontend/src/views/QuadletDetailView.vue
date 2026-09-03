<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { quadletsApi, systemdApi, containersApi } from '../api'
import { unitBadgeClass, restartingLabel } from '../unitBadge'
import CodeEditor from '../components/CodeEditor.vue'

const props = defineProps({ filename: { type: String, required: true } })
const router = useRouter()

const content = ref('')
const path = ref('')
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
    path.value = data.path
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
    <h1 style="margin-bottom: 0.5rem">{{ filename }}</h1>
    <div class="toolbar" style="margin-bottom: 0">
      <button :disabled="busy" @click="save">Save</button>
      <details class="dropdown">
        <summary role="button" class="secondary">More Options</summary>
        <ul>
          <li><a href="#" @click.prevent="!busy && action(quadletsApi.start)">Start</a></li>
          <li><a href="#" @click.prevent="!busy && action(quadletsApi.stop)">Stop</a></li>
          <li><a href="#" @click.prevent="!busy && action(quadletsApi.restart)">Restart</a></li>
          <li><a href="#" class="danger-link" @click.prevent="!busy && remove()">Delete</a></li>
        </ul>
      </details>
    </div>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>
  <p v-if="loading" aria-busy="true">Loading…</p>

  <template v-else>
    <p class="muted" style="margin-bottom: 0.25rem">{{ path }}</p>

    <div class="stack-columns">
      <div class="col">
        <CodeEditor v-model="content" language="unit" max-height="26rem" />
      </div>

      <div class="col">
        <article>
          <p v-if="meta?.stack" style="margin-bottom: 0.25rem">
            Stack: <RouterLink :to="`/stacks/${encodeURIComponent(meta.stack)}`">{{ meta.stack }}</RouterLink>
          </p>

          <p style="margin-bottom: 0.25rem">
            Systemd unit:
            <template v-if="systemdUnit">
              <span :class="unitBadgeClass(systemdUnit)" :title="restartingLabel(systemdUnit)">{{ systemdUnit.active }}</span>
              <RouterLink :to="`/systemd/${encodeURIComponent(systemdUnit.name)}`">{{ systemdUnit.name }}</RouterLink>
            </template>
            <span v-else class="muted">Not found.</span>
          </p>

          <p v-if="meta?.type === 'container'" style="margin-bottom: 0">
            Podman container:
            <template v-if="container">
              <span :class="['badge', container.state]">{{ container.state }}</span>
              <RouterLink :to="`/containers/${encodeURIComponent(container.id)}`">{{ container.names[0] }}</RouterLink>
            </template>
            <span v-else class="muted">No running container.</span>
          </p>
        </article>

        <button class="secondary" @click="viewLogs">{{ showLogs ? 'Hide Logs' : 'View Logs' }}</button>
        <CodeEditor v-if="showLogs" :model-value="logs" readonly autoscroll />
      </div>
    </div>
  </template>
</template>
