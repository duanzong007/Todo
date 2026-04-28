(() => {
  if (!("serviceWorker" in navigator)) {
    return;
  }

  const SW_URL = "/sw.js?v=20260405-native-shell-1";
  let refreshing = false;

  function isNativeShell() {
    try {
      if (!window.Capacitor) {
        return false;
      }
      if (typeof window.Capacitor.isNativePlatform === "function") {
        return window.Capacitor.isNativePlatform();
      }
      if (typeof window.Capacitor.getPlatform === "function") {
        return window.Capacitor.getPlatform() !== "web";
      }
      return true;
    } catch (_error) {
      return false;
    }
  }

  async function clearTodoPWACaches() {
    if (!("caches" in window)) {
      return;
    }
    try {
      const keys = await caches.keys();
      await Promise.all(
        keys
          .filter((key) => key.startsWith("todo-pwa-"))
          .map((key) => caches.delete(key))
      );
    } catch (_error) {
      // Ignore cache cleanup errors in the browser.
    }
  }

  async function disablePWAForNativeShell() {
    try {
      const registrations = await navigator.serviceWorker.getRegistrations();
      await Promise.all(registrations.map((registration) => registration.unregister()));
    } catch (_error) {
      // Ignore unregister errors in the browser.
    }

    await clearTodoPWACaches();
  }

  navigator.serviceWorker.addEventListener("controllerchange", () => {
    if (refreshing) {
      return;
    }
    refreshing = true;
    window.location.reload();
  });

  window.addEventListener("load", () => {
    if (isNativeShell()) {
      disablePWAForNativeShell();
      return;
    }

    navigator.serviceWorker.register(SW_URL).then((registration) => {
      registration.update().catch(() => { });
    }).catch((error) => {
      console.warn("failed to register service worker", error);
    });
  });
})();
