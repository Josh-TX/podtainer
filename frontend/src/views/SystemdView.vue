<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { systemdApi } from '../api'
import { unitBadgeClass, unitStatusLabel, statusTitle } from '../unitBadge'
import { showQuadlet, showFavorite, showAll, filterText } from '../systemdFilterState'

const units = ref([])
const error = ref('')
const loading = ref(true)

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
    <RouterLink to="/systemd/new" role="button">New Unit</RouterLink>
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
</template>

<style scoped>
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
