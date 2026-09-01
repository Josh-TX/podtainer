import { createRouter, createWebHistory } from 'vue-router'
import StacksView from './views/StacksView.vue'
import StackDetailView from './views/StackDetailView.vue'
import QuadletsView from './views/QuadletsView.vue'
import SystemdView from './views/SystemdView.vue'
import ContainersView from './views/ContainersView.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/stacks' },
    { path: '/stacks', component: StacksView },
    { path: '/stacks/new', component: StackDetailView, props: { isNew: true } },
    { path: '/stacks/:name', component: StackDetailView, props: true },
    { path: '/quadlets', component: QuadletsView },
    { path: '/systemd', component: SystemdView },
    { path: '/containers', component: ContainersView },
  ],
})
