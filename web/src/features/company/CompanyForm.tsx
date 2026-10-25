import { useState } from "react";
import type { CompanyFormValues } from "./form";

type Props = {
  initial: CompanyFormValues;
  submitLabel: string;
  pending: boolean;
  error: string | null;
  onSubmit: (values: CompanyFormValues) => void;
  cancelHref: string;
};

export function CompanyForm({
  initial,
  submitLabel,
  pending,
  error,
  onSubmit,
  cancelHref,
}: Props) {
  const [values, setValues] = useState(initial);

  function update(field: keyof CompanyFormValues, value: string) {
    setValues((current) => ({ ...current, [field]: value }));
  }

  function submit(event: React.FormEvent) {
    event.preventDefault();
    onSubmit(values);
  }

  return (
    <form onSubmit={submit} className="space-y-6">
      <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
        <h2 className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
          Основные данные
        </h2>
        <div className="mt-4 grid gap-4 sm:grid-cols-2">
          <label className="block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500 sm:col-span-2">
            Название *
            <input
              required
              value={values.name}
              onChange={(event) => update("name", event.target.value)}
              placeholder="ООО «Ромашка»"
              className="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm font-normal normal-case tracking-normal text-slate-900 outline-none focus:border-[#ef3124]"
            />
          </label>
          <label className="block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
            ИНН
            <input
              value={values.inn}
              onChange={(event) => update("inn", event.target.value)}
              inputMode="numeric"
              placeholder="10 или 12 цифр"
              className="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm font-normal normal-case tracking-normal text-slate-900 outline-none focus:border-[#ef3124]"
            />
          </label>
          <label className="block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
            Отрасль
            <input
              value={values.industry}
              onChange={(event) => update("industry", event.target.value)}
              placeholder="Ритейл"
              className="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm font-normal normal-case tracking-normal text-slate-900 outline-none focus:border-[#ef3124]"
            />
          </label>
          <label className="block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
            Регион
            <input
              value={values.region}
              onChange={(event) => update("region", event.target.value)}
              placeholder="Москва"
              className="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm font-normal normal-case tracking-normal text-slate-900 outline-none focus:border-[#ef3124]"
            />
          </label>
          <label className="block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
            Город
            <input
              value={values.city}
              onChange={(event) => update("city", event.target.value)}
              placeholder="Москва"
              className="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm font-normal normal-case tracking-normal text-slate-900 outline-none focus:border-[#ef3124]"
            />
          </label>
          <label className="block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
            Выручка, ₽
            <input
              value={values.revenue}
              onChange={(event) => update("revenue", event.target.value)}
              inputMode="numeric"
              placeholder="1000000"
              className="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm font-normal normal-case tracking-normal text-slate-900 outline-none focus:border-[#ef3124]"
            />
          </label>
          <label className="block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500 sm:col-span-2">
            Описание
            <textarea
              value={values.description}
              onChange={(event) => update("description", event.target.value)}
              rows={3}
              placeholder="Чем занимается компания"
              className="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm font-normal normal-case tracking-normal text-slate-900 outline-none focus:border-[#ef3124]"
            />
          </label>
        </div>
      </section>

      {error && (
        <p className="rounded-xl bg-red-50 p-3 text-sm text-red-700" role="alert">
          {error}
        </p>
      )}

      <div className="flex flex-wrap items-center gap-2">
        <button
          type="submit"
          disabled={pending}
          className="rounded-xl bg-[#ef3124] px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-[#d92b20] disabled:cursor-wait disabled:opacity-60"
        >
          {pending ? "Сохраняем…" : submitLabel}
        </button>
        <a
          href={cancelHref}
          className="rounded-xl border border-slate-200 px-3 py-2.5 text-sm font-semibold text-slate-600 transition hover:bg-slate-50"
        >
          Отмена
        </a>
      </div>
    </form>
  );
}
