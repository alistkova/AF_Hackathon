import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { api, eventsUrl } from "../../api/client";
import type { ErrorEvent, FeedItem, Insight, ResultEvent, StepEvent } from "../../api/types";
import { formatDate } from "../../shared/format";
import { AnalysisPanel } from "./AnalysisPanel";

type Props = {
  companyId: string;
};

export function Feed({ companyId }: Props) {
  const sourceRef = useRef<EventSource | null>(null);
  const [activeEventId, setActiveEventId] = useState<string | null>(null);
  const [steps, setSteps] = useState<StepEvent[]>([]);
  const [insight, setInsight] = useState<Insight | null>(null);
  const [status, setStatus] = useState("");
  const [error, setError] = useState<string | null>(null);

  const feed = useQuery({
    queryKey: ["feed", companyId],
    queryFn: () => api.feed(companyId),
    enabled: Boolean(companyId),
  });

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

  return (
    <section className="mt-6 space-y-4">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h2 className="text-xl font-bold tracking-tight text-slate-950">Лента событий</h2>
          <p className="mt-1 text-sm text-slate-500">
            Влияние новости на компанию, позиция банка и вывод AI
          </p>
        </div>
        <span className="rounded-full bg-white px-3 py-1 text-xs font-medium text-slate-500 ring-1 ring-slate-200">
          {feed.data?.items.length ?? 0} событий
        </span>
      </div>

      {feed.isLoading && (
        <div className="space-y-3">
          <div className="h-32 animate-pulse rounded-2xl bg-white/70" />
          <div className="h-32 animate-pulse rounded-2xl bg-white/70" />
        </div>
      )}

      {feed.isError && (
        <div className="rounded-2xl border border-red-200 bg-red-50 p-5 text-sm text-red-700">
          Лента недоступна. Проверьте insight-service.
        </div>
      )}

      {feed.data?.items.length === 0 && (
        <div className="rounded-2xl border border-dashed border-slate-300 bg-white p-8 text-center text-sm text-slate-500">
          Для этой компании пока нет событий.
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
              <span className="text-slate-300">·</span>
              <span className="font-mono text-[11px]">{item.event_id}</span>
            </div>

            <h3 className="mt-3 text-lg font-semibold leading-7 text-slate-950">{item.title}</h3>
            <p className="mt-2 text-sm leading-6 text-slate-600">{item.summary}</p>

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
