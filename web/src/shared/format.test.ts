import { describe, expect, it } from "vitest";
import { formatDate, formatRevenue } from "./format";

describe("formatDate", () => {
  it("formats an ISO date in Russian", () => {
    const value = formatDate("2026-10-18T09:00:00Z");

    expect(value).toContain("18");
    expect(value).toContain("октября");
    expect(value).toMatch(/\d{2}:\d{2}/);
  });

  it("returns an empty string for an invalid date", () => {
    expect(formatDate("")).toBe("");
    expect(formatDate("not-a-date")).toBe("");
  });
});

describe("formatRevenue", () => {
  it("formats a numeric string as compact rubles", () => {
    const value = formatRevenue("2500000000000");

    expect(value).toContain("трлн");
    expect(value).toContain("₽");
  });

  it("keeps non-numeric values untouched", () => {
    expect(formatRevenue("нет данных")).toBe("нет данных");
    expect(formatRevenue("")).toBe("");
    expect(formatRevenue(undefined)).toBe("");
  });
});
