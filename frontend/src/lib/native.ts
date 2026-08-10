// Penyesuaian khusus saat aplikasi berjalan sebagai APK Android.
// Seluruh UI dipakai apa adanya; berkas ini hanya mengatur splash screen dan
// status bar bawaan Android.

import { Capacitor } from '@capacitor/core'
import { SplashScreen } from '@capacitor/splash-screen'
import { StatusBar, Style } from '@capacitor/status-bar'

/** Warna merek (--brand-900) yang dipakai splash screen dan status bar. */
const BRAND_DARK = '#073b36'

/**
 * setupNative dijalankan sekali saat aplikasi dimuat. Pada web biasa fungsi
 * ini tidak melakukan apa pun, sehingga bundel browser tetap berperilaku sama.
 */
export async function setupNative(): Promise<void> {
  if (!Capacitor.isNativePlatform()) return

  try {
    await StatusBar.setBackgroundColor({ color: BRAND_DARK })
    await StatusBar.setStyle({ style: Style.Dark })
  } catch {
    // Beberapa perangkat/ROM tidak mengizinkan pewarnaan status bar; abaikan
    // saja karena ini murni kosmetik.
  }

  try {
    // Splash disembunyikan setelah React siap agar tidak ada layar putih.
    await SplashScreen.hide()
  } catch {
    // Splash mungkin sudah tersembunyi otomatis.
  }
}
