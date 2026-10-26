import type { ReactNode } from "react";
import type { CitedClaim, Insight, StepEvent } from "../../api/types";

type Props = {
  steps: StepEvent[];
  insight: Insight | null;
  status: string;
  error: string | null;
};

const stepLabels: Record<string, string> = {
  planning: "Планирование",
  enriching: "Догрузка контекста",
  retrieving: "Поиск источников",
  ranking: "Оценка влияния",
  analyzing: "Анализ AI",
  validating: "Проверка источников",
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

function Citations({ ids, sourceIndex }: { ids: string[]; sourceIndex: Map<string, number> }) {
  if (ids.length === 0) {
    return null;
  }
  return (
    <sup className="ml-1 inline-flex gap-1">
      {ids.map((id) => {
        const index = sourceIndex.get(id);
        return (
          <span
            key={id}
            title={id}
            className="rounded bg-slate-100 px-1 py-0.5 font-mono text-[10px] text-slate-500"
          >
            {index ?? "?"}
          </span>
        );
      })}
    </sup>
  );
}

function ClaimList({
  title,
  claims,
  sourceIndex,
  muted = false,
}: {
  title: string;
  claims: CitedClaim[] | undefined;
  sourceIndex: Map<string, number>;
  muted?: boolean;
}) {
  if (!claims || claims.length === 0) {
    return null;
  }
  return (
    <Section title={title}>
      <ul className={`space-y-1.5 ${muted ? "text-slate-500" : ""}`}>
        {claims.map((claim, index) => (
          <li key={`${title}-${index}-${claim.text}`}>
            {claim.text}
            <Citations ids={claim.source_ids} sourceIndex={sourceIndex} />
          </li>
        ))}
      </ul>
    </Section>
  );
}

function StringList({ title, items }: { title: string; items: string[] }) {
  if (items.length === 0) {
    return null;
  }
  return (
    <Section title={title}>
      <ul className="list-disc space-y-1 pl-4">
        {items.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </Section>
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

  const sourceIndex = new Map(
    insight.sources.map((source, index) => [source.source_id, index + 1] as const),
  );
  const risks: CitedClaim[] =
    insight.structured_risks && insight.structured_risks.length > 0
      ? insight.structured_risks
      : insight.risks.map((text) => ({ text, source_ids: [] }));
  const opportunities: CitedClaim[] =
    insight.structured_opportunities && insight.structured_opportunities.length > 0
      ? insight.structured_opportunities
      : insight.opportunities.map((text) => ({ text, source_ids: [] }));
  const bankView = insight.bank_view;
  const bankText = bankView?.used ? bankView.text : insight.bank_position;
  const aiText = insight.ai_view?.text?.trim() ? insight.ai_view.text : insight.ai_conclusion;
  const disclaimer = insight.disclaimer?.trim() || "Не является инвестиционной рекомендацией";

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

      <ClaimList title="Факты" claims={insight.facts} sourceIndex={sourceIndex} />
      <ClaimList title="Гипотезы" claims={insight.hypotheses} sourceIndex={sourceIndex} muted />

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl bg-amber-50 p-4">
          <ClaimList title="Риски" claims={risks} sourceIndex={sourceIndex} />
        </div>
        <div className="rounded-xl bg-emerald-50 p-4">
          <ClaimList title="Возможности" claims={opportunities} sourceIndex={sourceIndex} />
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        {bankText.trim() !== "" && (
          <div className="rounded-xl border border-slate-200 p-4">
            <Section title="Позиция банка">{bankText}</Section>
          </div>
        )}
        {aiText.trim() !== "" && (
          <div className="rounded-xl border border-slate-200 p-4">
            <Section title="Вывод AI">{aiText}</Section>
          </div>
        )}
      </div>

      <StringList title="Действия" items={insight.actions} />

      <Section title="Источники">
        <ol className="space-y-2">
          {insight.sources.map((source, index) => (
            <li key={source.source_id} className="flex gap-2">
              <span className="font-mono text-[10px] text-slate-400">{index + 1}</span>
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
        </ol>
      </Section>

      <p className="border-t border-slate-100 pt-3 text-[11px] leading-5 text-slate-400">
        {disclaimer}
      </p>
    </div>
  );
}
