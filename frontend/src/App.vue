<script setup>
</script>

<template>
  <header class="container-fluid">
    <nav>
      <ul>
        <li><strong>Podtainer</strong></li>
      </ul>
      <ul>
        <li><RouterLink to="/stacks">Stacks</RouterLink></li>
        <li><RouterLink to="/quadlets">Quadlets</RouterLink></li>
        <li><RouterLink to="/systemd">Systemd</RouterLink></li>
        <li><RouterLink to="/containers">Containers</RouterLink></li>
      </ul>
    </nav>
  </header>
  <main class="container-fluid">
    <RouterView />
  </main>
</template>
