import { createRouter, createWebHistory } from 'vue-router'
import StacksView from './views/StacksView.vue'
import StackDetailView from './views/StackDetailView.vue'
import QuadletsView from './views/QuadletsView.vue'
import QuadletDetailView from './views/QuadletDetailView.vue'
import SystemdView from './views/SystemdView.vue'
import SystemdDetailView from './views/SystemdDetailView.vue'
import ContainersView from './views/ContainersView.vue'
import ContainerDetailView from './views/ContainerDetailView.vue'
import VolumesView from './views/VolumesView.vue'
import VolumeDetailView from './views/VolumeDetailView.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/stacks' },
    { path: '/stacks', component: StacksView },
    { path: '/stacks/new', component: StackDetailView, props: { isNew: true } },
    { path: '/stacks/:name', component: StackDetailView, props: true },
    { path: '/quadlets', component: QuadletsView },
    { path: '/quadlets/:filename', component: QuadletDetailView, props: true },
    { path: '/systemd', component: SystemdView },
    { path: '/systemd/:name', component: SystemdDetailView, props: true },
    { path: '/containers', component: ContainersView },
    { path: '/containers/:id', component: ContainerDetailView, props: true },
    { path: '/volumes', component: VolumesView },
    { path: '/volumes/:name', component: VolumeDetailView, props: true },
  ],
})
