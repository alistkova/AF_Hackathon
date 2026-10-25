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

describe("profile api", () => {
  it("patches the current user", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          id: "usr_1",
          email: "new@example.com",
          role: "analyst",
          primary_company_id: "magnit",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const user = await api.updateProfile({ email: "new@example.com", primary_company_id: "magnit" });
    expect(user.email).toBe("new@example.com");

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/me");
    expect(init?.method).toBe("PATCH");
    expect(JSON.parse(init?.body as string)).toEqual({
      email: "new@example.com",
      primary_company_id: "magnit",
    });
  });

  it("posts the password change", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await api.changePassword("secret-pass", "next-secret");

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/me/password");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(init?.body as string)).toEqual({
      current_password: "secret-pass",
      new_password: "next-secret",
    });
  });

  it("reads memberships and company members", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ items: [{ company_id: "magnit", role: "owner" }] }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ items: [{ user_id: "usr_1", email: "a@b.com", role: "member" }] }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);

    const memberships = await api.myMemberships();
    expect(memberships.items[0].role).toBe("owner");
    expect(fetchMock.mock.calls[0][0]).toBe("/api/v1/me/memberships");

    const members = await api.companyMembers("magnit");
    expect(members.items[0].email).toBe("a@b.com");
    expect(fetchMock.mock.calls[1][0]).toBe("/api/v1/companies/magnit/members");
  });

  it("creates a company", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: "cmp_1", name: "Ромашка" }), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const company = await api.createCompany({ name: "Ромашка", inn: "7701234567" });
    expect(company.id).toBe("cmp_1");

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/companies");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(init?.body as string)).toEqual({ name: "Ромашка", inn: "7701234567" });
  });

  it("updates a company", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: "cmp_1", name: "Ромашка+" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const updated = await api.updateCompany("cmp_1", { name: "Ромашка+" });
    expect(updated.name).toBe("Ромашка+");

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/companies/cmp_1");
    expect(init?.method).toBe("PUT");
    expect(JSON.parse(init?.body as string)).toEqual({ name: "Ромашка+" });
  });

  it("adds and removes company members", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ user_id: "usr_2", email: "member@example.com", role: "member" }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    const member = await api.addCompanyMember("magnit", "member@example.com");
    expect(member.user_id).toBe("usr_2");

    const [addUrl, addInit] = fetchMock.mock.calls[0];
    expect(addUrl).toBe("/api/v1/companies/magnit/members");
    expect(addInit?.method).toBe("POST");
    expect(JSON.parse(addInit?.body as string)).toEqual({ email: "member@example.com" });

    await api.removeCompanyMember("magnit", "usr_2");
    const [removeUrl, removeInit] = fetchMock.mock.calls[1];
    expect(removeUrl).toBe("/api/v1/companies/magnit/members/usr_2");
    expect(removeInit?.method).toBe("DELETE");
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
