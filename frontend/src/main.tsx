import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import App from './App'
import { setupNative } from './lib/native'
import './styles.css'

const container = document.getElementById('root')
if (!container) throw new Error('Elemen #root tidak ditemukan')

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
)

// Penyesuaian Android (splash screen, status bar). Tidak berpengaruh di web.
void setupNative()
