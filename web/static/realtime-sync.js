(() => {
  const SYNC_DEBOUNCE_MS = 180;
  const RECONNECT_DELAY_MS = 2200;

  if (!window.EventSource) {
    return;
  }

  if (!document.querySelector("main.focus-page")) {
    return;
  }

  const clientID = (() => {
    if (window.todoClientID) {
      return window.todoClientID;
    }
    if (window.crypto && typeof window.crypto.randomUUID === "function") {
      window.todoClientID = window.crypto.randomUUID();
      return window.todoClientID;
    }
    window.todoClientID = `todo-${Date.now()}-${Math.random().toString(16).slice(2)}`;
    return window.todoClientID;
  })();

  let eventSource = null;
  let reconnectTimer = 0;
  let syncTimer = 0;
  let syncInFlight = false;
  let syncQueued = false;
  let staleWhileHidden = false;
  let initialPageShowHandled = false;

  function currentFocusDate() {
    if (typeof window.getFocusDateForSync === "function") {
      return window.getFocusDateForSync();
    }

    const url = new URL(window.location.href);
    return (url.searchParams.get("date") || "").trim();
  }

  function hasPendingMutations() {
    return typeof window.todoHasPendingTaskRequests === "function" && window.todoHasPendingTaskRequests();
  }

  function clearReconnectTimer() {
    if (!reconnectTimer) {
      return;
    }
    window.clearTimeout(reconnectTimer);
    reconnectTimer = 0;
  }

  function fetchSnapshot() {
    const url = new URL("/dashboard/snapshot", window.location.origin);
    const focusDate = currentFocusDate();
    if (focusDate) {
      url.searchParams.set("date", focusDate);
    }

    return fetch(url.pathname + url.search, {
      cache: "no-store",
      headers: {
        "X-Requested-With": "fetch",
      },
    }).then(async (response) => {
      if (!response.ok) {
        throw new Error("同步任务数据失败");
      }
      return response.json();
    });
  }

  async function syncNow() {
    if (syncInFlight) {
      syncQueued = true;
      return;
    }

    if (document.visibilityState !== "visible") {
      staleWhileHidden = true;
      return;
    }

    if (hasPendingMutations()) {
      syncQueued = true;
      return;
    }

    syncInFlight = true;
    syncQueued = false;

    try {
      const snapshot = await fetchSnapshot();
      if (typeof window.applyTaskSnapshotQuietly === "function") {
        window.applyTaskSnapshotQuietly(snapshot);
      } else if (typeof window.applyTaskSnapshot === "function") {
        window.applyTaskSnapshot(snapshot);
      }
    } catch (_error) {
      // Ignore transient realtime sync errors.
    } finally {
      syncInFlight = false;
      if (syncQueued && document.visibilityState === "visible" && !hasPendingMutations()) {
        scheduleSync();
      }
    }
  }

  function scheduleSync() {
    window.clearTimeout(syncTimer);
    syncTimer = window.setTimeout(() => {
      syncNow();
    }, SYNC_DEBOUNCE_MS);
  }

  function scheduleReconnect() {
    if (reconnectTimer) {
      return;
    }
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = 0;
      connect();
    }, RECONNECT_DELAY_MS);
  }

  function handleDashboardEvent(event) {
    let payload = null;
    try {
      payload = JSON.parse(event.data || "{}");
    } catch (_error) {
      payload = null;
    }

    if (payload && payload.actor_client_id && payload.actor_client_id === clientID) {
      return;
    }

    if (document.visibilityState !== "visible") {
      staleWhileHidden = true;
      return;
    }

    syncQueued = true;
    scheduleSync();
  }

  function connect() {
    clearReconnectTimer();

    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }

    eventSource = new EventSource("/events");
    eventSource.addEventListener("dashboard", handleDashboardEvent);
    eventSource.onmessage = handleDashboardEvent;
    eventSource.onerror = () => {
      if (eventSource) {
        eventSource.close();
        eventSource = null;
      }
      scheduleReconnect();
    };
  }

  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState !== "visible") {
      return;
    }

    if (typeof window.ensureImplicitTodayViewIsFresh === "function" && window.ensureImplicitTodayViewIsFresh()) {
      return;
    }

    if (staleWhileHidden) {
      staleWhileHidden = false;
      syncQueued = true;
      scheduleSync();
    }
  });

  window.addEventListener("pageshow", (event) => {
    if (typeof window.ensureImplicitTodayViewIsFresh === "function" && window.ensureImplicitTodayViewIsFresh()) {
      return;
    }

    if (!initialPageShowHandled) {
      initialPageShowHandled = true;
      if (!event.persisted) {
        return;
      }
    }

    syncQueued = true;
    scheduleSync();
  });

  window.addEventListener("focus", () => {
    if (typeof window.ensureImplicitTodayViewIsFresh === "function" && window.ensureImplicitTodayViewIsFresh()) {
      return;
    }

    if (staleWhileHidden) {
      staleWhileHidden = false;
      syncQueued = true;
      scheduleSync();
    }
  });

  window.addEventListener("todo:pending-drained", () => {
    if (syncQueued && document.visibilityState === "visible") {
      scheduleSync();
    }
  });

  window.addEventListener("beforeunload", () => {
    clearReconnectTimer();
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  });

  connect();
})();
