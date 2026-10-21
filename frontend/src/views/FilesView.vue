<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
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
const filterText = ref('')

const selectedName = ref('')
const fileContent = ref('')
const originalContent = ref('')
const notEditable = ref(false)
const fileError = ref('')
const fileBusy = ref(false)

const selection = ref(new Set())
const anchor = ref('')
const focusName = ref('') // last-clicked row, gets the border
const clipboard = ref(null) // { mode: 'cut' | 'copy', dir, names }
const renaming = ref('')
const renameValue = ref('')
let renameInput = null
let renameTimer = null
let renameDone = false

const conflictDialog = ref(null)
const conflict = ref({ name: '', index: 0, total: 0, all: false })
let conflictResolve = null

function resolveConflict(choice) {
  conflictResolve?.(choice)
}

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
  const q = filterText.value.trim().toLowerCase()
  return entries.value.filter((e) => (showHidden.value || !e.name.startsWith('.')) && (!q || e.name.toLowerCase().includes(q))).sort((a, b) => {
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

function joinPath(dir, name) {
  return dir === '/' ? `/${name}` : `${dir}/${name}`
}

function fullPath(name) {
  return joinPath(currentPath.value, name)
}

async function loadDir(path) {
  closeFile()
  clearSelection()
  filterText.value = ''
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

const editingPath = ref(false)
const crumbNav = ref(null)
watch([currentPath, editingPath], async () => {
  await nextTick()
  if (crumbNav.value) crumbNav.value.scrollLeft = crumbNav.value.scrollWidth
})
const pathInput = ref('')
const pathInputEl = ref(null)
const pathRest = computed({
  get: () => pathInput.value.slice(1),
  set: (v) => { pathInput.value = '/' + v.replace(/^\/+/, '') },
})
const suggestions = ref([])
const sugIndex = ref(-1)
let sugSeq = 0

async function startEditPath(e) {
  if (e.target.closest('a')) return
  pathInput.value = currentPath.value === '/' ? '/' : currentPath.value + '/'
  editingPath.value = true
  await nextTick()
  pathInputEl.value?.focus()
  updateSuggestions()
}

function cancelEditPath() {
  editingPath.value = false
  suggestions.value = []
  sugIndex.value = -1
}

async function updateSuggestions() {
  const seq = ++sugSeq
  const v = pathInput.value
  const i = v.lastIndexOf('/')
  const parent = v.slice(0, i + 1) || '/'
  const prefix = v.slice(i + 1).toLowerCase()
  try {
    const data = await filesApi.list(parent)
    if (seq !== sugSeq) return
    suggestions.value = (data.entries || [])
      .filter((x) => x.isDir && x.name.toLowerCase().startsWith(prefix))
      .map((x) => parent + x.name)
      .slice(0, 50)
  } catch {
    if (seq !== sugSeq) return
    suggestions.value = []
  }
  sugIndex.value = prefix && suggestions.value.length ? 0 : -1
}

function pickSuggestion(p) {
  pathInput.value = p + '/'
  if (p !== currentPath.value) loadDir(p)
  pathInputEl.value?.focus()
  updateSuggestions()
}

async function submitPath() {
  let p = pathInput.value.trim()
  if (p.length > 1) p = p.replace(/\/+$/, '')
  cancelEditPath()
  if (p && p !== currentPath.value) await loadDir(p)
}

function onPathInput(e) {
  const v = pathInput.value
  if (e.inputType?.startsWith('insert') && v.endsWith('/')) {
    const p = v.length > 1 ? v.replace(/\/+$/, '') : v
    if (p && p !== currentPath.value) loadDir(p)
  }
  updateSuggestions()
}

function onPathKey(e) {
  if (e.key === 'ArrowDown' && suggestions.value.length) {
    e.preventDefault()
    sugIndex.value = (sugIndex.value + 1) % suggestions.value.length
  } else if (e.key === 'ArrowUp' && suggestions.value.length) {
    e.preventDefault()
    sugIndex.value = (sugIndex.value - 1 + suggestions.value.length) % suggestions.value.length
  } else if (e.key === 'Tab' && suggestions.value.length) {
    e.preventDefault()
    pickSuggestion(suggestions.value[Math.max(sugIndex.value, 0)])
  } else if (e.key === 'Enter') {
    e.preventDefault()
    if (sugIndex.value >= 0) pickSuggestion(suggestions.value[sugIndex.value])
    else submitPath()
  } else if (e.key === 'Escape') {
    cancelEditPath()
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
  a.download = path.split('/').pop()
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

// --- selection ---

const selectedEntries = computed(() => sortedEntries.value.filter((e) => selection.value.has(e.name)))
const singleSelected = computed(() => (selectedEntries.value.length === 1 ? selectedEntries.value[0] : null))
const clipHere = computed(() => clipboard.value && clipboard.value.dir === currentPath.value)

function clearSelection() {
  cancelRenameTimer()
  selection.value = new Set()
  anchor.value = ''
  focusName.value = ''
}

function selectOnly(name) {
  focusName.value = name
  selection.value = new Set([name])
  anchor.value = name
}

function cancelRenameTimer() {
  clearTimeout(renameTimer)
  renameTimer = null
}

function onRowClick(e, entry) {
  cancelRenameTimer()
  if (renaming.value) return
  const name = entry.name
  focusName.value = name
  if (e.ctrlKey || e.metaKey) {
    const next = new Set(selection.value)
    if (!next.delete(name)) next.add(name)
    selection.value = next
    anchor.value = name
  } else if (e.shiftKey) {
    const names = sortedEntries.value.map((x) => x.name)
    const a = names.indexOf(anchor.value)
    const b = names.indexOf(name)
    if (a < 0) return selectOnly(name)
    const next = new Set(selection.value)
    for (let i = Math.min(a, b); i <= Math.max(a, b); i++) next.add(names[i])
    selection.value = next
  } else if (selection.value.size === 1 && selection.value.has(name)) {
    renameTimer = setTimeout(() => startRename(entry), 300)
  } else {
    selectOnly(name)
  }
}

function onRowDblClick(entry) {
  cancelRenameTimer()
  if (renaming.value) return
  openEntry(entry)
}

function countText(list) {
  const dirs = list.filter((e) => e.isDir).length
  const files = list.length - dirs
  const parts = []
  if (dirs) parts.push(`${dirs} ${dirs === 1 ? 'folder' : 'folders'}`)
  if (files) parts.push(`${files} ${files === 1 ? 'file' : 'files'}`)
  return parts.join(', ') || '0 items'
}

function filterCountText() {
  const all = entries.value.filter((e) => showHidden.value || !e.name.startsWith('.'))
  const totalDirs = all.filter((e) => e.isDir).length
  const dirs = sortedEntries.value.filter((e) => e.isDir).length
  const files = sortedEntries.value.length - dirs
  return `Showing ${dirs}/${totalDirs} folders, ${files}/${all.length - totalDirs} files`
}

const statusText = computed(() => {
  let text = selection.value.size
    ? `${countText(selectedEntries.value)} selected`
    : filterText.value.trim() ? filterCountText() : countText(sortedEntries.value)
  return text
})

function onWindowClick(e) {
  closeMenu()
  if (e.target.closest?.('tr, button, a, input, label, summary, dialog, .ctx-menu')) return
  clearSelection()
}

function selectAll() {
  selection.value = new Set(sortedEntries.value.map((e) => e.name))
}

// --- rename ---

async function startRename(entry) {
  cancelRenameTimer()
  renameDone = false
  renameValue.value = entry.name
  renaming.value = entry.name
  await nextTick()
  const el = renameInput
  if (!el) return
  el.focus()
  const dot = entry.isDir ? -1 : entry.name.lastIndexOf('.')
  el.setSelectionRange(0, dot > 0 ? dot : entry.name.length)
}

async function commitRename() {
  if (renameDone) return
  renameDone = true
  const old = renaming.value
  const name = renameValue.value.trim()
  renaming.value = ''
  if (!name || name === old) return
  if (name.includes('/')) {
    dirError.value = 'Name cannot contain "/"'
    return
  }
  dirError.value = ''
  try {
    await filesApi.move(fullPath(old), fullPath(name), false)
    await refreshListing()
    selectOnly(name)
  } catch (e) {
    dirError.value = e.message
  }
}

function cancelRename() {
  renameDone = true
  renaming.value = ''
}

// --- clipboard ---

function stash(mode) {
  if (!selection.value.size || clipboard.value) return
  clipboard.value = { mode, dir: currentPath.value, names: [...selection.value] }
  clearSelection()
}

function cancelClipboard() {
  clipboard.value = null
}

function askConflict(names) {
  return new Promise((resolve) => {
    const decided = {}
    let i = 0
    conflictResolve = (choice) => {
      if (!choice) {
        conflictResolve = null
        resolve(null)
        return
      }
      if (conflict.value.all) {
        for (; i < names.length; i++) decided[names[i]] = choice
      } else {
        decided[names[i++]] = choice
      }
      if (i >= names.length) {
        conflictResolve = null
        conflictDialog.value.close()
        resolve(decided)
        return
      }
      conflict.value = { name: names[i], index: i + 1, total: names.length, all: false }
    }
    conflict.value = { name: names[0], index: 1, total: names.length, all: false }
    conflictDialog.value.showModal()
  })
}

function onConflictClose() {
  // dialog dismissed (Esc / backdrop / Cancel) while still undecided
  if (conflictResolve) conflictResolve(null)
}

async function paste(targetDir = currentPath.value) {
  const cb = clipboard.value
  if (!cb) return
  const isCopy = cb.mode === 'copy'
  const same = cb.dir === targetDir
  if (same && !isCopy) {
    clipboard.value = null
    return
  }
  dirError.value = ''
  try {
    const resolutions = {}
    if (same) {
      for (const n of cb.names) resolutions[n] = 'rename'
    } else {
      const existing = new Set(await filesApi.conflicts(cb.names.map((n) => joinPath(targetDir, n))))
      const conflicts = cb.names.filter((n) => existing.has(joinPath(targetDir, n)))
      if (conflicts.length) {
        const decided = await askConflict(conflicts)
        if (!decided) return
        Object.assign(resolutions, decided)
      }
    }
    const items = cb.names.map((n) => ({
      src: joinPath(cb.dir, n),
      dest: joinPath(targetDir, n),
      resolution: resolutions[n] || '',
    }))
    const errors = await filesApi.paste(items, isCopy)
    if (!isCopy) clipboard.value = null
    await reload()
    if (errors.length) dirError.value = errors.map((x) => `${x.path}: ${x.error}`).join('; ')
  } catch (e) {
    dirError.value = e.message
  }
}

async function deleteSelected() {
  const list = selectedEntries.value
  if (!list.length) return
  if (list.length === 1) return deleteEntry(list[0])
  if (!confirm(`Delete ${list.length} items?`)) return
  dirError.value = ''
  try {
    const errors = await filesApi.deleteMany(list.map((e) => fullPath(e.name)))
    await reload()
    if (errors.length) dirError.value = errors.map((x) => `${x.path}: ${x.error}`).join('; ')
  } catch (e) {
    dirError.value = e.message
  }
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
  if (!selection.value.has(entry.name)) selectOnly(entry.name)
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
  if (e.target.closest?.('input, textarea, select, dialog, [contenteditable]')) return
  if (fileDialog.value?.open || permDialog.value?.open || conflictDialog.value?.open) return
  const mod = e.ctrlKey || e.metaKey
  const key = e.key.toLowerCase()
  if (e.key === 'Escape') {
    if (selection.value.size) clearSelection()
    else cancelClipboard()
  } else if (mod && key === 'a') {
    e.preventDefault()
    selectAll()
  } else if (mod && key === 'x') {
    stash('cut')
  } else if (mod && key === 'c') {
    stash('copy')
  } else if (mod && key === 'v') {
    paste()
  } else if (e.key === 'Delete') {
    deleteSelected()
  } else if (e.key === 'F2' && singleSelected.value) {
    e.preventDefault()
    startRename(singleSelected.value)
  } else if (e.key === 'Enter' && singleSelected.value) {
    openEntry(singleSelected.value)
  }
}

onMounted(() => {
  window.addEventListener('click', onWindowClick)
  window.addEventListener('resize', closeMenu)
  window.addEventListener('scroll', closeMenu, true)
  window.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => {
  window.removeEventListener('click', onWindowClick)
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
  <div class="toolbar action-bar">
    <span v-if="clipboard" class="clip-info" :title="clipboard.mode === 'cut' ? 'Cut' : 'Copied'">
      <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="5" y="4" width="14" height="18" rx="2"/><path d="M9 2h6v4H9z"/><path d="M9 11h6M9 14.5h6M9 18h6"/></svg>
      {{ clipboard.names.length }} {{ clipboard.names.length === 1 ? 'item' : 'items' }}
    </span>
    <template v-if="selection.size">
      <template v-if="clipboard">
        <button class="secondary" @click="paste()">Paste</button>
        <button class="secondary" @click="cancelClipboard">Cancel {{ clipboard.mode }}</button>
      </template>
      <template v-else>
        <button class="secondary" @click="stash('cut')">Cut</button>
        <button class="secondary" @click="stash('copy')">Copy</button>
      </template>
      <button class="secondary" @click="deleteSelected">Delete</button>
      <button class="secondary" @click="clearSelection">Deselect all</button>
    </template>
    <template v-else-if="clipboard">
      <button class="secondary" @click="paste()">Paste</button>
      <button class="secondary" @click="cancelClipboard">Cancel</button>
    </template>
    <template v-else>
      <button class="secondary" @click="newFile">New File</button>
      <button class="secondary" @click="newFolder">New Folder</button>
      <button class="secondary" @click="triggerUpload">Upload</button>
    </template>
    <input ref="fileInput" type="file" multiple style="display: none" @change="onUploadChange" />
  </div>
  </div>

  <div class="toolbar">
    <div class="path-box" @click="!editingPath && startEditPath($event)">
      <nav v-if="!editingPath" ref="crumbNav" aria-label="breadcrumb">
        <ul>
          <li v-for="(crumb, i) in breadcrumbs" :key="crumb.path">
            <a v-if="i < breadcrumbs.length - 1" href="#" @click.prevent="loadDir(crumb.path)">{{ crumb.name }}</a>
            <strong v-else>{{ crumb.name }}</strong>
          </li>
        </ul>
      </nav>
      <template v-else>
        <span class="path-root">/</span>
        <input
          ref="pathInputEl"
          v-model="pathRest"
          class="path-input"
          type="text"
          spellcheck="false"
          autocomplete="off"
          @input="onPathInput"
          @keydown="onPathKey"
          @blur="cancelEditPath"
        />
        <ul v-if="suggestions.length" class="path-suggestions">
          <li
            v-for="(sg, i) in suggestions"
            :key="sg"
            :class="{ active: i === sugIndex }"
            @mousedown.prevent="pickSuggestion(sg)"
          >{{ sg }}</li>
        </ul>
      </template>
    </div>
  </div>

  <div class="toolbar">
    <div class="filter-wrap">
      <input v-model="filterText" class="filter-input" type="text" placeholder="Filter files and folders" autocomplete="off" @keydown.esc="filterText = ''" />
      <button v-if="filterText" type="button" class="filter-clear" aria-label="Clear filter" @click="filterText = ''">&times;</button>
    </div>
    <label style="margin: 0">
      <input v-model="showHidden" type="checkbox" />
      Show hidden
    </label>
  </div>

  <p class="muted status-line">{{ statusText }}</p>

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
            <th class="sortable" @click="setSort('name')">Name<span v-if="sortKey === 'name'" class="chev" :class="{ up: sortAsc }"></span></th>
            <th class="sortable" @click="setSort('size')">Size<span v-if="sortKey === 'size'" class="chev" :class="{ up: sortAsc }"></span></th>
            <th class="sortable" @click="setSort('modTime')">Modified<span v-if="sortKey === 'modTime'" class="chev" :class="{ up: sortAsc }"></span></th>
            <th class="sortable" @click="setSort('mode')">Mode<span v-if="sortKey === 'mode'" class="chev" :class="{ up: sortAsc }"></span></th>
            <th>Owner</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="entry in sortedEntries"
            :key="entry.name"
            :class="{ selected: selection.has(entry.name), focused: focusName === entry.name, cut: clipboard?.mode === 'cut' && clipHere && clipboard.names.includes(entry.name) }"
            @click="onRowClick($event, entry)"
            @dblclick="onRowDblClick(entry)"
            @contextmenu.prevent="openMenu($event, entry)"
          >
            <td>
              <span v-if="renaming === entry.name" class="rename-wrap">
                {{ entry.isDir ? '📁' : '📄' }}
                <input
                :ref="(el) => (renameInput = el)"
                v-model="renameValue"
                class="rename-input"
                @click.stop
                @dblclick.stop
                @keydown.enter.prevent="commitRename"
                @keydown.esc.stop.prevent="cancelRename"
                @blur="commitRename"
                />
              </span>
              <template v-else>{{ entry.isDir ? '📁 ' : '📄 ' }}{{ entry.name }}<span v-if="entry.isSymlink" class="muted"> → {{ entry.linkTarget }}</span></template>
            </td>
            <td class="muted" style="position: relative; z-index: 1">
              <template v-if="entry.isDir">
                <span v-if="sizeLoading[entry.name]" aria-busy="true"></span>
                <template v-else-if="dirSizes[entry.name] !== undefined">{{ humanSize(dirSizes[entry.name]) }}</template>
                <a v-else href="#" @click.prevent.stop="computeSize(entry)">compute?</a>
              </template>
              <template v-else>{{ humanSize(entry.size) }}</template>
            </td>
            <td class="muted" :title="fullDate(entry.modTime)">{{ timeAgo(entry.modTime, true) }}</td>
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
    <template v-if="singleSelected">
      <li><a href="#" @click.prevent="menuAction(openEntry)">Open</a></li>
      <li v-if="!menu.entry.isDir"><a href="#" @click.prevent="menuAction((e) => triggerDownload(fullPath(e.name)))">Download</a></li>
      <li><a href="#" @click.prevent="menuAction(startRename)">Rename</a></li>
    </template>
    <template v-if="!clipboard">
      <li><a href="#" @click.prevent="menuAction(() => stash('cut'))">Cut</a></li>
      <li><a href="#" @click.prevent="menuAction(() => stash('copy'))">Copy</a></li>
    </template>
    <li v-if="clipboard && singleSelected?.isDir"><a href="#" @click.prevent="menuAction((e) => paste(fullPath(e.name)))">Paste into</a></li>
    <li v-if="singleSelected"><a href="#" @click.prevent="menuAction(openPerms)">Permissions</a></li>
    <li><a href="#" class="danger-link" @click.prevent="menuAction(deleteSelected)">Delete</a></li>
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

  <dialog ref="conflictDialog" @close="onConflictClose">
    <article>
      <h4>Name conflict</h4>
      <p>
        "{{ conflict.name }}" already exists in this folder.
        <span class="muted">({{ conflict.index }} of {{ conflict.total }})</span>
      </p>
      <label v-if="conflict.total > 1">
        <input v-model="conflict.all" type="checkbox" />
        Apply to all
      </label>
      <footer>
        <button class="secondary" @click="conflictDialog.close()">Cancel</button>
        <button class="secondary" @click="resolveConflict('skip')">Skip</button>
        <button class="secondary" @click="resolveConflict('rename')">Keep both</button>
        <button @click="resolveConflict('overwrite')">Override</button>
      </footer>
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
.clip-info {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.9rem;
  white-space: nowrap;
}
.action-bar {
  margin-bottom: 0;
}
.filter-wrap {
  position: relative;
  flex: 1;
  min-width: 0;
}
.filter-clear {
  position: absolute;
  right: 0.25rem;
  top: 50%;
  transform: translateY(-50%);
  width: 2rem;
  height: 2rem;
  padding: 0;
  margin: 0;
  border: 0;
  background: transparent;
  color: var(--pico-color);
  font-size: 1.6rem;
  line-height: 1;
}
.filter-clear:hover {
  background: rgba(128, 128, 128, 0.25);
}
.filter-input {
  width: 100%;
  padding-right: 2.5rem;
  margin: 0;
  border-radius: 0;
  height: 2.25rem;
  font-size: 0.9rem;
}
.status-line {
  margin: 0 0 0.5rem;
  font-size: 0.85rem;
}
.path-box {
  position: relative;
  flex: 1;
  min-width: 0;
  max-width: 100%;
  border: var(--pico-border-width) solid var(--pico-form-element-border-color);
  border-radius: var(--pico-border-radius);
  background: var(--pico-form-element-background-color);
  padding: 0.25rem 0.75rem;
  cursor: text;
}
.path-box:focus-within {
  border-color: var(--pico-form-element-active-border-color);
}
.path-box nav[aria-label='breadcrumb'] {
  display: block;
  min-width: 0;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: thin;
}
.path-box nav[aria-label='breadcrumb'] ul {
  display: flex;
  flex-wrap: nowrap;
  white-space: nowrap;
  width: max-content;
  min-width: 100%;
}
.path-box nav[aria-label='breadcrumb'] li {
  flex-shrink: 0;
}
.path-box nav[aria-label='breadcrumb'] {
  min-height: 1.75rem;
  align-items: center;
}
.path-root {
  position: absolute;
  left: 0.75rem;
  top: 50%;
  transform: translateY(-50%);
  font-family: monospace;
  font-size: 1rem;
  line-height: 1;
  pointer-events: none;
  user-select: none;
}
.path-input {
  width: 100%;
  margin: 0;
  padding: 0.2rem 0 0.2rem 1.5ch;
  font-size: 1rem;
  border: 0;
  background: transparent;
  box-shadow: none;
  font-family: monospace;
  height: 1.75rem;
}
.path-input:focus {
  box-shadow: none;
}
.path-suggestions {
  position: absolute;
  left: -1px;
  right: -1px;
  top: 100%;
  z-index: 50;
  margin: 2px 0 0;
  padding: 0.25rem 0;
  max-height: 10rem;
  overflow-y: auto;
  list-style: none;
  background: var(--pico-card-background-color, var(--pico-background-color));
  border: var(--pico-border-width) solid var(--pico-form-element-border-color);
  border-radius: var(--pico-border-radius);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.25);
}
.path-suggestions li {
  margin: 0;
  padding: 0.25rem 0.75rem;
  font-family: monospace;
  cursor: pointer;
}
.path-suggestions li:hover,
.path-suggestions li.active {
  background: rgba(59, 130, 246, 0.2);
}
nav[aria-label='breadcrumb'] {
  --pico-nav-element-spacing-vertical: 0.15rem;
}
nav[aria-label='breadcrumb'] ul {
  margin-bottom: 0;
}
table.rows tbody tr {
  user-select: none;
}
table.rows tbody tr.selected {
  --sel-border: rgba(59, 130, 246, 0.3);
}
table.rows tbody tr.selected.focused {
  --sel-border: rgba(59, 130, 246, 0.7);
}
table.rows tbody tr.selected td {
  background-color: rgba(59, 130, 246, 0.12);
  box-shadow: inset 0 1px 0 var(--sel-border), inset 0 -1px 0 var(--sel-border);
}
table.rows tbody tr.selected td:first-child {
  box-shadow: inset 1px 1px 0 var(--sel-border), inset 0 -1px 0 var(--sel-border);
}
table.rows tbody tr.selected td:last-child {
  box-shadow: inset -1px 1px 0 var(--sel-border), inset 0 -1px 0 var(--sel-border);
}
table.rows tbody tr.cut td {
  opacity: 0.45;
}
.rename-wrap {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}
.rename-input {
  margin: 0;
  padding: 0.1rem 0.4rem;
  height: auto;
  width: min(100%, 24rem);
}
.chev {
  display: inline-block;
  width: 0.45em;
  height: 0.45em;
  margin-left: 0.5em;
  border-right: 2px solid currentColor;
  border-bottom: 2px solid currentColor;
  transform: translateY(-0.15em) rotate(45deg);
}
.chev.up {
  transform: translateY(0.1em) rotate(-135deg);
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
