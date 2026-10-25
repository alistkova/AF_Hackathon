import { useState } from "react";
import type { FeedFacet, FeedFacets } from "../../api/types";
import {
  activeFeedFilterCount,
  hasActiveFeedFilters,
  hasInvalidFeedDateRange,
  toggleValue,
  type FeedFilterState,
} from "./filters";

type Props = {
  filters: FeedFilterState;
  facets: FeedFacets | null;
  onChange: (filters: FeedFilterState) => void;
  onReset: () => void;
};

type FacetGroupProps = {
  label: string;
  facets: FeedFacet[];
  selected: string[];
  onToggle: (value: string) => void;
};

function FacetGroup({ label, facets, selected, onToggle }: FacetGroupProps) {
  const [expanded, setExpanded] = useState(true);
  const extras = selected
    .filter((value) => !facets.some((facet) => facet.value === value))
    .map((value) => ({ value, count: 0 }));
  const options = [...facets, ...extras];

  return (
    <div>
      <button
        type="button"
        onClick={() => setExpanded((value) => !value)}
        aria-expanded={expanded}
        className="flex w-full items-center justify-between gap-2 text-left"
      >
        <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">
          {label}
        </span>
        <span className="flex items-center gap-1.5 text-[10px] text-slate-400">
          {selected.length > 0 && (
            <span className="rounded-full bg-slate-900 px-1.5 py-0.5 font-semibold text-white">
              {selected.length}
            </span>
          )}
          <span className="w-3 text-center text-xs font-semibold">{expanded ? "−" : "+"}</span>
        </span>
      </button>

      {expanded && (
        <div className="mt-1 max-h-40 space-y-1 overflow-y-auto pr-1">
          {options.length === 0 && <p className="text-xs text-slate-400">Нет значений</p>}
          {options.map((facet) => {
            const active = selected.includes(facet.value);
            return (
              <button
                key={facet.value}
                type="button"
                aria-pressed={active}
                title={facet.value}
                onClick={() => onToggle(facet.value)}
                className={`flex w-full items-center justify-between gap-2 rounded-lg px-2 py-1.5 text-left text-xs transition ${
                  active
                    ? "bg-slate-900 font-medium text-white"
                    : "bg-slate-50 text-slate-600 hover:bg-slate-100"
                }`}
              >
                <span className="truncate">{facet.value}</span>
                <span className="shrink-0 text-[10px] opacity-70">{facet.count}</span>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

export function FeedFilters({ filters, facets, onChange, onReset }: Props) {
  const [open, setOpen] = useState(() =>
    typeof window === "undefined" ? true : window.matchMedia("(min-width: 1024px)").matches,
  );
  const sources = facets?.sources ?? [];
  const tags = facets?.tags ?? [];
  const sectors = facets?.sectors ?? [];
  const activeCount = activeFeedFilterCount(filters);
  const invalidDateRange = hasInvalidFeedDateRange(filters);

  return (
    <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between gap-2">
        <div>
          <p className="text-sm font-semibold text-slate-800">Фильтры</p>
          <p className="mt-1 text-xs leading-5 text-slate-400">
            Поиск и отбор применяются ко всей ленте
          </p>
        </div>
        <button
          type="button"
          onClick={() => setOpen((value) => !value)}
          aria-expanded={open}
          aria-controls="feed-filters"
          className="shrink-0 rounded-lg border border-slate-200 px-2.5 py-1.5 text-[11px] font-semibold text-slate-600 transition hover:bg-slate-50 lg:hidden"
        >
          {open ? "Скрыть" : activeCount ? `Фильтры (${activeCount})` : "Открыть"}
        </button>
      </div>

      <div
        id="feed-filters"
        className={`${open ? "mt-3 block" : "hidden"} space-y-3 lg:mt-3 lg:block`}
      >
        <label className="block">
          <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">
            Поиск
          </span>
          <input
            type="search"
            value={filters.query}
            onChange={(event) => onChange({ ...filters, query: event.target.value })}
            placeholder="Заголовок, текст или тег"
            className="mt-1 w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 outline-none transition placeholder:text-slate-400 focus:border-[#ef3124]"
          />
        </label>

        <FacetGroup
          label="Источники"
          facets={sources}
          selected={filters.sources}
          onToggle={(source) =>
            onChange({ ...filters, sources: toggleValue(filters.sources, source) })
          }
        />

        <FacetGroup
          label="Темы"
          facets={tags}
          selected={filters.tags}
          onToggle={(tag) => onChange({ ...filters, tags: toggleValue(filters.tags, tag) })}
        />

        <FacetGroup
          label="Отрасли"
          facets={sectors}
          selected={filters.sectors}
          onToggle={(sector) =>
            onChange({ ...filters, sectors: toggleValue(filters.sectors, sector) })
          }
        />

        <div>
          <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">
            Период
          </span>
          <div className="mt-1 grid grid-cols-2 gap-2">
            <label className="block">
              <span className="text-[10px] font-medium text-slate-400">С</span>
              <input
                type="date"
                value={filters.from}
                max={filters.to || undefined}
                onChange={(event) => onChange({ ...filters, from: event.target.value })}
                className="mt-1 w-full rounded-xl border border-slate-200 bg-white px-2 py-1.5 text-xs text-slate-700 outline-none transition focus:border-[#ef3124]"
              />
            </label>
            <label className="block">
              <span className="text-[10px] font-medium text-slate-400">По</span>
              <input
                type="date"
                value={filters.to}
                min={filters.from || undefined}
                onChange={(event) => onChange({ ...filters, to: event.target.value })}
                className="mt-1 w-full rounded-xl border border-slate-200 bg-white px-2 py-1.5 text-xs text-slate-700 outline-none transition focus:border-[#ef3124]"
              />
            </label>
          </div>
          <p className="mt-1 text-[10px] leading-4 text-slate-400">
            Пустое поле — без ограничения
          </p>
        </div>

        {invalidDateRange && (
          <p className="text-[11px] leading-4 text-red-600">
            Начало периода не может быть позже окончания.
          </p>
        )}

        <div>
          <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">
            Сортировка
          </span>
          <div className="mt-1 grid grid-cols-2 gap-1 rounded-xl bg-slate-100 p-1">
            <button
              type="button"
              onClick={() => onChange({ ...filters, sort: "relevance" })}
              className={`rounded-lg px-2 py-1.5 text-[11px] font-semibold transition ${
                filters.sort === "relevance"
                  ? "bg-white text-slate-900 shadow-sm"
                  : "text-slate-500 hover:text-slate-700"
              }`}
            >
              По релевантности
            </button>
            <button
              type="button"
              onClick={() => onChange({ ...filters, sort: "date" })}
              className={`rounded-lg px-2 py-1.5 text-[11px] font-semibold transition ${
                filters.sort === "date"
                  ? "bg-white text-slate-900 shadow-sm"
                  : "text-slate-500 hover:text-slate-700"
              }`}
            >
              Сначала новые
            </button>
          </div>
        </div>

        {hasActiveFeedFilters(filters) && (
          <button
            type="button"
            onClick={onReset}
            className="w-full rounded-xl border border-red-100 bg-red-50 px-3 py-2 text-xs font-semibold text-[#b42318] transition hover:bg-red-100"
          >
            Сбросить фильтры
          </button>
        )}
      </div>
    </div>
  );
}
