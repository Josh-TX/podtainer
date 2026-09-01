<script setup>
</script>

<template>
  <div class="app">
    <nav class="topnav">
      <span class="brand">Podtainer</span>
      <RouterLink to="/stacks">Stacks</RouterLink>
      <RouterLink to="/quadlets">Quadlets</RouterLink>
      <RouterLink to="/systemd">Systemd</RouterLink>
      <RouterLink to="/containers">Containers</RouterLink>
    </nav>
    <main class="content">
      <RouterView />
    </main>
  </div>
</template>

<style>
* { box-sizing: border-box; }
body { margin: 0; font-family: system-ui, sans-serif; background: #111418; color: #e6e6e6; }
a { color: #7db8ff; }
.app { display: flex; flex-direction: column; min-height: 100vh; }
.topnav { display: flex; align-items: center; gap: 1.25rem; padding: 0.75rem 1.5rem; background: #1a1e24; border-bottom: 1px solid #2a2f37; }
.topnav .brand { font-weight: 700; margin-right: 1rem; }
.topnav a { text-decoration: none; color: #b8c0cc; padding: 0.25rem 0; }
.topnav a.router-link-active { color: #fff; border-bottom: 2px solid #7db8ff; }
.content { padding: 1.5rem; flex: 1; }
table { width: 100%; border-collapse: collapse; }
th, td { text-align: left; padding: 0.5rem 0.75rem; border-bottom: 1px solid #2a2f37; }
th { color: #9aa4b2; font-weight: 600; font-size: 0.85rem; text-transform: uppercase; }
.row-table { display: table; width: 100%; border-collapse: collapse; }
.row-table-row { display: table-row; }
.row-table-row > * { display: table-cell; text-align: left; padding: 0.5rem 0.75rem; border-bottom: 1px solid #2a2f37; vertical-align: middle; }
.row-table-head { color: #9aa4b2; font-weight: 600; font-size: 0.85rem; text-transform: uppercase; }
a.row-table-row { text-decoration: none; color: inherit; cursor: pointer; }
a.row-table-row:hover { background: #1e232b; }
button { cursor: pointer; background: #262b33; color: #e6e6e6; border: 1px solid #3a4048; border-radius: 4px; padding: 0.35rem 0.75rem; font-size: 0.85rem; }
button:hover { background: #323945; }
button.danger:hover { background: #5a2a2a; border-color: #7a3a3a; }
button.primary { background: #2d5fa8; border-color: #3a6fc0; }
button.primary:hover { background: #3a6fc0; }
button:disabled { opacity: 0.5; cursor: not-allowed; }
textarea { width: 100%; font-family: ui-monospace, monospace; font-size: 0.85rem; background: #0d0f12; color: #e6e6e6; border: 1px solid #3a4048; border-radius: 4px; padding: 0.75rem; }
pre.logs { background: #0d0f12; border: 1px solid #3a4048; border-radius: 4px; padding: 0.75rem; max-height: 400px; overflow: auto; font-size: 0.8rem; white-space: pre-wrap; }
.badge { display: inline-block; padding: 0.1rem 0.5rem; border-radius: 10px; font-size: 0.75rem; font-weight: 600; }
.badge.active, .badge.healthy, .badge.deployed { background: #1f4d2e; color: #7de89a; }
.badge.inactive, .badge.none { background: #3a3f47; color: #b8c0cc; }
.badge.failed, .badge.unhealthy { background: #5a1f1f; color: #ff8a8a; }
.badge.drift, .badge.notdeployed, .badge.starting { background: #5a4a1f; color: #ffd27d; }
.error-banner { background: #5a1f1f; color: #ffb8b8; border: 1px solid #7a3a3a; border-radius: 4px; padding: 0.75rem; margin-bottom: 1rem; white-space: pre-wrap; }
.warn-banner { background: #4a3f1f; color: #ffe08a; border: 1px solid #7a6a3a; border-radius: 4px; padding: 0.75rem; margin-bottom: 1rem; }
.toolbar { display: flex; gap: 0.5rem; margin-bottom: 1rem; flex-wrap: wrap; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
.muted { color: #9aa4b2; font-size: 0.85rem; }
.loading { display: flex; align-items: center; gap: 0.6rem; color: #9aa4b2; padding: 1.5rem 0; opacity: 0; animation: fade-in 0.8s ease forwards; }
.spinner { width: 1.1rem; height: 1.1rem; border: 2px solid #3a4048; border-top-color: #7db8ff; border-radius: 50%; animation: spin 0.7s linear infinite; flex-shrink: 0; }
@keyframes spin { to { transform: rotate(360deg); } }
@keyframes fade-in { to { opacity: 1; } }
</style>
