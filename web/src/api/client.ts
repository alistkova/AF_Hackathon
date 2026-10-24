import type { Company, FeedItem, InsightJob, User } from "./types";

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { message?: string } | null;
    throw new Error(body?.message ?? `Request failed: ${response.status}`);
  }

  return (await response.json()) as T;
}

async function requestVoid(path: string, init?: RequestInit): Promise<void> {
  const response = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { message?: string } | null;
    throw new Error(body?.message ?? `Request failed: ${response.status}`);
  }
}

export const api = {
  me: () => request<User>("/api/v1/me"),
  login: (email: string, password: string) =>
    request<{ token: string; user: User }>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }).then((session) => session.user),
  logout: () => requestVoid("/api/v1/auth/logout", { method: "POST" }),
  companies: () => request<{ items: Company[] }>("/api/v1/companies"),
  company: (id: string) => request<Company>(`/api/v1/companies/${id}`),
  feed: (companyId: string) =>
    request<{ items: FeedItem[] }>(`/api/v1/feed?company_id=${encodeURIComponent(companyId)}`),
  createJob: (companyId: string, event: FeedItem) =>
    request<InsightJob>("/api/v1/insights/jobs", {
      method: "POST",
      body: JSON.stringify({
        company_id: companyId,
        event: {
          id: event.event_id,
          title: event.title,
          summary: event.summary,
          source: event.source,
          url: event.url,
          published_at: event.published_at,
        },
      }),
    }),
};

export function eventsUrl(jobId: string): string {
  return `${API_BASE}/api/v1/insights/jobs/${jobId}/events`;
}
