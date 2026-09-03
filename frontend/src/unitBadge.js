// Shared helpers for rendering systemd unit status badges, including the
// "activating (auto-restart)" crash-loop case, which systemd reports as a
// neutral "activating" state even though the unit is actually failing.
export function isRestarting(u) {
  return u.sub === 'auto-restart'
}

export function unitBadgeClass(u) {
  return ['badge', u.active, isRestarting(u) ? 'restarting' : '']
}

export function restartingLabel(u) {
  if (!isRestarting(u)) return ''
  const parts = [`${u.nRestarts} restart${u.nRestarts === 1 ? '' : 's'}`]
  if (u.sinceTimestamp) parts.push(`failing for ${formatDuration(Date.now() - new Date(u.sinceTimestamp).getTime())}`)
  return parts.join(', ')
}

export function formatDuration(ms) {
  const s = Math.max(0, Math.floor(ms / 1000))
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ${m % 60}m`
  const d = Math.floor(h / 24)
  return `${d}d ${h % 24}h`
}
