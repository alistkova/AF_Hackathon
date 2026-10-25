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
  return isFilterSelected(values, value)
    ? values.filter((item) => !filterValuesEqual(item, value))
    : [...values, value];
}

export function normalizeFilterValue(value: string): string {
  return value.trim().toLowerCase().replace(/ё/g, "е").replace(/\s+/g, " ");
}

export function filterValuesEqual(left: string, right: string): boolean {
  return normalizeFilterValue(left) === normalizeFilterValue(right);
}

export function isFilterSelected(values: string[], value: string): boolean {
  return values.some((item) => filterValuesEqual(item, value));
}

function sameFilterValues(left: string[], right: string[]): boolean {
  return (
    left.length === right.length &&
    left.every((item) => isFilterSelected(right, item))
  );
}

export function feedFiltersEqual(left: FeedFilterState, right: FeedFilterState): boolean {
  return (
    left.query === right.query &&
    left.from === right.from &&
    left.to === right.to &&
    left.sort === right.sort &&
    sameFilterValues(left.sources, right.sources) &&
    sameFilterValues(left.tags, right.tags) &&
    sameFilterValues(left.sectors, right.sectors)
  );
}

export function filterFacetOptions(
  options: FeedFacet[],
  selected: string[],
  query: string,
): FeedFacet[] {
  const term = normalizeFilterValue(query);
  if (!term) return options;
  return options.filter(
    (option) =>
      normalizeFilterValue(option.value).includes(term) || isFilterSelected(selected, option.value),
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
