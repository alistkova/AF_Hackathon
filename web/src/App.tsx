import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "./api/client";
import type { FeedFacets, User } from "./api/types";
import { Login } from "./features/auth/Login";
import { CompanyList } from "./features/company/CompanyList";
import { CompanyProfile } from "./features/company/CompanyProfile";
import { Feed } from "./features/feed/Feed";
import { FeedFilters } from "./features/feed/FeedFilters";
import { createDefaultFeedFilters, toggleValue, type FeedFilterState } from "./features/feed/filters";

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    api
      .me()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setChecking(false));
  }, []);

  if (checking) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-[#f5f7fa] text-sm text-slate-500">
        Проверяем сессию…
      </div>
    );
  }

  if (!user) {
    return <Login onSuccess={setUser} />;
  }

  return <Workspace user={user} onLogout={() => setUser(null)} />;
}

function Workspace({ user, onLogout }: { user: User; onLogout: () => void }) {
  const [companyId, setCompanyId] = useState("");
  const [filters, setFilters] = useState<FeedFilterState>(createDefaultFeedFilters);
  const [facets, setFacets] = useState<FeedFacets | null>(null);
  const companies = useQuery({
    queryKey: ["companies"],
    queryFn: api.companies,
  });

  useEffect(() => {
    if (!companyId && companies.data?.items.length) {
      setCompanyId(companies.data.items[0].id);
    }
  }, [companyId, companies.data]);

  function selectCompany(id: string) {
    setCompanyId(id);
    setFilters(createDefaultFeedFilters());
    setFacets(null);
  }

  function resetFilters() {
    setFilters(createDefaultFeedFilters());
  }

  function toggleSource(source: string) {
    setFilters((current) => ({ ...current, sources: toggleValue(current.sources, source) }));
  }

  function toggleTag(tag: string) {
    setFilters((current) => ({ ...current, tags: toggleValue(current.tags, tag) }));
  }

  function toggleSector(sector: string) {
    setFilters((current) => ({ ...current, sectors: toggleValue(current.sectors, sector) }));
  }

  async function logout() {
    await api.logout().catch(() => undefined);
    onLogout();
  }

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
            <div className="hidden text-right sm:block">
              <p className="text-xs font-semibold text-slate-700">{user.email}</p>
              <p className="text-[11px] text-slate-400">{user.role}</p>
            </div>
            <button
              type="button"
              onClick={logout}
              className="rounded-xl border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-600 transition hover:bg-slate-50"
            >
              Выйти
            </button>
          </div>
        </div>
      </header>

      <main className="mx-auto grid max-w-7xl gap-6 px-6 py-6 lg:grid-cols-[280px_minmax(0,1fr)]">
        <aside className="space-y-4">
          <CompanyList selectedId={companyId} onSelect={selectCompany} />
          <div className="rounded-2xl border border-slate-200 bg-white p-4 text-xs leading-5 text-slate-500 shadow-sm">
            <p className="font-semibold text-slate-700">Как это работает</p>
            <p className="mt-2">
              Лента строится из новостей content-сервиса и ранжируется по профилю компании.
              Разбор запускает агента отдельно, прогресс приходит по SSE.
            </p>
          </div>
          {companyId && (
            <FeedFilters
              filters={filters}
              facets={facets}
              onChange={setFilters}
              onReset={resetFilters}
            />
          )}
        </aside>

        <div className="min-w-0">
          {companyId ? (
            <>
              <CompanyProfile companyId={companyId} />
              <Feed
                key={companyId}
                companyId={companyId}
                filters={filters}
                onFacetsChange={setFacets}
                onResetFilters={resetFilters}
                onToggleSource={toggleSource}
                onToggleTag={toggleTag}
                onToggleSector={toggleSector}
              />
            </>
          ) : (
            <div className="rounded-2xl border border-dashed border-slate-300 bg-white p-8 text-center text-sm text-slate-500">
              {companies.isLoading
                ? "Загружаем компании…"
                : companies.isError
                  ? "Не удалось загрузить компании. Проверьте, что сервисы запущены."
                  : "Нет доступных компаний. Обратитесь к администратору за доступом."}
            </div>
          )}
          <footer className="py-8 text-center text-xs text-slate-400">
            Альфа Хак 2026 · gateway + company + content + insight + agent
          </footer>
        </div>
      </main>
    </div>
  );
}
