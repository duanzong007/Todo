import type { DashboardSnapshot } from "../types";

export class APIError extends Error {
  constructor(
    message: string,
    public readonly status: number,
  ) {
    super(message);
  }
}

function apiBase(): string {
  return import.meta.env.VITE_TODO_API_BASE || "";
}

function endpoint(path: string): string {
  const base = apiBase().replace(/\/$/, "");
  return `${base}${path}`;
}

export async function fetchDashboardSnapshot(focusDate?: string): Promise<DashboardSnapshot> {
  const url = new URL(endpoint("/dashboard/snapshot"), window.location.origin);
  if (focusDate) {
    url.searchParams.set("date", focusDate);
  }

  const requestURL = apiBase() ? url.toString() : `${url.pathname}${url.search}`;
  const response = await fetch(requestURL, {
    credentials: "include",
    cache: "no-store",
    headers: {
      "X-Requested-With": "fetch",
    },
  });

  if (response.status === 401) {
    throw new APIError("unauthorized", response.status);
  }
  if (!response.ok) {
    throw new APIError("snapshot request failed", response.status);
  }

  return response.json() as Promise<DashboardSnapshot>;
}

export function openDashboardEvents(onDashboard: () => void): EventSource {
  const stream = new EventSource(endpoint("/events"), {
    withCredentials: true,
  });

  const handler = () => {
    onDashboard();
  };
  stream.addEventListener("dashboard", handler);
  stream.onmessage = handler;
  return stream;
}
