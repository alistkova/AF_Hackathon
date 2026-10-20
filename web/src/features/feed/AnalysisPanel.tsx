import type { ReactNode } from "react";
import type { Insight, StepEvent } from "../../api/types";

type Props = {
  steps: StepEvent[];
  insight: Insight | null;
  status: string;
  error: string | null;
};

const stepLabels: Record<string, string> = {
  retrieving: "Поиск источников",
  ranking: "Оценка влияния",
  analyzing: "Анализ AI",
};

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div>
      <h4 className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">
        {title}
      </h4>
      <div className="mt-1.5 text-sm leading-6 text-slate-700">{children}</div>
    </div>
  );
}

export function AnalysisPanel({ steps, insight, status, error }: Props) {
  if (error) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">
        {error}
      </div>
    );
  }

  if (!insight) {
    return (
      <div className="rounded-xl border border-slate-200 bg-slate-50 p-4">
        <div className="flex items-center gap-2 text-sm font-medium text-slate-700">
          <span className="h-2 w-2 animate-pulse rounded-full bg-[#ef3124]" />
          {status}
        </div>
        <ol className="mt-3 space-y-2">
          {steps.map((item, index) => (
            <li key={`${item.seq}-${item.status}`} className="flex items-start gap-2 text-sm">
              <span
                className={`mt-1 h-2 w-2 shrink-0 rounded-full ${
                  item.status === "done" ? "bg-emerald-500" : "bg-amber-400"
                }`}
              />
              <span className="text-slate-600">
                <span className="font-medium text-slate-800">
                  {stepLabels[item.name] ?? item.name}
                </span>
                <span className="text-slate-400"> · {item.message}</span>
              </span>
              <span className="ml-auto font-mono text-[10px] text-slate-400">#{index + 1}</span>
            </li>
          ))}
        </ol>
      </div>
    );
  }

  return (
    <div className="space-y-5 rounded-xl border border-slate-200 bg-white p-5">
      <div className="flex items-center justify-between gap-4">
        <h3 className="text-lg font-semibold text-slate-950">Разбор события</h3>
        <span className="rounded-full bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700">
          Уверенность {Math.round(insight.confidence * 100)}%
        </span>
      </div>

      <Section title="Что случилось">{insight.summary}</Section>
      <Section title="Почему это важно">{insight.why_it_matters}</Section>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl bg-amber-50 p-4">
          <Section title="Риски">
            <ul className="list-disc space-y-1 pl-4">
              {insight.risks.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </Section>
        </div>
        <div className="rounded-xl bg-emerald-50 p-4">
          <Section title="Возможности">
            <ul className="list-disc space-y-1 pl-4">
              {insight.opportunities.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </Section>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-200 p-4">
          <Section title="Позиция банка">{insight.bank_position}</Section>
        </div>
        <div className="rounded-xl border border-slate-200 p-4">
          <Section title="Вывод AI">{insight.ai_conclusion}</Section>
        </div>
      </div>

      <Section title="Действия">
        <ul className="list-disc space-y-1 pl-4">
          {insight.actions.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </Section>

      <Section title="Источники">
        <ul className="space-y-2">
          {insight.sources.map((source) => (
            <li key={source.source_id}>
              <a
                href={source.url}
                target="_blank"
                rel="noreferrer"
                className="text-sm font-medium text-[#b42318] underline decoration-red-200 underline-offset-4 hover:decoration-red-500"
              >
                {source.title}
              </a>
            </li>
          ))}
        </ul>
      </Section>
    </div>
  );
}
