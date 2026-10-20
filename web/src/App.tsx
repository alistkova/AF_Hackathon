import { useState } from "react";
import { CompanyList } from "./features/company/CompanyList";
import { CompanyProfile } from "./features/company/CompanyProfile";
import { Feed } from "./features/feed/Feed";

export default function App() {
  const [companyId, setCompanyId] = useState("magnit");

  return (
    <div className="min-h-screen bg-[#f5f7fa] text-slate-900">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-6 py-4">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-[#ef3124] text-lg font-black text-white">
              А
            </div>
            <div>
              <p className="text-sm font-bold tracking-tight text-slate-950">
                Альфа Хак · Новостная аналитика
              </p>
              <p className="text-xs text-slate-500">Влияние событий на компании</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <span className="hidden rounded-full bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-700 sm:inline">
              демо-режим
            </span>
            <div className="flex h-9 w-9 items-center justify-center rounded-full bg-slate-100 text-xs font-semibold text-slate-600">
              МК
            </div>
          </div>
        </div>
      </header>

      <main className="mx-auto grid max-w-7xl gap-6 px-6 py-6 lg:grid-cols-[280px_minmax(0,1fr)]">
        <aside className="space-y-4">
          <CompanyList selectedId={companyId} onSelect={setCompanyId} />
          <div className="rounded-2xl border border-slate-200 bg-white p-4 text-xs leading-5 text-slate-500 shadow-sm">
            <p className="font-semibold text-slate-700">Как это работает</p>
            <p className="mt-2">
              Лента показывает события из кэша. Разбор запускает агента отдельно, прогресс приходит
              по SSE.
            </p>
          </div>
        </aside>

        <div className="min-w-0">
          <CompanyProfile companyId={companyId} />
          <Feed companyId={companyId} />
          <footer className="py-8 text-center text-xs text-slate-400">
            Альфа Хак 2026 · gateway + company + content + insight + agent
          </footer>
        </div>
      </main>
    </div>
  );
}
