import { createApp } from 'vue'
import '@picocss/pico/css/pico.min.css'
import './style.css'
import App from './App.vue'
import router from './router.js'

createApp(App).use(router).mount('#app')
