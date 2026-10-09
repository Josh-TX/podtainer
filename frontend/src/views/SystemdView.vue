<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { systemdApi } from '../api'
import { unitBadgeClass, unitStatusLabel, statusTitle } from '../unitBadge'
import { showQuadlet, showFavorite, showAll, filterText } from '../systemdFilterState'

const units = ref([])
const error = ref('')
const loading = ref(true)

const envDialog = ref(null)
const envInfo = ref(null)
const envContent = ref('')
const envError = ref('')
const envSaved = ref(false)
const envBusy = ref(false)

const USER_TINT = 'background: rgba(0, 190, 170, 0.15)'

// Shell PATH entries, flagged when they also appear in the system PATH (unflagged ones are user-only).
const shellParts = computed(() => {
  const sys = new Set((envInfo.value?.system || '').split(':'))
  return (envInfo.value?.shell || '').split(':').map((entry) => ({ entry, sys: sys.has(entry) }))
})

// PATH line prepending the user-only shell entries, with the home dir written as $HOME.
const recommended = computed(() => {
  const home = envInfo.value?.home
  const own = shellParts.value.filter((p) => !p.sys && p.entry).map((p) => p.entry)
  if (!own.length) return ''
  const withHome = own.map((e) => (home && (e === home || e.startsWith(home + '/')) ? '$HOME' + e.slice(home.length) : e))
  return 'PATH=' + [...withHome, '$PATH'].join(':')
})

async function openEnv() {
  envError.value = ''
  envSaved.value = false
  envInfo.value = null
  envDialog.value.showModal()
  try {
    envInfo.value = await systemdApi.getEnv()
    envContent.value = envInfo.value.content
  } catch (e) {
    envError.value = e.message
  }
}

async function saveEnv() {
  envError.value = ''
  envBusy.value = true
  try {
    await systemdApi.setEnv(envContent.value)
    envSaved.value = true
  } catch (e) {
    envError.value = e.message
  } finally {
    envBusy.value = false
  }
}

// Only close when the press and release both land on the backdrop, so a
// text selection dragged out of the textarea doesn't dismiss the modal.
let envMouseDownOnBackdrop = false
function onEnvDialogMouseDown(e) {
  envMouseDownOnBackdrop = e.target === envDialog.value
}
function onEnvDialogClick(e) {
  if (envMouseDownOnBackdrop && e.target === envDialog.value) envDialog.value.close()
  envMouseDownOnBackdrop = false
}

const filteredUnits = computed(() => {
  const q = filterText.value.trim().toLowerCase()
  if (!q) return units.value
  return units.value.filter(
    (u) => u.name.toLowerCase().includes(q) || (u.description || '').toLowerCase().includes(q)
  )
})

async function load() {
  error.value = ''
  loading.value = true
  try {
    units.value = await systemdApi.list({
      all: showAll.value,
      quadlet: showQuadlet.value,
      favorite: showFavorite.value,
    })
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function toggleFavorite(u) {
  const favorite = !u.isFavorite
  try {
    await systemdApi.setFavorite(u.name, favorite)
    u.isFavorite = favorite
  } catch (e) {
    error.value = e.message
  }
}

watch([showQuadlet, showFavorite, showAll], load)
onMounted(load)
</script>

<template>
  <div class="page-header">
    <h1>Systemd</h1>
    <div class="header-actions">
      <button class="secondary" @click="openEnv">Environment</button>
      <RouterLink to="/systemd/new" role="button">New Unit</RouterLink>
    </div>
  </div>

  <div class="toolbar systemd-filters">
    <input type="text" v-model="filterText" placeholder="Filter by name or description" />
    <label><input type="checkbox" v-model="showQuadlet" :disabled="showAll" /> Quadlet</label>
    <label><input type="checkbox" v-model="showFavorite" :disabled="showAll" /> Favorites</label>
    <label><input type="checkbox" v-model="showAll" /> All</label>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>

  <p v-if="loading" aria-busy="true">Loading…</p>
  <template v-else>
    <table class="rows">
      <thead>
        <tr>
          <th>Unit</th>
          <th>Active</th>
          <th>Sub</th>
          <th>File State</th>
          <th>Description</th>
          <th>Favorite</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="u in filteredUnits" :key="u.name">
          <td><RouterLink class="row-link" :to="`/systemd/${encodeURIComponent(u.name)}`">{{ u.name }}</RouterLink></td>
          <td><span :class="unitBadgeClass(u)" :title="statusTitle(u)">{{ unitStatusLabel(u) }}</span></td>
          <td class="muted">{{ u.sub }}</td>
          <td class="muted">{{ u.unitFileState }}</td>
          <td class="muted">{{ u.description }}</td>
          <td style="padding-top: 5px; padding-bottom: 0px">
            <button
              class="star-toggle"
              :class="{ active: u.isFavorite }"
              :aria-label="u.isFavorite ? 'Unfavorite' : 'Favorite'"
              :title="u.isFavorite ? 'Unfavorite' : 'Favorite'"
              @click="toggleFavorite(u)"
            >
              <svg viewBox="0 0 24 24" width="30" height="30" :fill="u.isFavorite ? 'currentColor' : 'none'" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M11.525 2.295a.53.53 0 0 1 .95 0l2.31 4.679a2.123 2.123 0 0 0 1.595 1.16l5.166.756a.53.53 0 0 1 .294.904l-3.736 3.638a2.123 2.123 0 0 0-.611 1.878l.882 5.14a.53.53 0 0 1-.771.56l-4.618-2.428a2.122 2.122 0 0 0-1.973 0L6.396 21.01a.53.53 0 0 1-.77-.56l.881-5.139a2.122 2.122 0 0 0-.611-1.879L2.16 9.795a.53.53 0 0 1 .294-.906l5.165-.755a2.122 2.122 0 0 0 1.597-1.16z" />
              </svg>
            </button>
          </td>
        </tr>
      </tbody>
    </table>
    <p class="muted" v-if="filterText.trim()">Showing {{ filteredUnits.length }}/{{ units.length }} units</p>
    <p class="muted" v-else>Showing {{ units.length }} units</p>
  </template>

  <dialog ref="envDialog" @mousedown="onEnvDialogMouseDown" @click="onEnvDialogClick">
    <article style="width: min(95vw, 100rem); max-width: none">
      <h3>Environment</h3>
      <div v-if="envError" class="error-banner">{{ envError }}</div>
      <p v-if="!envInfo && !envError" aria-busy="true">Loading…</p>
      <template v-if="envInfo">
        <p class="muted" style="margin-bottom: 0.25rem">{{ envInfo.path }}</p>
        <textarea
          v-model="envContent"
          class="env-text"
          rows="10"
          spellcheck="false"
          placeholder="e.g. PATH=$HOME/.local/bin:$PATH"
          @input="envSaved = false"
        ></textarea>
        <p class="env-label">System PATH</p>
        <pre class="env-ro">{{ envInfo.systemError ? "Couldn't detect: " + envInfo.systemError : envInfo.system }}</pre>
        <p class="env-label">User shell PATH</p>
        <pre class="env-ro"><template v-if="envInfo.shellError">Couldn't detect: {{ envInfo.shellError }}</template><template v-else><template v-for="(p, i) in shellParts" :key="i"><template v-if="i">:</template><span :style="p.sys ? '' : USER_TINT">{{ p.entry }}</span></template></template></pre>
        <template v-if="recommended">
          <p class="env-label">Recommended config</p>
          <pre class="env-ro">{{ recommended }}</pre>
        </template>
      </template>
      <p v-if="envSaved" class="muted">Saved and reloaded. Restart running services to apply.</p>
      <footer>
        <button class="secondary" @click="envDialog.close()">Close</button>
        <button :disabled="!envInfo || envBusy" :aria-busy="envBusy" @click="saveEnv">Save &amp; Reload</button>
      </footer>
    </article>
  </dialog>
</template>

<style scoped>
.header-actions {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}
.env-text,
.env-ro {
  font-family: monospace;
  font-size: 0.85rem;
}
.env-text {
  width: 100%;
  white-space: pre;
}
.env-label {
  margin-bottom: 0.25rem;
}
.env-ro {
  white-space: pre-wrap;
  word-break: break-all;
  padding: 0.5rem;
  margin-bottom: 0.75rem;
}
.systemd-filters {
  align-items: center;
  gap: 1rem;
}
.systemd-filters label {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  margin-bottom: 0;
  white-space: nowrap;
}
.systemd-filters input[type='text'] {
  margin-bottom: 0;
  max-width: 20rem;
}
.star-toggle {
  display: inline-flex;
  background: none;
  border: none;
  cursor: pointer;
  padding: 0;
  color: var(--pico-muted-color);
}
.star-toggle.active {
  color: light-dark(#9a6700, #e3b341);
}
</style>
