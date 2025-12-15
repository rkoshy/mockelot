import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'
import { consoleCapture } from './utils/consoleCapture'
import { useLogStore } from './stores/logStore'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)

// Install console capture after Pinia is initialized
// Forward captured console logs to log store
const logStore = useLogStore()
consoleCapture.install((entry) => {
  logStore.addLog(entry)
})

app.mount('#app')
