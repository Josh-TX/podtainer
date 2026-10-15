<script setup>
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { filesApi } from '../api'
import CodeEditor from '../components/CodeEditor.vue'
import { humanSize, timeAgo, fullDate } from '../utils/format'

const currentPath = ref('')
const entries = ref([])
const dirError = ref('')
const dirLoading = ref(false)
const dirSizes = ref({})
const sizeLoading = ref({})
const sortKey = ref('name')
const sortAsc = ref(true)
const dragging = ref(false)
const showHidden = ref(true)

const selectedName = ref('')
const fileContent = ref('')
const originalContent = ref('')
const notEditable = ref(false)
const fileError = ref('')
const fileBusy = ref(false)

const fileInput = ref(null)
const fileDialog = ref(null)
const permDialog = ref(null)
const permEntry = ref(null)
const permBits = ref(0)
const permRecursive = ref(false)
const permError = ref('')

const isUnchanged = computed(() => notEditable.value || fileContent.value === originalContent.value)

const breadcrumbs = computed(() => {
  const crumbs = [{ name: '/', path: '/' }]
  let acc = ''
  for (const part of currentPath.value.split('/').filter(Boolean)) {
    acc += '/' + part
    crumbs.push({ name: part, path: acc })
  }
  return crumbs
})

const sortedEntries = computed(() => {
  const dir = sortAsc.value ? 1 : -1
  const key = sortKey.value
  const val = (e) => {
    if (key === 'size') return e.isDir ? (dirSizes.value[e.name] ?? -1) : e.size
    if (key === 'modTime') return new Date(e.modTime).getTime()
    if (key === 'mode') return e.perm
    return e.name.toLowerCase()
  }
  return entries.value.filter((e) => showHidden.value || !e.name.startsWith('.')).sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
    const x = val(a)
    const y = val(b)
    return x < y ? -dir : x > y ? dir : 0
  })
})

function setSort(key) {
  if (sortKey.value === key) sortAsc.value = !sortAsc.value
  else {
    sortKey.value = key
    sortAsc.value = true
  }
}

function arrow(key) {
  return sortKey.value === key ? (sortAsc.value ? ' ▲' : ' ▼') : ''
}

function joinPath(dir, name) {
  return dir === '/' ? `/${name}` : `${dir}/${name}`
}

function fullPath(name) {
  return joinPath(currentPath.value, name)
}

async function loadDir(path) {
  closeFile()
  dirError.value = ''
  dirLoading.value = true
  try {
    const data = await filesApi.list(path)
    currentPath.value = data.path
    entries.value = data.entries
    dirSizes.value = {}
    sizeLoading.value = {}
  } catch (e) {
    dirError.value = e.message
  } finally {
    dirLoading.value = false
  }
}

async function reload() {
  await loadDir(currentPath.value)
}

async function computeSize(entry) {
  sizeLoading.value = { ...sizeLoading.value, [entry.name]: true }
  try {
    const size = await filesApi.dirSize(fullPath(entry.name))
    dirSizes.value = { ...dirSizes.value, [entry.name]: size }
  } catch (e) {
    dirError.value = e.message
  } finally {
    sizeLoading.value = { ...sizeLoading.value, [entry.name]: false }
  }
}

function openEntry(entry) {
  if (entry.isDir) loadDir(fullPath(entry.name))
  else openFile(entry)
}

async function openFile(entry) {
  fileError.value = ''
  selectedName.value = entry.name
  notEditable.value = false
  fileContent.value = ''
  originalContent.value = ''
  fileDialog.value.showModal()
  try {
    const data = await filesApi.read(fullPath(entry.name))
    if (!data.editable) {
      notEditable.value = true
      return
    }
    fileContent.value = data.content
    originalContent.value = data.content
  } catch (e) {
    fileError.value = e.message
  }
}

function closeFile() {
  fileDialog.value?.close()
  selectedName.value = ''
  fileContent.value = ''
  originalContent.value = ''
  notEditable.value = false
  fileError.value = ''
}

function onDialogClick(e, close) {
  if (e.target === e.currentTarget) close()
}

async function saveFile() {
  fileError.value = ''
  fileBusy.value = true
  try {
    await filesApi.writeFile(fullPath(selectedName.value), fileContent.value)
    originalContent.value = fileContent.value
    await refreshListing()
  } catch (e) {
    fileError.value = e.message
  } finally {
    fileBusy.value = false
  }
}

// refreshListing reloads the table without closing the open editor.
async function refreshListing() {
  const data = await filesApi.list(currentPath.value)
  entries.value = data.entries
}

function triggerDownload(path) {
  const a = document.createElement('a')
  a.href = filesApi.downloadUrl(path)
  a.download = ''
  document.body.appendChild(a)
  a.click()
  a.remove()
}

async function saveAndDownload() {
  const path = fullPath(selectedName.value)
  if (!isUnchanged.value) {
    fileBusy.value = true
    try {
      await filesApi.writeFile(path, fileContent.value)
      originalContent.value = fileContent.value
    } catch (e) {
      fileError.value = e.message
      return
    } finally {
      fileBusy.value = false
    }
  }
  triggerDownload(path)
}

async function run(fn) {
  dirError.value = ''
  try {
    await fn()
    await reload()
  } catch (e) {
    dirError.value = e.message
  }
}

async function newFile() {
  const name = prompt('New file name:')
  if (name) await run(() => filesApi.createFile(fullPath(name)))
}

async function newFolder() {
  const name = prompt('New folder name:')
  if (name) await run(() => filesApi.mkdir(fullPath(name)))
}

async function renameEntry(entry) {
  const name = prompt('Rename to:', entry.name)
  if (!name || name === entry.name) return
  await run(() => filesApi.move(fullPath(entry.name), fullPath(name), false))
}

async function moveEntry(entry, isCopy) {
  const src = fullPath(entry.name)
  const dest = prompt(isCopy ? 'Copy to (full path):' : 'Move to (full path):', src)
  if (!dest || dest === src) return
  await run(() => filesApi.move(src, dest, isCopy))
}

async function deleteEntry(entry) {
  const path = fullPath(entry.name)
  if (entry.isDir) {
    let nonEmpty = true
    try {
      nonEmpty = (await filesApi.list(path)).entries.length > 0
    } catch {
      // unreadable: treat as non-empty
    }
    if (nonEmpty) {
      const typed = prompt(`Folder "${entry.name}" is not empty. Type its name to delete it and everything in it:`)
      if (typed !== entry.name) return
    } else if (!confirm(`Delete empty folder "${entry.name}"?`)) return
  } else if (!confirm(`Delete "${entry.name}"?`)) return
  await run(() => filesApi.deleteEntry(path))
}

async function deleteOpenFile() {
  const entry = { name: selectedName.value, isDir: false }
  closeFile()
  await deleteEntry(entry)
}

// --- permissions ---

const permOctal = computed({
  get: () => permBits.value.toString(8).padStart(3, '0'),
  set: (v) => {
    if (/^[0-7]{1,3}$/.test(v)) permBits.value = parseInt(v, 8)
  },
})

function permHas(bit) {
  return (permBits.value & bit) !== 0
}

function togglePerm(bit, on) {
  permBits.value = on ? permBits.value | bit : permBits.value & ~bit
}

function openPerms(entry) {
  permEntry.value = entry
  permBits.value = entry.perm
  permRecursive.value = false
  permError.value = ''
  permDialog.value.showModal()
}

function closePerms() {
  permDialog.value.close()
  permEntry.value = null
}

async function savePerms() {
  permError.value = ''
  try {
    await filesApi.chmod(fullPath(permEntry.value.name), permBits.value, permRecursive.value)
    closePerms()
    await reload()
  } catch (e) {
    permError.value = e.message
  }
}

const permRows = [
  { label: 'Owner', shift: 6 },
  { label: 'Group', shift: 3 },
  { label: 'Other', shift: 0 },
]
const permCols = [
  { label: 'Read', bit: 4 },
  { label: 'Write', bit: 2 },
  { label: 'Execute', bit: 1 },
]

// --- context menu ---

const menu = ref({ open: false, x: 0, y: 0, entry: null })
const menuEl = ref(null)

async function openMenu(e, entry) {
  menu.value = { open: true, x: e.clientX, y: e.clientY, entry }
  await nextTick()
  // Clamp to the viewport so a click near the bottom/right edge still shows
  // the whole menu: flip above/left of the cursor, never off-screen.
  const rect = menuEl.value.getBoundingClientRect()
  const pad = 4
  let x = e.clientX
  let y = e.clientY
  if (y + rect.height + pad > window.innerHeight) y = y - rect.height
  if (x + rect.width + pad > window.innerWidth) x = x - rect.width
  menu.value.x = Math.max(pad, x)
  menu.value.y = Math.max(pad, y)
}

function closeMenu() {
  menu.value.open = false
}

function menuAction(fn) {
  const entry = menu.value.entry
  closeMenu()
  fn(entry)
}

function onKeydown(e) {
  if (e.key === 'Escape') closeMenu()
}

onMounted(() => {
  window.addEventListener('click', closeMenu)
  window.addEventListener('resize', closeMenu)
  window.addEventListener('scroll', closeMenu, true)
  window.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => {
  window.removeEventListener('click', closeMenu)
  window.removeEventListener('resize', closeMenu)
  window.removeEventListener('scroll', closeMenu, true)
  window.removeEventListener('keydown', onKeydown)
})

// --- upload ---

function triggerUpload() {
  fileInput.value.click()
}

async function uploadFiles(fileList) {
  const list = Array.from(fileList)
  if (!list.length) return
  const existing = new Set(entries.value.map((e) => e.name))
  const conflicts = list.filter((f) => existing.has(f.name)).map((f) => f.name)
  if (conflicts.length && !confirm(`Overwrite existing: ${conflicts.join(', ')}?`)) {
    list.splice(0, list.length, ...list.filter((f) => !existing.has(f.name)))
  }
  dirError.value = ''
  const errors = []
  for (const f of list) {
    try {
      await filesApi.upload(fullPath(f.name), f, existing.has(f.name))
    } catch (e) {
      errors.push(`${f.name}: ${e.message}`)
    }
  }
  if (errors.length) dirError.value = errors.join('; ')
  try {
    await refreshListing()
  } catch (e) {
    dirError.value = e.message
  }
}

async function onUploadChange(e) {
  const files = e.target.files
  await uploadFiles(files)
  e.target.value = ''
}

// Only react to drags carrying real files, not dragged text/links/UI.
function hasFiles(e) {
  return Array.from(e.dataTransfer?.types || []).includes('Files')
}

function onDragOver(e) {
  if (!hasFiles(e)) return
  e.preventDefault()
  dragging.value = true
}

function onDragLeave(e) {
  if (!e.currentTarget.contains(e.relatedTarget)) dragging.value = false
}

function onDrop(e) {
  dragging.value = false
  if (!hasFiles(e)) return
  e.preventDefault()
  uploadFiles(e.dataTransfer.files)
}

onMounted(() => loadDir(''))
</script>

<template>
  <div class="files-view">
  <div class="page-header">
    <h1>Files</h1>
  </div>

  <div class="toolbar">
    <nav aria-label="breadcrumb">
      <ul>
        <li v-for="(crumb, i) in breadcrumbs" :key="crumb.path">
          <a v-if="i < breadcrumbs.length - 1" href="#" @click.prevent="loadDir(crumb.path)">{{ crumb.name }}</a>
          <strong v-else>{{ crumb.name }}</strong>
        </li>
      </ul>
    </nav>
  </div>

  <div class="toolbar">
    <button class="secondary" @click="newFile">New File</button>
    <button class="secondary" @click="newFolder">New Folder</button>
    <button class="secondary" @click="triggerUpload">Upload</button>
    <input ref="fileInput" type="file" multiple style="display: none" @change="onUploadChange" />
    <label style="margin: 0 0 0 0.5rem">
      <input v-model="showHidden" type="checkbox" />
      Show hidden
    </label>
  </div>

  <div v-if="dirError" class="error-banner">{{ dirError }}</div>
  <div
    :class="['dropzone', { dragging }]"
    @dragover="onDragOver"
    @dragleave="onDragLeave"
    @drop="onDrop"
  >
    <p v-if="dirLoading" aria-busy="true">Loading…</p>
    <template v-else>
      <table class="rows">
        <thead>
          <tr>
            <th class="sortable" @click="setSort('name')">Name{{ arrow('name') }}</th>
            <th class="sortable" @click="setSort('size')">Size{{ arrow('size') }}</th>
            <th class="sortable" @click="setSort('modTime')">Modified{{ arrow('modTime') }}</th>
            <th class="sortable" @click="setSort('mode')">Mode{{ arrow('mode') }}</th>
            <th>Owner</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="currentPath !== '/'">
            <td colspan="5">
              <a href="#" class="row-link" @click.prevent="loadDir(currentPath.replace(/\/[^/]*$/, '') || '/')">📁 ..</a>
            </td>
          </tr>
          <tr v-for="entry in sortedEntries" :key="entry.name" @contextmenu.prevent="openMenu($event, entry)">
            <td>
              <a href="#" class="row-link" @click.prevent="openEntry(entry)">{{ entry.isDir ? '📁 ' : '📄 ' }}{{ entry.name }}<span v-if="entry.isSymlink" class="muted"> → {{ entry.linkTarget }}</span></a>
            </td>
            <td class="muted" style="position: relative; z-index: 1">
              <template v-if="entry.isDir">
                <span v-if="sizeLoading[entry.name]" aria-busy="true"></span>
                <template v-else-if="dirSizes[entry.name] !== undefined">{{ humanSize(dirSizes[entry.name]) }}</template>
                <a v-else href="#" @click.prevent="computeSize(entry)">compute?</a>
              </template>
              <template v-else>{{ humanSize(entry.size) }}</template>
            </td>
            <td class="muted" :title="fullDate(entry.modTime)">{{ timeAgo(entry.modTime) }}</td>
            <td class="muted"><code>{{ entry.mode }}</code></td>
            <td class="muted">{{ entry.owner }}:{{ entry.group }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="!sortedEntries.length" class="muted">Empty directory.</p>
    </template>
  </div>

  <ul
    v-if="menu.open"
    ref="menuEl"
    class="ctx-menu"
    :style="{ left: menu.x + 'px', top: menu.y + 'px' }"
    @click.stop
    @contextmenu.prevent
  >
    <li><a href="#" @click.prevent="menuAction(openEntry)">Open</a></li>
    <li v-if="!menu.entry.isDir"><a href="#" @click.prevent="menuAction((e) => triggerDownload(fullPath(e.name)))">Download</a></li>
    <li><a href="#" @click.prevent="menuAction(renameEntry)">Rename</a></li>
    <li><a href="#" @click.prevent="menuAction((e) => moveEntry(e, false))">Move</a></li>
    <li><a href="#" @click.prevent="menuAction((e) => moveEntry(e, true))">Copy</a></li>
    <li><a href="#" @click.prevent="menuAction(openPerms)">Permissions</a></li>
    <li><a href="#" class="danger-link" @click.prevent="menuAction(deleteEntry)">Delete</a></li>
  </ul>

  <dialog ref="fileDialog" @click="onDialogClick($event, closeFile)">
    <article style="width: min(90vw, 60rem); max-width: min(90vw, 60rem)">
      <div class="page-header">
        <h4>{{ selectedName }}</h4>
        <div class="toolbar" style="margin-bottom: 0">
          <button v-if="!notEditable" :disabled="fileBusy || isUnchanged" @click="saveFile">Save</button>
          <details class="dropdown">
            <summary role="button" class="secondary">Actions</summary>
            <ul>
              <li><a href="#" @click.prevent="!fileBusy && saveAndDownload()">{{ isUnchanged ? 'Download' : 'Save & Download' }}</a></li>
              <li><a href="#" class="danger-link" @click.prevent="deleteOpenFile">Delete</a></li>
            </ul>
          </details>
          <button class="secondary" @click="closeFile">Close</button>
        </div>
      </div>
      <div v-if="fileError" class="error-banner">{{ fileError }}</div>
      <div v-if="notEditable">
        <p class="muted">Can't edit this file here (binary or larger than 1MB).</p>
        <button @click="triggerDownload(fullPath(selectedName))">Download</button>
      </div>
      <CodeEditor v-else v-model="fileContent" max-height="60vh" />
    </article>
  </dialog>

  <dialog ref="permDialog" @click="onDialogClick($event, closePerms)">
    <article v-if="permEntry">
      <h4>Permissions: {{ permEntry.name }}</h4>
      <div v-if="permError" class="error-banner">{{ permError }}</div>
      <table>
        <thead>
          <tr>
            <th></th>
            <th v-for="c in permCols" :key="c.label">{{ c.label }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in permRows" :key="row.label">
            <td>{{ row.label }}</td>
            <td v-for="c in permCols" :key="c.label">
              <input
                type="checkbox"
                :checked="permHas(c.bit << row.shift)"
                @change="togglePerm(c.bit << row.shift, $event.target.checked)"
              />
            </td>
          </tr>
        </tbody>
      </table>
      <label>
        Octal
        <input v-model="permOctal" maxlength="3" style="width: 6rem" />
      </label>
      <label v-if="permEntry.isDir">
        <input v-model="permRecursive" type="checkbox" />
        Apply recursively (same mode to everything inside)
      </label>
      <footer>
        <button class="secondary" @click="closePerms">Cancel</button>
        <button @click="savePerms">Apply</button>
      </footer>
    </article>
  </dialog>
  </div>
</template>

<style scoped>
.files-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.files-view > :not(.dropzone) {
  flex: none;
}
.dropzone table {
  margin-bottom: 0;
}
.dropzone thead th {
  position: sticky;
  top: 0;
  z-index: 2;
  background: var(--pico-background-color);
}
.ctx-menu {
  position: fixed;
  z-index: 1000;
  margin: 0;
  padding: 0.25rem 0;
  list-style: none;
  min-width: 10rem;
  background: var(--pico-card-background-color);
  border: 1px solid var(--pico-muted-border-color);
  border-radius: var(--pico-border-radius);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.3);
}
.ctx-menu li {
  margin: 0;
  padding: 0;
  list-style: none;
}
.ctx-menu a {
  display: block;
  padding: 0.35rem 1rem;
  text-decoration: none;
  color: var(--pico-color);
}
.ctx-menu a.danger-link {
  color: var(--pico-del-color, #c62828);
}
.ctx-menu a:hover {
  background: var(--pico-dropdown-hover-background-color, rgba(128, 128, 128, 0.2));
}
nav[aria-label='breadcrumb'] {
  --pico-nav-element-spacing-vertical: 0.15rem;
}
nav[aria-label='breadcrumb'] ul {
  margin-bottom: 0;
}
.sortable {
  cursor: pointer;
  user-select: none;
}
.dropzone {
  flex: 1;
  min-height: 0;
  overflow: auto;
  /* push the scrollbar out to the viewport edge, past main's padding */
  margin-right: calc(-1 * var(--pico-spacing));
  padding-right: var(--pico-spacing);
  border: 2px dashed transparent;
}
.dropzone.dragging {
  border-color: var(--pico-primary);
}
</style>
