async function request(path, opts = {}) {
  const res = await fetch('/api' + path, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  })
  const body = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(body.error || res.statusText)
  }
  return body
}

const enc = encodeURIComponent

export const stacksApi = {
  list: () => request('/stacks'),
  get: (name) => request(`/stacks/${enc(name)}`),
  deploy: (name, content, force = false) =>
    request(`/stacks/${enc(name)}`, { method: 'PUT', body: JSON.stringify({ content, force }) }),
  delete: (name) => request(`/stacks/${enc(name)}`, { method: 'DELETE' }),
  pull: (name) => request(`/stacks/${enc(name)}/pull`, { method: 'POST' }),
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
}

export const systemdApi = {
  list: () => request('/systemd'),
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
