import { describe, expect, it } from "vitest";
import {
  activeFeedFilterCount,
  createDefaultFeedFilters,
  feedFiltersEqual,
  filterFacetOptions,
  hasActiveFeedFilters,
  hasInvalidFeedDateRange,
  isFilterSelected,
  toApiFeedFilters,
  toggleValue,
} from "./filters";

describe("feed filters", () => {
  it("toggles values without duplicates", () => {
    expect(toggleValue([], "ритейл")).toEqual(["ритейл"]);
    expect(toggleValue(["ритейл"], "ритейл")).toEqual([]);
    expect(toggleValue(["ритейл"], "логистика")).toEqual(["ритейл", "логистика"]);
  });

  it("treats filter values case-insensitively", () => {
    expect(toggleValue(["Магнит"], "магнит")).toEqual([]);
    expect(toggleValue(["магнит"], "Магнит")).toEqual([]);
    expect(isFilterSelected(["Магнит"], "магнит")).toBe(true);
    expect(feedFiltersEqual(
      { ...createDefaultFeedFilters(), tags: ["Магнит"] },
      { ...createDefaultFeedFilters(), tags: ["магнит"] },
    )).toBe(true);
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

  it("detects pending changes against the applied filters", () => {
    const applied = createDefaultFeedFilters();
    const draft = { ...applied, tags: ["ритейл"], sort: "date" as const };

    expect(feedFiltersEqual(applied, { ...applied })).toBe(true);
    expect(feedFiltersEqual(applied, draft)).toBe(false);
    expect(feedFiltersEqual(draft, { ...draft })).toBe(true);
  });

  it("filters facet options but keeps selected values visible", () => {
    const options = [
      { value: "Ведомости", count: 3 },
      { value: "РБК", count: 2 },
      { value: "Коммерсантъ", count: 1 },
    ];

    expect(filterFacetOptions(options, [], "рб")).toEqual([{ value: "РБК", count: 2 }]);
    expect(filterFacetOptions(options, ["Ведомости"], "рб")).toEqual([
      { value: "Ведомости", count: 3 },
      { value: "РБК", count: 2 },
    ]);
    expect(filterFacetOptions(options, [], "  ")).toEqual(options);
  });
});
