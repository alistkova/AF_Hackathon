import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "./api/client";
import type { Company, FeedFacets, User } from "./api/types";
import { Login } from "./features/auth/Login";
import { CompanyCreatePage } from "./features/company/CompanyCreatePage";
import { CompanyEditPage } from "./features/company/CompanyEditPage";
import { CompanyList } from "./features/company/CompanyList";
import { CompanyMembersPage } from "./features/company/CompanyMembersPage";
import { CompanyProfile } from "./features/company/CompanyProfile";
import { Feed } from "./features/feed/Feed";
import { FeedFilters } from "./features/feed/FeedFilters";
import { createDefaultFeedFilters, toggleValue, type FeedFilterState } from "./features/feed/filters";
import { ProfileEditPage } from "./features/profile/ProfileEditPage";
import { ProfilePage } from "./features/profile/ProfilePage";
import { parseRoute, routeHref, type Route } from "./shared/hash";

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

  return <Workspace user={user} onUserChange={setUser} onLogout={() => setUser(null)} />;
}

type WorkspaceProps = {
  user: User;
  onUserChange: (user: User) => void;
  onLogout: () => void;
};

function Workspace({ user, onUserChange, onLogout }: WorkspaceProps) {
  const [route, setRoute] = useState<Route>(() => parseRoute(window.location.hash));
  const [companyId, setCompanyId] = useState("");
  const [filters, setFilters] = useState<FeedFilterState>(createDefaultFeedFilters);
  const [facets, setFacets] = useState<FeedFacets | null>(null);
  const companies = useQuery({
    queryKey: ["companies"],
    queryFn: api.companies,
  });

  useEffect(() => {
    const sync = () => setRoute(parseRoute(window.location.hash));
    window.addEventListener("hashchange", sync);
    return () => window.removeEventListener("hashchange", sync);
  }, []);

  useEffect(() => {
    const items = companies.data?.items;
    if (!items) return;
    if (items.length === 0) {
      if (companyId) setCompanyId("");
      return;
    }
    if (!companyId || !items.some((item) => item.id === companyId)) {
      setCompanyId(items[0].id);
      setFilters(createDefaultFeedFilters());
      setFacets(null);
    }
  }, [companies.data, companyId]);

  function selectCompany(id: string) {
    setCompanyId(id);
    setFilters(createDefaultFeedFilters());
    setFacets(null);
  }

  function openCompany(id: string) {
    selectCompany(id);
    window.location.hash = routeHref({ name: "feed" });
  }

  function resetFilters() {
    setFilters(createDefaultFeedFilters());
  }

  function setQuery(query: string) {
    setFilters((current) => (current.query === query ? current : { ...current, query }));
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

  async function handleCompanyCreated(company: Company) {
    if (!user.primary_company_id) {
      const updated = await api.updateProfile({ primary_company_id: company.id }).catch(() => null);
      if (updated) onUserChange(updated);
    }
    selectCompany(company.id);
    window.location.hash = routeHref({ name: "company-members", companyId: company.id });
  }

  async function logout() {
    await api.logout().catch(() => undefined);
    window.location.hash = routeHref({ name: "feed" });
    onLogout();
  }

  return (
    <div className="min-h-screen bg-[#f5f7fa] text-slate-900">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4 px-6 py-4">
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
          <div className="flex w-full min-w-0 items-center justify-between gap-3 sm:ml-auto sm:w-auto sm:justify-end">
            <span className="hidden shrink-0 rounded-full bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-700 md:inline">
              демо-режим
            </span>
            <a
              href={routeHref({ name: "profile" })}
              title="Открыть профиль"
              className="min-w-0 max-w-[65%] text-left transition hover:opacity-70 sm:max-w-52 sm:text-right"
            >
              <span className="block truncate text-xs font-semibold text-slate-700">{user.email}</span>
              <span className="block text-[11px] text-slate-400">
                {user.role === "admin" ? "администратор" : "аналитик"}
              </span>
            </a>
            <button
              type="button"
              onClick={logout}
              className="shrink-0 rounded-xl border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-600 transition hover:bg-slate-50"
            >
              Выйти
            </button>
          </div>
        </div>
      </header>

      {route.name === "feed" ? (
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
                  onQueryChange={setQuery}
                  onFacetsChange={setFacets}
                  onResetFilters={resetFilters}
                  onToggleSource={toggleSource}
                  onToggleTag={toggleTag}
                  onToggleSector={toggleSector}
                />
              </>
            ) : (
              <div className="rounded-2xl border border-dashed border-slate-300 bg-white p-8 text-center text-sm text-slate-500">
                {companies.isLoading ? (
                  "Загружаем компании…"
                ) : companies.isError ? (
                  "Не удалось загрузить компании. Проверьте, что сервисы запущены."
                ) : (
                  <>
                    Нет доступных компаний.{" "}
                    <a
                      href={routeHref({ name: "company-new" })}
                      className="font-semibold text-[#b42318] hover:underline"
                    >
                      Создайте компанию
                    </a>{" "}
                    или попросите владельца добавить вас.
                  </>
                )}
              </div>
            )}
            <footer className="py-8 text-center text-xs text-slate-400">
              Альфа Хак 2026 · gateway + company + content + insight + agent
            </footer>
          </div>
        </main>
      ) : (
        <main className="mx-auto max-w-5xl px-6 py-6">
          {route.name === "profile" && (
            <ProfilePage user={user} onLogout={logout} onOpenCompany={openCompany} />
          )}
          {route.name === "profile-edit" && (
            <ProfileEditPage user={user} onUserChange={onUserChange} />
          )}
          {route.name === "company-new" && <CompanyCreatePage onCreated={handleCompanyCreated} />}
          {route.name === "company-edit" && (
            <CompanyEditPage companyId={route.companyId} user={user} />
          )}
          {route.name === "company-members" && (
            <CompanyMembersPage
              companyId={route.companyId}
              user={user}
              onOpenCompany={openCompany}
            />
          )}
          <footer className="py-8 text-center text-xs text-slate-400">
            Альфа Хак 2026 · gateway + company + content + insight + agent
          </footer>
        </main>
      )}
    </div>
  );
}
