import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useDeferredValue, useEffect, useRef, useState } from "react";
import { api, eventsUrl } from "../../api/client";
import type {
  ErrorEvent,
  FeedFacet,
  FeedItem,
  Insight,
  ResultEvent,
  StepEvent,
} from "../../api/types";
import { formatDate } from "../../shared/format";
import { AnalysisPanel } from "./AnalysisPanel";

const periodOptions = [
  { value: "all", label: "Всё время", days: 0 },
  { value: "90", label: "90 дней", days: 90 },
  { value: "30", label: "30 дней", days: 30 },
  { value: "7", label: "7 дней", days: 7 },
];

type Props = {
  companyId: string;
};

type FacetSelectProps = {
  label: string;
  allLabel: string;
  value: string;
  facets: FeedFacet[];
  onChange: (value: string) => void;
};

function FacetSelect({ label, allLabel, value, facets, onChange }: FacetSelectProps) {
  const hasValue = value !== "" && !facets.some((facet) => facet.value === value);

  return (
    <label className="block">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">
        {label}
      </span>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 outline-none transition focus:border-[#ef3124]"
      >
        <option value="">{allLabel}</option>
        {hasValue && <option value={value}>{value}</option>}
        {facets.map((facet) => (
          <option key={facet.value} value={facet.value}>
            {facet.value} ({facet.count})
          </option>
        ))}
      </select>
    </label>
  );
}

function periodStart(period: string): string | undefined {
  const option = periodOptions.find((item) => item.value === period);
  if (!option || option.days === 0) {
    return undefined;
  }

  const date = new Date();
  date.setUTCDate(date.getUTCDate() - option.days);
  return date.toISOString().slice(0, 10);
}

export function Feed({ companyId }: Props) {
  const sourceRef = useRef<EventSource | null>(null);
  const [activeEventId, setActiveEventId] = useState<string | null>(null);
  const [steps, setSteps] = useState<StepEvent[]>([]);
  const [insight, setInsight] = useState<Insight | null>(null);
  const [status, setStatus] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [source, setSource] = useState("");
  const [tag, setTag] = useState("");
  const [period, setPeriod] = useState("all");
  const deferredQuery = useDeferredValue(query);
  const from = periodStart(period);
  const hasFilters = Boolean(source || tag || period !== "all" || query.trim());

  const feed = useQuery({
    queryKey: ["feed", companyId, source, tag, deferredQuery, from],
    queryFn: () => api.feed(companyId, { source, tag, query: deferredQuery, from }),
    enabled: Boolean(companyId),
    staleTime: 30_000,
    placeholderData: keepPreviousData,
  });

  function resetFilters() {
    setQuery("");
    setSource("");
    setTag("");
    setPeriod("all");
  }

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

  const activeItem = feed.data?.items.find((item) => item.event_id === activeEventId);
  const sources = feed.data?.facets.sources ?? [];
  const tags = feed.data?.facets.tags ?? [];
  const shown = feed.data?.items.length ?? 0;
  const total = feed.data?.total ?? 0;
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

      <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <label className="block">
            <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">
              Поиск
            </span>
            <input
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Заголовок, текст или тег"
              className="mt-1 w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 outline-none transition placeholder:text-slate-400 focus:border-[#ef3124]"
            />
          </label>

          <FacetSelect
            label="Источник"
            allLabel="Все источники"
            value={source}
            facets={sources}
            onChange={setSource}
          />

          <FacetSelect
            label="Тема"
            allLabel="Все темы"
            value={tag}
            facets={tags}
            onChange={setTag}
          />

          <label className="block">
            <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">
              Период
            </span>
            <select
              value={period}
              onChange={(event) => setPeriod(event.target.value)}
              className="mt-1 w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 outline-none transition focus:border-[#ef3124]"
            >
              {periodOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </label>
        </div>

        <div className="mt-3 flex min-h-6 items-center justify-between gap-3">
          <p className="text-xs text-slate-400">
            {hasFilters ? "Фильтры применяются ко всей ленте" : "Показаны все события"}
          </p>
          {hasFilters && (
            <button
              type="button"
              onClick={resetFilters}
              className="text-xs font-semibold text-[#b42318] underline decoration-red-200 underline-offset-4 hover:decoration-red-500"
            >
              Сбросить фильтры
            </button>
          )}
        </div>
      </div>

      {feed.isLoading && (
        <div className="space-y-3">
          <div className="h-32 animate-pulse rounded-2xl bg-white/70" />
          <div className="h-32 animate-pulse rounded-2xl bg-white/70" />
        </div>
      )}

      {feed.isError && (
        <div className="rounded-2xl border border-red-200 bg-red-50 p-5 text-sm text-red-700">
          Лента недоступна. Проверьте insight-service и content-service.
        </div>
      )}

      {feed.data?.items.length === 0 && (
        <div className="rounded-2xl border border-dashed border-slate-300 bg-white p-8 text-center text-sm text-slate-500">
          {hasFilters ? (
            <>
              <p>По выбранным фильтрам ничего не найдено.</p>
              <button
                type="button"
                onClick={resetFilters}
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

      {feed.data?.items.map((item) => {
        const isActive = item.event_id === activeEventId;
        const isBusy = isActive && !insight && !error;
        return (
          <article
            key={item.event_id}
            className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm"
          >
            <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500">
              <span className="rounded-full bg-slate-100 px-2.5 py-1 font-medium text-slate-600">
                {item.source}
              </span>
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

            <div className="mt-4 flex items-center gap-3">
              <button
                type="button"
                onClick={() => startAnalysis(item)}
                disabled={isBusy}
                className="rounded-xl bg-[#ef3124] px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-[#d92b20] disabled:cursor-wait disabled:opacity-60"
              >
                {isBusy ? "Анализируем…" : "Разобрать событие"}
              </button>
              <a
                href={item.url}
                target="_blank"
                rel="noreferrer"
                className="text-sm font-medium text-slate-500 underline decoration-slate-300 underline-offset-4 hover:text-slate-800"
              >
                Источник
              </a>
            </div>

            {isActive && (
              <div className="mt-5 border-t border-slate-100 pt-5">
                <AnalysisPanel steps={steps} insight={insight} status={status} error={error} />
              </div>
            )}
          </article>
        );
      })}

      {activeItem && activeItem.event_id === activeEventId && !insight && !error && (
        <p className="sr-only" aria-live="polite">
          {status}
        </p>
      )}
    </section>
  );
}
