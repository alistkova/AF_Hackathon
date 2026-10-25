import type { FeedFacet, FeedFilters, FeedSort } from "../../api/types";

export type FeedFilterState = {
  query: string;
  sources: string[];
  tags: string[];
  sectors: string[];
  from: string;
  to: string;
  sort: FeedSort;
};

export function createDefaultFeedFilters(): FeedFilterState {
  return {
    query: "",
    sources: [],
    tags: [],
    sectors: [],
    from: "",
    to: "",
    sort: "relevance",
  };
}

export function hasActiveFeedFilters(filters: FeedFilterState): boolean {
  return Boolean(
    filters.sources.length ||
      filters.tags.length ||
      filters.sectors.length ||
      filters.from ||
      filters.to ||
      filters.query.trim(),
  );
}

export function activeFeedFilterCount(filters: FeedFilterState): number {
  return (
    filters.sources.length +
    filters.tags.length +
    filters.sectors.length +
    (filters.query.trim() ? 1 : 0) +
    (filters.from || filters.to ? 1 : 0)
  );
}

export function hasInvalidFeedDateRange(filters: FeedFilterState): boolean {
  return Boolean(filters.from && filters.to && filters.from > filters.to);
}

export function toggleValue(values: string[], value: string): string[] {
  return values.includes(value)
    ? values.filter((item) => item !== value)
    : [...values, value];
}

export function feedFiltersEqual(left: FeedFilterState, right: FeedFilterState): boolean {
  return (
    left.query === right.query &&
    left.from === right.from &&
    left.to === right.to &&
    left.sort === right.sort &&
    left.sources.join("\u0000") === right.sources.join("\u0000") &&
    left.tags.join("\u0000") === right.tags.join("\u0000") &&
    left.sectors.join("\u0000") === right.sectors.join("\u0000")
  );
}

export function filterFacetOptions(
  options: FeedFacet[],
  selected: string[],
  query: string,
): FeedFacet[] {
  const term = query.trim().toLowerCase();
  if (!term) return options;
  return options.filter(
    (option) => option.value.toLowerCase().includes(term) || selected.includes(option.value),
  );
}

export function toApiFeedFilters(
  filters: FeedFilterState,
  query = filters.query,
): FeedFilters {
  return {
    sources: filters.sources.length ? filters.sources : undefined,
    tags: filters.tags.length ? filters.tags : undefined,
    sectors: filters.sectors.length ? filters.sectors : undefined,
    query: query.trim() || undefined,
    from: filters.from || undefined,
    to: filters.to || undefined,
    sort: filters.sort,
  };
}
