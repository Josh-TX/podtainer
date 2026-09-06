<script setup>
import { ref, computed, onMounted } from 'vue'
import { imagesApi } from '../api'
import { timeAgo, fullDate, humanSize } from '../utils/format'

const images = ref([])
const error = ref('')
const loading = ref(true)

const unusedCount = computed(() => images.value.filter(img => img.containers.length === 0).length)
const danglingCount = computed(() => images.value.filter(img => img.repository === '<none>' && img.containers.length === 0).length)

async function load() {
  error.value = ''
  loading.value = true
  try {
    images.value = await imagesApi.list()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function deleteImage(img) {
  if (!confirm(`Delete image "${img.repository}:${img.tag}"? This cannot be undone.`)) return
  error.value = ''
  try {
    await imagesApi.delete(img.id)
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function prune(all) {
  const count = all ? unusedCount.value : danglingCount.value
  if (count === 0) return
  const label = all ? 'unused' : 'dangling'
  if (!confirm(`Remove ${count} ${label} image${count === 1 ? '' : 's'}? This cannot be undone.`)) return
  error.value = ''
  try {
    await imagesApi.prune(all)
    await load()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div class="page-header">
    <h1>Images</h1>
    <div class="toolbar" style="margin-bottom: 0">
      <details class="dropdown">
        <summary role="button" class="secondary">Actions</summary>
        <ul>
          <li>
            <a href="#" :class="{ disabled: unusedCount === 0 }" :aria-disabled="unusedCount === 0" @click.prevent="prune(true)">
              Prune unused ({{ unusedCount }})
            </a>
          </li>
          <li>
            <a href="#" :class="{ disabled: danglingCount === 0 }" :aria-disabled="danglingCount === 0" @click.prevent="prune(false)">
              Prune dangling ({{ danglingCount }})
            </a>
          </li>
        </ul>
      </details>
    </div>
  </div>
  <p class="muted">Every podman image on the host, regardless of origin.</p>

  <div v-if="error" class="error-banner">{{ error }}</div>

  <p v-if="loading" aria-busy="true">Loading…</p>
  <template v-else>
    <table class="rows">
      <thead>
        <tr>
          <th>Repository</th>
          <th>Tag</th>
          <th>Image ID</th>
          <th>Created</th>
          <th>Size</th>
          <th>Containers</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="img in images" :key="img.id + ':' + img.repository + ':' + img.tag">
          <td>{{ img.repository }}</td>
          <td>{{ img.tag }}</td>
          <td class="muted no-wrap">{{ img.id.slice(0, 12) }}</td>
          <td class="muted no-wrap" :title="fullDate(img.createdAt)">{{ timeAgo(img.createdAt) }}</td>
          <td class="muted no-wrap">{{ humanSize(img.size) }}</td>
          <td class="muted">
            <RouterLink v-if="img.containers.length === 1" :to="`/containers/${encodeURIComponent(img.containers[0].id)}`">
              {{ img.containers[0].names?.[0] || img.containers[0].id.slice(0, 12) }}
            </RouterLink>
            <span v-else-if="img.containers.length > 1">{{ img.containers.length }} containers</span>
            <span v-else>&mdash;</span>
          </td>
          <td class="no-wrap">
            <button
              class="danger"
              :disabled="img.containers.length > 0"
              :title="img.containers.length > 0 ? 'In use by a container' : ''"
              @click="deleteImage(img)"
            >
              Delete
            </button>
          </td>
        </tr>
      </tbody>
    </table>
    <p v-if="!images.length" class="muted">No images found.</p>
  </template>
</template>

<style scoped>
table.rows {
  width: 100%;
}
table.rows tr {
  cursor: auto;
}
table.rows tbody tr:hover td {
  background-color: transparent;
}
.no-wrap {
  white-space: nowrap;
}
.dropdown a.disabled {
  color: var(--pico-muted-color);
  pointer-events: none;
}
</style>
