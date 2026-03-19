(() => {
  if (!("serviceWorker" in navigator)) {
    return;
  }

  const SW_URL = "/sw.js?v=20260319-pwa-day-rollover-1";
  let refreshing = false;

  navigator.serviceWorker.addEventListener("controllerchange", () => {
    if (refreshing) {
      return;
    }
    refreshing = true;
    window.location.reload();
  });

  window.addEventListener("load", () => {
    navigator.serviceWorker.register(SW_URL).then((registration) => {
      registration.update().catch(() => {});
    }).catch((error) => {
      console.warn("failed to register service worker", error);
    });
  });
})();
