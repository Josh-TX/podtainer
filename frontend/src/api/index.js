import { authState } from './authState.js'

async function request(path, opts = {}) {
  const res = await fetch('/api' + path, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  })
  if (res.status === 401 && path !== '/auth/status' && path !== '/auth/login') {
    authState.authenticated = false
  }
  const body = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(body.error || res.statusText)
  }
  return body
}

const enc = encodeURIComponent

export const authApi = {
  status: () => request('/auth/status'),
  setup: (password) => request('/auth/setup', { method: 'POST', body: JSON.stringify({ password }) }),
  login: (password) => request('/auth/login', { method: 'POST', body: JSON.stringify({ password }) }),
  logout: () => request('/auth/logout', { method: 'POST' }),
}

export const stacksApi = {
  list: () => request('/stacks'),
  get: (name) => request(`/stacks/${enc(name)}`),
  deploy: (name, content, { force = false, pull = false, isCreate = false } = {}) =>
    request(`/stacks/${enc(name)}`, { method: 'PUT', body: JSON.stringify({ content, force, pull, isCreate }) }),
  delete: (name, { stack, quadlet, images, volumes }) => {
    const params = new URLSearchParams({ stack, quadlet, images, volumes })
    return request(`/stacks/${enc(name)}?${params}`, { method: 'DELETE' })
  },
  serviceLogs: (name, service, lines = 200) =>
    request(`/stacks/${enc(name)}/services/${enc(service)}/logs?lines=${lines}`),
}

export const quadletsApi = {
  list: () => request('/quadlets'),
  get: (filename) => request(`/quadlets/${enc(filename)}`),
  write: (filename, content) =>
    request(`/quadlets/${enc(filename)}`, { method: 'PUT', body: JSON.stringify({ content }) }),
  delete: (filename) => request(`/quadlets/${enc(filename)}`, { method: 'DELETE' }),
  start: (filename) => request(`/quadlets/${enc(filename)}/start`, { method: 'POST' }),
  stop: (filename) => request(`/quadlets/${enc(filename)}/stop`, { method: 'POST' }),
  restart: (filename) => request(`/quadlets/${enc(filename)}/restart`, { method: 'POST' }),
  logs: (filename, lines = 200) => request(`/quadlets/${enc(filename)}/logs?lines=${lines}`),
  generatorLogs: (lines = 200) => request(`/quadlets/generator-logs?lines=${lines}`),
}

export const systemdApi = {
  list: ({ all = false, quadlet = false, favorite = false } = {}) =>
    request(`/systemd?all=${all}&quadlet=${quadlet}&favorite=${favorite}`),
  logs: (name, lines = 200) => request(`/systemd/${enc(name)}/logs?lines=${lines}`),
  content: (name) => request(`/systemd/${enc(name)}/content`),
  writeContent: (name, content) =>
    request(`/systemd/${enc(name)}/content`, { method: 'PUT', body: JSON.stringify({ content }) }),
  setFavorite: (name, favorite) =>
    request(`/systemd/${enc(name)}/favorite`, { method: 'PUT', body: JSON.stringify({ favorite }) }),
  start: (name) => request(`/systemd/${enc(name)}/start`, { method: 'POST' }),
  stop: (name) => request(`/systemd/${enc(name)}/stop`, { method: 'POST' }),
  restart: (name) => request(`/systemd/${enc(name)}/restart`, { method: 'POST' }),
  enable: (name) => request(`/systemd/${enc(name)}/enable`, { method: 'POST' }),
  disable: (name) => request(`/systemd/${enc(name)}/disable`, { method: 'POST' }),
}

function encPath(path) {
  return (path || '').split('/').filter(Boolean).map(enc).join('/')
}

function fsUrl(name, path) {
  return `/volumes/${enc(name)}/fs/${encPath(path)}`
}

// fs() hits the unified GET endpoint, which returns either
// {isDir: true, entries} or {isDir: false, size, content}.
function fs(name, path) {
  return request(fsUrl(name, path))
}

export const volumesApi = {
  list: () => request('/volumes'),
  get: (name) => request(`/volumes/${enc(name)}`),
  listDir: async (name, path) => (await fs(name, path)).entries,
  readFile: (name, path) => fs(name, path),
  writeFile: (name, path, content) =>
    request(fsUrl(name, path), { method: 'PUT', body: JSON.stringify({ content }) }),
  createFile: (name, path) => request(fsUrl(name, path), { method: 'POST', body: JSON.stringify({ type: 'file' }) }),
  mkdir: (name, path) => request(fsUrl(name, path), { method: 'POST', body: JSON.stringify({ type: 'dir' }) }),
  deleteEntry: (name, path) => request(fsUrl(name, path), { method: 'DELETE' }),
  move: (name, path, dest, isCopy) =>
    request(fsUrl(name, path), { method: 'PATCH', body: JSON.stringify({ dest, isCopy }) }),
  downloadUrl: (name, path) => `/api${fsUrl(name, path)}?download=1`,
  async upload(name, path, file) {
    const res = await fetch(`/api${fsUrl(name, path)}?upload=1`, { method: 'POST', body: file })
    const body = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(body.error || res.statusText)
    return body
  },
}

export const imagesApi = {
  list: () => request('/images'),
  delete: (id) => request(`/images/${enc(id)}`, { method: 'DELETE' }),
  prune: (all) => request(`/images/prune?all=${all ? '1' : '0'}`, { method: 'POST' }),
}

export const containersApi = {
  list: () => request('/containers'),
  stats: (id) => request(`/containers/${enc(id)}/stats`),
  logs: (id, lines = 200) => request(`/containers/${enc(id)}/logs?lines=${lines}`),
  start: (id) => request(`/containers/${enc(id)}/start`, { method: 'POST' }),
  stop: (id) => request(`/containers/${enc(id)}/stop`, { method: 'POST' }),
  restart: (id) => request(`/containers/${enc(id)}/restart`, { method: 'POST' }),
  remove: (id) => request(`/containers/${enc(id)}`, { method: 'DELETE' }),
}
