import type { CapacitorConfig } from '@capacitor/cli'

/**
 * Konfigurasi Capacitor untuk membungkus SPA Steca POS menjadi aplikasi
 * Android. UI tidak ditulis ulang: WebView memuat hasil `npm run build`
 * (folder `dist`) yang disalin ke project Android lewat `npx cap sync`.
 *
 * Catatan penting soal jaringan: WebView Android berjalan pada origin
 * `https://localhost`, sehingga `localhost` di dalam aplikasi menunjuk ke
 * perangkat itu sendiri — bukan ke komputer pengembang. Karena itu build
 * Android WAJIB memakai `VITE_API_BASE_URL` berisi URL penuh backend
 * (lihat `.env.android.example` dan bagian Android di README).
 */
const config: CapacitorConfig = {
  appId: 'id.stecapos.app',
  appName: 'Steca POS',
  webDir: 'dist',

  android: {
    // Skema https membuat origin WebView menjadi https://localhost sehingga
    // API backend yang memakai HTTPS tidak tertolak mixed-content.
    androidScheme: 'https',
  },

  plugins: {
    SplashScreen: {
      launchShowDuration: 1200,
      launchAutoHide: true,
      // Warna merek (--brand-900) agar splash menyatu dengan sidebar aplikasi.
      backgroundColor: '#073b36',
      androidSplashResourceName: 'splash',
      androidScaleType: 'CENTER_CROP',
      showSpinner: false,
      splashFullScreen: true,
      splashImmersive: false,
    },
    StatusBar: {
      style: 'DARK',
      backgroundColor: '#073b36',
    },
  },
}

export default config
