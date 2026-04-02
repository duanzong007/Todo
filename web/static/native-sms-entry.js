(() => {
  function isNativeSMSAvailable() {
    return !!(window.Capacitor && window.Capacitor.Plugins && window.Capacitor.Plugins.SmsBridge);
  }

  function initializeNativeSMSEntry(scope = document) {
    if (!isNativeSMSAvailable()) {
      return;
    }

    scope.querySelectorAll("[data-composer-tab='sms']").forEach((button) => {
      if (button.dataset.nativeSmsBound === "1") {
        return;
      }
      button.dataset.nativeSmsBound = "1";
      button.addEventListener(
        "click",
        (event) => {
          event.preventDefault();
          event.stopImmediatePropagation();
          const currentPath = `${window.location.pathname}${window.location.search}`;
          window.location.assign(`/sms/native?return=${encodeURIComponent(currentPath)}`);
        },
        true,
      );
    });
  }

  window.initializeNativeSMSEntry = initializeNativeSMSEntry;

  document.addEventListener("DOMContentLoaded", () => {
    initializeNativeSMSEntry(document);
  });
})();
