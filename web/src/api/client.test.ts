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
};

afterEach(() => {
  vi.unstubAllGlobals();
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
