<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { stacksApi } from '../api'
import { unitBadgeClass, restartingLabel } from '../unitBadge'

const props = defineProps({ isNew: { type: Boolean, default: false } })
const route = useRoute()
const router = useRouter()

const name = ref(props.isNew ? '' : route.params.name)
const content = ref(props.isNew ? 'services:\n  web:\n    image: docker.io/library/nginx:latest\n    ports:\n      - "8080:80"\n' : '')
const path = ref('')
const status = ref(null)
const error = ref('')
const busy = ref(false)
const loading = ref(!props.isNew)
let firstLoad = true

function healthRatio(containers) {
  const total = containers.length
  const success = containers.filter((c) => c.state === 'running' && (c.health === 'healthy' || c.health === 'none')).length
  return { success, total }
}

function healthClass({ success, total }) {
  if (total === 0 || success === 0) return 'unhealthy'
  if (success === total) return 'healthy'
  return 'starting'
}

function hasHealthChecks(containers) {
  return containers.some((c) => c.health === 'healthy' || c.health === 'unhealthy')
}

async function load() {
  if (props.isNew) return
  error.value = ''
  if (firstLoad) loading.value = true
  try {
    const data = await stacksApi.get(name.value)
    content.value = data.content
    path.value = data.path
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

onMounted(load)
</script>

<template>
  <div class="page-header">
    <h1 style="margin-bottom: 0.5rem">
      {{ isNew ? 'New Stack' : name }}
      <span v-if="!isNew && status && !status.deployed" class="badge notdeployed">Not Deployed</span>
      <span v-else-if="!isNew && status && status.drift" class="badge drift">Needs Redeploy</span>
      <span v-else-if="!isNew && status" :class="['badge', healthClass(healthRatio(status.podmanContainers))]">{{ healthRatio(status.podmanContainers).success }}/{{ healthRatio(status.podmanContainers).total }}</span>
    </h1>
    <div class="toolbar" style="margin-bottom: 0">
      <button :disabled="busy || !name" @click="saveAndDeploy(false)">Save &amp; Deploy</button>
      <details v-if="!isNew" class="dropdown">
        <summary role="button" class="secondary">More Options</summary>
        <ul>
          <li><a href="#" @click.prevent="!busy && saveAndDeploy(true)">Force Redeploy</a></li>
          <li><a href="#" @click.prevent="!busy && pullAndRestart()">Pull &amp; Restart</a></li>
          <li><a href="#" class="danger-link" @click.prevent="!busy && remove()">Delete</a></li>
        </ul>
      </details>
    </div>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>
  <p v-if="loading" aria-busy="true">Loading…</p>

  <template v-else>
    <label v-if="isNew">
      Stack name
      <input v-model="name" placeholder="mystack" />
    </label>

    <p v-if="!isNew" class="muted" style="margin-bottom: 0.25rem">{{ path }}</p>

    <div class="stack-columns">
      <div class="col">
        <textarea v-model="content" rows="16"></textarea>
      </div>

      <div class="col" v-if="!isNew && status">
        <h2>Quadlet Units</h2>
        <table class="rows">
          <thead>
            <tr>
              <th>Filename</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in status.quadletUnits" :key="u.filename">
              <td><RouterLink class="row-link" :to="`/quadlets/${encodeURIComponent(u.filename)}`">{{ u.filename }}</RouterLink></td>
              <td><span :class="unitBadgeClass(u)" :title="restartingLabel(u)">{{ u.active }}</span></td>
            </tr>
          </tbody>
        </table>

        <h2>Systemd Units</h2>
        <table class="rows">
          <thead>
            <tr>
              <th>Unit</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in status.systemdUnits" :key="u.unit">
              <td><RouterLink class="row-link" :to="`/systemd/${encodeURIComponent(u.unit)}`">{{ u.unit }}</RouterLink></td>
              <td><span :class="unitBadgeClass(u)" :title="restartingLabel(u)">{{ u.active }}</span></td>
            </tr>
          </tbody>
        </table>

        <h2>Podman Containers</h2>
        <table class="rows">
          <thead>
            <tr>
              <th>Name</th>
              <th>State</th>
              <th v-if="hasHealthChecks(status.podmanContainers)">Health</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in status.podmanContainers" :key="c.name">
              <td><RouterLink class="row-link" :to="`/containers/${encodeURIComponent(c.name)}`">{{ c.name }}</RouterLink></td>
              <td><span :class="['badge', c.state]">{{ c.state }}</span></td>
              <td v-if="hasHealthChecks(status.podmanContainers)"><span :class="['badge', c.health]">{{ c.health }}</span></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </template>
</template>
