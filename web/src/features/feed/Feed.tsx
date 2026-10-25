import { keepPreviousData, useInfiniteQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { api, eventsUrl } from "../../api/client";
import type {
  ErrorEvent,
  FeedFacets,
  FeedItem,
  Insight,
  ResultEvent,
  StepEvent,
} from "../../api/types";
import { formatDate } from "../../shared/format";
import { useDebouncedValue } from "../../shared/useDebouncedValue";
import { AnalysisPanel } from "./AnalysisPanel";
import {
  hasActiveFeedFilters,
  hasInvalidFeedDateRange,
  toApiFeedFilters,
  type FeedFilterState,
} from "./filters";

const feedPageSize = 30;

type Props = {
  companyId: string;
  filters: FeedFilterState;
  onFacetsChange: (facets: FeedFacets | null) => void;
  onResetFilters: () => void;
  onToggleSource: (source: string) => void;
  onToggleTag: (tag: string) => void;
  onToggleSector: (sector: string) => void;
};

export function Feed({
  companyId,
  filters,
  onFacetsChange,
  onResetFilters,
  onToggleSource,
  onToggleTag,
  onToggleSector,
}: Props) {
  const sourceRef = useRef<EventSource | null>(null);
  const [activeEventId, setActiveEventId] = useState<string | null>(null);
  const [steps, setSteps] = useState<StepEvent[]>([]);
  const [insight, setInsight] = useState<Insight | null>(null);
  const [status, setStatus] = useState("");
  const [error, setError] = useState<string | null>(null);
  const debouncedQuery = useDebouncedValue(filters.query, 300);
  const apiFilters = toApiFeedFilters(filters, debouncedQuery);
  const dateRangeInvalid = hasInvalidFeedDateRange(filters);
  const hasFilters = hasActiveFeedFilters(filters);

  const feed = useInfiniteQuery({
    queryKey: ["feed", companyId, apiFilters],
    queryFn: ({ pageParam }) => api.feed(companyId, apiFilters, feedPageSize, pageParam),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.reduce((total, page) => total + page.items.length, 0);
      return loaded < lastPage.total ? loaded : undefined;
    },
    enabled: Boolean(companyId) && !dateRangeInvalid,
    staleTime: 30_000,
    placeholderData: keepPreviousData,
  });

  const items = feed.data?.pages.flatMap((page) => page.items) ?? [];
  const total = feed.data?.pages[0]?.total ?? 0;
  const facets = feed.data?.pages[0]?.facets ?? null;

  useEffect(() => {
    onFacetsChange(facets);
  }, [facets, onFacetsChange]);

  useEffect(() => {
    return () => sourceRef.current?.close();
  }, []);

  async function startAnalysis(item: FeedItem) {
    sourceRef.current?.close();
    sourceRef.current = null;

    setActiveEventId(item.event_id);
    setSteps([]);
    setInsight(null);
    setError(null);
    setStatus("Создаём задачу");

    let jobId: string;
    try {
      const job = await api.createJob(companyId, item);
      jobId = job.job_id;
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Не удалось создать задачу");
      return;
    }

    const source = new EventSource(eventsUrl(jobId));
    sourceRef.current = source;
    setStatus("Агент начал работу");

    source.onopen = () => setStatus("Получаем события");

    source.addEventListener("step", (event) => {
      const message = event as MessageEvent<string>;
      if (!message.data) return;
      const step = JSON.parse(message.data) as StepEvent;
      setSteps((previous) => [...previous, step]);
    });

    source.addEventListener("result", (event) => {
      const message = event as MessageEvent<string>;
      if (!message.data) return;
      const result = JSON.parse(message.data) as ResultEvent;
      setInsight(result.insight);
      setStatus("Готово");
      source.close();
      sourceRef.current = null;
    });

    source.addEventListener("error", (event) => {
      const message = event as MessageEvent<string>;
      if (!message.data) return;
      const failure = JSON.parse(message.data) as ErrorEvent;
      setError(failure.message);
      source.close();
      sourceRef.current = null;
    });

    source.onerror = () => {
      if (source.readyState === EventSource.CLOSED) {
        setError("Соединение с агентом прервано");
        sourceRef.current = null;
      }
    };
  }

  const activeItem = items.find((item) => item.event_id === activeEventId);
  const shown = items.length;
  const countLabel = total > shown ? `${shown} из ${total}` : `${total} событий`;

  return (
    <section className="mt-6 space-y-4">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h2 className="text-xl font-bold tracking-tight text-slate-950">Лента событий</h2>
          <p className="mt-1 text-sm text-slate-500">
            Влияние новости на компанию, позиция банка и вывод AI
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => feed.refetch()}
            disabled={feed.isFetching}
            className="rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-600 transition hover:bg-slate-50 disabled:cursor-wait disabled:opacity-60"
          >
            {feed.isFetching ? "Обновляем…" : "Обновить"}
          </button>
          <span className="rounded-full bg-white px-3 py-1 text-xs font-medium text-slate-500 ring-1 ring-slate-200">
            {countLabel}
          </span>
        </div>
      </div>

      {dateRangeInvalid && (
        <div className="rounded-2xl border border-red-200 bg-red-50 p-5 text-sm text-red-700">
          Начало периода не может быть позже окончания.
        </div>
      )}

      {feed.isLoading && !dateRangeInvalid && (
        <div className="space-y-3">
          <div className="h-32 animate-pulse rounded-2xl bg-white/70" />
          <div className="h-32 animate-pulse rounded-2xl bg-white/70" />
        </div>
      )}

      {feed.isError && !dateRangeInvalid && (
        <div className="rounded-2xl border border-red-200 bg-red-50 p-5 text-sm text-red-700">
          {feed.error instanceof Error
            ? feed.error.message
            : "Лента недоступна. Проверьте insight-service и content-service."}
        </div>
      )}

      {!dateRangeInvalid && !feed.isLoading && items.length === 0 && (
        <div className="rounded-2xl border border-dashed border-slate-300 bg-white p-8 text-center text-sm text-slate-500">
          {hasFilters ? (
            <>
              <p>По выбранным фильтрам ничего не найдено.</p>
              <button
                type="button"
                onClick={onResetFilters}
                className="mt-3 text-sm font-semibold text-[#b42318] underline decoration-red-200 underline-offset-4 hover:decoration-red-500"
              >
                Сбросить фильтры
              </button>
            </>
          ) : (
            "В content-сервисе пока нет новостей. Загрузите их через seed или API документов."
          )}
        </div>
      )}

      {!dateRangeInvalid &&
        items.map((item) => {
        const isActive = item.event_id === activeEventId;
        const isBusy = isActive && !insight && !error;
        return (
          <article
            key={item.event_id}
            className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm"
          >
            <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500">
              {item.source ? (
                <button
                  type="button"
                  onClick={() => onToggleSource(item.source)}
                  title="Фильтровать по источнику"
                  className={`rounded-full px-2.5 py-1 font-medium transition ${
                    filters.sources.includes(item.source)
                      ? "bg-slate-900 text-white"
                      : "bg-slate-100 text-slate-600 hover:bg-slate-200"
                  }`}
                >
                  {item.source}
                </button>
              ) : (
                <span className="rounded-full bg-slate-100 px-2.5 py-1 font-medium text-slate-500">
                  Без источника
                </span>
              )}
              <span>{formatDate(item.published_at)}</span>
              <span className="rounded-full bg-emerald-50 px-2.5 py-1 font-medium text-emerald-700">
                релевантность {Math.round(item.relevance * 100)}%
              </span>
              <span className="text-slate-300">·</span>
              <span className="font-mono text-[11px]">{item.event_id}</span>
            </div>

            <h3 className="mt-3 text-lg font-semibold leading-7 text-slate-950">{item.title}</h3>
            <p className="mt-2 text-sm leading-6 text-slate-600">{item.summary}</p>

            {item.reasons.length > 0 && (
              <div className="mt-3 flex flex-wrap gap-1.5">
                {item.reasons.map((reason) => (
                  <span
                    key={reason}
                    className="rounded-full bg-slate-50 px-2 py-1 text-[11px] text-slate-500 ring-1 ring-slate-200"
                  >
                    {reason}
                  </span>
                ))}
              </div>
            )}

            {(item.tags.length > 0 || item.target_sectors.length > 0) && (
              <div className="mt-2 flex flex-wrap gap-1.5">
                {item.tags.map((tag) => {
                  const active = filters.tags.includes(tag);
                  return (
                    <button
                      key={`tag-${tag}`}
                      type="button"
                      aria-pressed={active}
                      onClick={() => onToggleTag(tag)}
                      className={`rounded-full px-2 py-1 text-[11px] transition ${
                        active
                          ? "bg-slate-900 font-medium text-white"
                          : "bg-slate-50 text-slate-500 ring-1 ring-slate-200 hover:bg-slate-100"
                      }`}
                    >
                      #{tag}
                    </button>
                  );
                })}
                {item.target_sectors.map((sector) => {
                  const active = filters.sectors.includes(sector);
                  return (
                    <button
                      key={`sector-${sector}`}
                      type="button"
                      aria-pressed={active}
                      onClick={() => onToggleSector(sector)}
                      className={`rounded-full px-2 py-1 text-[11px] transition ${
                        active
                          ? "bg-sky-800 font-medium text-white"
                          : "bg-sky-50 text-sky-700 ring-1 ring-sky-100 hover:bg-sky-100"
                      }`}
                    >
                      {sector}
                    </button>
                  );
                })}
              </div>
            )}

            <div className="mt-4 flex items-center gap-3">
              <button
                type="button"
                onClick={() => startAnalysis(item)}
                disabled={isBusy}
                className="rounded-xl bg-[#ef3124] px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-[#d92b20] disabled:cursor-wait disabled:opacity-60"
              >
                {isBusy ? "Анализируем…" : "Разобрать событие"}
              </button>
              {item.url && (
                <a
                  href={item.url}
                  target="_blank"
                  rel="noreferrer"
                  className="text-sm font-medium text-slate-500 underline decoration-slate-300 underline-offset-4 hover:text-slate-800"
                >
                  Источник
                </a>
              )}
            </div>

            {isActive && (
              <div className="mt-5 border-t border-slate-100 pt-5">
                <AnalysisPanel steps={steps} insight={insight} status={status} error={error} />
              </div>
            )}
          </article>
        );
      })}

      {!dateRangeInvalid && feed.hasNextPage && (
        <div className="flex justify-center pt-2">
          <button
            type="button"
            onClick={() => feed.fetchNextPage()}
            disabled={feed.isFetchingNextPage}
            className="rounded-xl border border-slate-200 bg-white px-4 py-2.5 text-sm font-semibold text-slate-600 transition hover:bg-slate-50 disabled:cursor-wait disabled:opacity-60"
          >
            {feed.isFetchingNextPage ? "Загружаем…" : "Показать ещё"}
          </button>
        </div>
      )}

      {activeItem && activeItem.event_id === activeEventId && !insight && !error && (
        <p className="sr-only" aria-live="polite">
          {status}
        </p>
      )}
    </section>
  );
}
