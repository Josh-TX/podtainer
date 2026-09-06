<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { stacksApi } from '../api'
import { unitBadgeClass, restartingLabel } from '../unitBadge'
import CodeEditor from '../components/CodeEditor.vue'

const props = defineProps({ isNew: { type: Boolean, default: false } })
const route = useRoute()
const router = useRouter()

const name = ref('')
const content = ref('')
const path = ref('')
const status = ref(null)
const error = ref('')
const busy = ref(false)
const loading = ref(false)
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

// /stacks/new and /stacks/:name both render this same component, so Vue
// Router reuses the instance instead of remounting when navigating between
// them (e.g. right after creating a stack) — reinitialize state and reload.
function resetForRoute() {
  name.value = props.isNew ? '' : route.params.name
  content.value = props.isNew ? 'services:\n  web:\n    image: docker.io/library/nginx:latest\n    ports:\n      - "8080:80"\n' : ''
  path.value = ''
  status.value = null
  error.value = ''
  loading.value = !props.isNew
  firstLoad = true
  load()
}

watch(() => [props.isNew, route.params.name], resetForRoute)

async function createStack() {
  error.value = ''
  busy.value = true
  try {
    await stacksApi.deploy(name.value, content.value, { isCreate: true })
    router.push(`/stacks/${name.value}`)
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

const showDeployModal = ref(false)
const deployForce = ref(false)
const deployPull = ref(false)

watch(deployPull, (v) => { if (v) deployForce.value = true })

function openDeployModal() {
  deployForce.value = false
  deployPull.value = false
  showDeployModal.value = true
}

async function confirmDeploy() {
  showDeployModal.value = false
  error.value = ''
  busy.value = true
  try {
    await stacksApi.deploy(name.value, content.value, { force: deployForce.value, pull: deployPull.value })
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

const showDeleteModal = ref(false)
const delStack = ref(true)
const delQuadlet = ref(true)
const delImages = ref(false)
const delVolumes = ref(false)

const deleteValidationError = computed(() => {
  if ((delImages.value || delVolumes.value) && !delQuadlet.value) {
    return 'Deleting images or volumes requires also deleting quadlet files.'
  }
  return ''
})

function openDeleteModal() {
  delStack.value = true
  delQuadlet.value = true
  delImages.value = false
  delVolumes.value = false
  showDeleteModal.value = true
}

async function confirmDelete() {
  showDeleteModal.value = false
  error.value = ''
  busy.value = true
  try {
    await stacksApi.delete(name.value, { stack: delStack.value, quadlet: delQuadlet.value, images: delImages.value, volumes: delVolumes.value })
    router.push('/stacks')
  } catch (e) {
    error.value = e.message
    busy.value = false
  }
}

onMounted(resetForRoute)
</script>

<template>
  <div class="page-header">
    <h1 style="margin-bottom: 0.5rem">
      {{ isNew ? 'New Stack' : name }}
      <span v-if="!isNew && status && !status.deployed" class="badge notdeployed">Not Deployed</span>
      <span v-else-if="!isNew && status" :class="['badge', healthClass(healthRatio(status.podmanContainers))]">{{ healthRatio(status.podmanContainers).success }}/{{ healthRatio(status.podmanContainers).total }}</span>
      <span v-if="!isNew && status && status.drift" class="drift-icon" data-tooltip="Deployed quadlet units differ from the compose file — redeploy to apply changes">
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="m18.84 12.25 1.72-1.71a5 5 0 0 0-.12-7.07 5.006 5.006 0 0 0-6.95 0l-1.72 1.71" />
          <path d="m5.17 11.75-1.71 1.71a5.004 5.004 0 0 0 .12 7.07 5.006 5.006 0 0 0 6.95 0l1.71-1.71" />
          <line x1="8" x2="8" y1="2" y2="5" />
          <line x1="2" x2="5" y1="8" y2="8" />
          <line x1="16" x2="16" y1="19" y2="22" />
          <line x1="19" x2="22" y1="16" y2="16" />
        </svg>
      </span>
    </h1>
    <div class="toolbar" style="margin-bottom: 0">
      <button :disabled="busy || !name" @click="isNew ? createStack() : openDeployModal()">Save &amp; Deploy</button>
      <button v-if="!isNew" class="danger" :disabled="busy" @click="openDeleteModal">Delete</button>
    </div>
  </div>

  <dialog :open="showDeployModal">
    <article>
      <header>
        <button aria-label="Close" rel="prev" @click="showDeployModal = false"></button>
        <strong>Save &amp; Deploy</strong>
      </header>
      <label>
        <input type="checkbox" v-model="deployForce" :disabled="deployPull" />
        Re-deploy unchanged systemd services
      </label>
      <label>
        <input type="checkbox" v-model="deployPull" />
        Re-pull all images
      </label>
      <footer>
        <button class="secondary" @click="showDeployModal = false">Cancel</button>
        <button @click="confirmDeploy">Deploy</button>
      </footer>
    </article>
  </dialog>

  <dialog :open="showDeleteModal">
    <article>
      <header>
        <button aria-label="Close" rel="prev" @click="showDeleteModal = false"></button>
        <strong>Delete Stack</strong>
      </header>
      <label>
        <input type="checkbox" v-model="delStack" />
        Delete Stack (compose yaml)
      </label>
      <label>
        <input type="checkbox" v-model="delQuadlet" />
        Delete Quadlet Files
      </label>
      <label>
        <input type="checkbox" v-model="delImages" />
        Delete Images (if unused)
      </label>
      <label>
        <input type="checkbox" v-model="delVolumes" />
        Delete Volumes (if unused)
      </label>
      <p v-if="deleteValidationError" class="error-banner">{{ deleteValidationError }}</p>
      <footer>
        <button class="secondary" @click="showDeleteModal = false">Cancel</button>
        <button class="danger" :disabled="!!deleteValidationError" @click="confirmDelete">Delete</button>
      </footer>
    </article>
  </dialog>

  <div v-if="error" class="error-banner">{{ error }}</div>
  <p v-if="loading" aria-busy="true">Loading…</p>

  <template v-else>
    <label v-if="isNew">
      Stack name
      <input v-model="name" />
    </label>

    <p v-if="!isNew" class="muted" style="margin-bottom: 0.25rem">{{ path }}</p>

    <div class="stack-columns">
      <div class="col">
        <CodeEditor v-model="content" language="yaml" max-height="26rem" />
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
