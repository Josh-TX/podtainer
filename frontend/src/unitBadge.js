// Shared helpers for rendering systemd unit status badges, including the
// "activating (auto-restart)" crash-loop case, which systemd reports as a
// neutral "activating" state even though the unit is actually failing.
export function isRestarting(u) {
  return u.sub === 'auto-restart'
}

// An orphaned unit is one systemd is still running (or the run just failed)
// even though it no longer has a backing file, e.g. because the quadlet
// generator failed to regenerate it on the last daemon-reload (a syntax
// error in the source file, most commonly).
export function isOrphaned(u) {
  return u.load === 'not-found' && u.active !== 'inactive'
}

export function unitStatusLabel(u) {
  return isOrphaned(u) ? 'orphaned' : u.active
}

export function unitBadgeClass(u) {
  return ['badge', isOrphaned(u) ? 'orphaned' : u.active, isRestarting(u) ? 'restarting' : '']
}

export function statusTitle(u) {
  if (isOrphaned(u)) {
    return 'This unit has no backing file (its quadlet source failed to regenerate it, e.g. after a syntax error) but is still running from before. Fix the source file and reload, or stop it manually.'
  }
  return restartingLabel(u)
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
