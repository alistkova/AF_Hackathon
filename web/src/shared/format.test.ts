import { describe, expect, it } from "vitest";
import { formatDate } from "./format";

describe("formatDate", () => {
  it("formats an ISO date in Russian", () => {
    const value = formatDate("2026-10-18T09:00:00Z");

    expect(value).toContain("18");
    expect(value).toContain("октября");
    expect(value).toMatch(/\d{2}:\d{2}/);
  });
});
