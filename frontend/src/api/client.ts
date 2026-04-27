import type { AccountActionResponse, AccountPageData, DashboardSnapshot } from "../types";

export class APIError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly payload?: unknown,
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

function requestURL(path: string, search = ""): string {
  const url = new URL(endpoint(path), window.location.origin);
  if (search) {
    const normalized = search.startsWith("?") ? search.slice(1) : search;
    url.search = normalized;
  }
  return apiBase() ? url.toString() : `${url.pathname}${url.search}`;
}

async function parseError(response: Response, fallback: string): Promise<never> {
  let payload: unknown;
  let message = fallback;
  try {
    payload = await response.json();
    if (
      payload &&
      typeof payload === "object" &&
      "error" in payload &&
      typeof (payload as { error?: unknown }).error === "string"
    ) {
      message = (payload as { error: string }).error;
    }
  } catch (_error) {
    message = (await response.text().catch(() => fallback)) || fallback;
  }
  throw new APIError(message, response.status, payload);
}

export async function fetchDashboardSnapshot(focusDate?: string): Promise<DashboardSnapshot> {
  const search = new URLSearchParams();
  if (focusDate) {
    search.set("date", focusDate);
  }

  const response = await fetch(requestURL("/dashboard/snapshot", search.toString()), {
    credentials: "include",
    cache: "no-store",
    headers: {
      "X-Requested-With": "fetch",
    },
  });

  if (!response.ok) {
    await parseError(response, response.status === 401 ? "unauthorized" : "snapshot request failed");
  }

  return response.json() as Promise<DashboardSnapshot>;
}

export async function fetchAccountData(search = window.location.search): Promise<AccountPageData> {
  const response = await fetch(requestURL("/me/data", search), {
    credentials: "include",
    cache: "no-store",
    headers: {
      "X-Requested-With": "fetch",
    },
  });

  if (!response.ok) {
    await parseError(response, response.status === 401 ? "unauthorized" : "account request failed");
  }

  return response.json() as Promise<AccountPageData>;
}

export async function applyAccountAction(formData: FormData): Promise<AccountActionResponse> {
  const response = await fetch(requestURL("/me/tasks/apply"), {
    method: "POST",
    body: formData,
    credentials: "include",
    cache: "no-store",
    headers: {
      "X-Requested-With": "fetch",
    },
  });

  if (!response.ok) {
    await parseError(response, "操作失败");
  }

  return response.json() as Promise<AccountActionResponse>;
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
