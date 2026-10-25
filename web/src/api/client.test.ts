import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "./client";
import type { FeedItem } from "./types";

const event: FeedItem = {
  event_id: "news-001",
  company_id: "magnit",
  title: "Магнит расширяет логистику",
  summary: "Компания расширяет распределительные центры.",
  source: "Ведомости",
  url: "https://example.com/news-001",
  published_at: "2026-10-18T09:00:00Z",
  relevance: 0.86,
  reasons: ["Упоминание компании", "Отрасль: Торговля"],
  tags: ["ритейл", "Магнит"],
  target_sectors: ["Торговля"],
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("api.feed", () => {
  it("passes filter values to the feed endpoint", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          items: [],
          total: 0,
          facets: { sources: [], tags: [], sectors: [] },
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await api.feed(
      "magnit",
      {
        sources: ["Ведомости", "РБК"],
        tags: ["ритейл"],
        sectors: ["Торговля"],
        query: "логистика",
        from: "2026-09-01",
        sort: "date",
      },
      10,
      20,
    );

    const [url] = fetchMock.mock.calls[0];
    const parsed = new URL(url as string, "http://localhost");
    expect(parsed.pathname).toBe("/api/v1/feed");
    expect(parsed.searchParams.get("company_id")).toBe("magnit");
    expect(parsed.searchParams.getAll("source")).toEqual(["Ведомости", "РБК"]);
    expect(parsed.searchParams.getAll("tag")).toEqual(["ритейл"]);
    expect(parsed.searchParams.getAll("sector")).toEqual(["Торговля"]);
    expect(parsed.searchParams.get("q")).toBe("логистика");
    expect(parsed.searchParams.get("from")).toBe("2026-09-01");
    expect(parsed.searchParams.get("sort")).toBe("date");
    expect(parsed.searchParams.get("limit")).toBe("10");
    expect(parsed.searchParams.get("offset")).toBe("20");
  });
});

describe("api.createJob", () => {
  it("posts the company and event to the jobs endpoint", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          job_id: "job-1",
          company_id: "magnit",
          status: "queued",
          events_url: "/api/v1/insights/jobs/job-1/events",
          result_url: "/api/v1/insights/jobs/job-1",
        }),
        {
          status: 202,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const job = await api.createJob("magnit", event);
    expect(job.job_id).toBe("job-1");

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/insights/jobs");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(init?.body as string)).toMatchObject({
      company_id: "magnit",
      event: {
        id: "news-001",
        title: "Магнит расширяет логистику",
        url: "https://example.com/news-001",
      },
    });
  });

  it("uses the server error message", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ message: "insight unavailable" }), {
          status: 502,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    await expect(api.companies()).rejects.toThrow("insight unavailable");
  });
});
