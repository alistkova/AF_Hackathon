import { describe, expect, it } from "vitest";
import {
  activeFeedFilterCount,
  createDefaultFeedFilters,
  hasActiveFeedFilters,
  hasInvalidFeedDateRange,
  toApiFeedFilters,
  toggleValue,
} from "./filters";

describe("feed filters", () => {
  it("toggles values without duplicates", () => {
    expect(toggleValue([], "ритейл")).toEqual(["ритейл"]);
    expect(toggleValue(["ритейл"], "ритейл")).toEqual([]);
    expect(toggleValue(["ритейл"], "логистика")).toEqual(["ритейл", "логистика"]);
  });

  it("uses from-to dates and sort", () => {
    const filters = {
      ...createDefaultFeedFilters(),
      from: "2026-09-01",
      to: "2026-09-30",
      sort: "date" as const,
    };
    const api = toApiFeedFilters(filters);

    expect(api.from).toBe("2026-09-01");
    expect(api.to).toBe("2026-09-30");
    expect(api.sort).toBe("date");
    expect(hasInvalidFeedDateRange(filters)).toBe(false);
    expect(hasActiveFeedFilters(filters)).toBe(true);
  });

  it("omits empty date bounds", () => {
    const api = toApiFeedFilters(createDefaultFeedFilters());

    expect(api.from).toBeUndefined();
    expect(api.to).toBeUndefined();
  });

  it("detects reversed dates", () => {
    const filters = {
      ...createDefaultFeedFilters(),
      from: "2026-09-30",
      to: "2026-09-01",
    };

    expect(hasInvalidFeedDateRange(filters)).toBe(true);
  });

  it("counts active filter groups", () => {
    const filters = {
      ...createDefaultFeedFilters(),
      query: "ставка",
      sources: ["РБК"],
      tags: ["ритейл", "Магнит"],
      from: "2026-09-01",
    };

    expect(activeFeedFilterCount(filters)).toBe(5);
  });
});
