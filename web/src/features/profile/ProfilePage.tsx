import { useQuery } from "@tanstack/react-query";
import { api } from "../../api/client";
import type { User } from "../../api/types";
import { formatRevenue } from "../../shared/format";
import { routeHref } from "../../shared/hash";
import { activeAttributes, activeProducts, roleLabels } from "./companyContext";

type Props = {
  user: User;
  onLogout: () => void;
  onOpenCompany: (companyId: string) => void;
};

export function ProfilePage({ user, onLogout, onOpenCompany }: Props) {
  const companies = useQuery({
    queryKey: ["companies"],
    queryFn: api.companies,
  });
  const memberships = useQuery({
    queryKey: ["memberships"],
    queryFn: api.myMemberships,
  });
  const company = useQuery({
    queryKey: ["company", user.primary_company_id],
    queryFn: () => api.company(user.primary_company_id),
    enabled: Boolean(user.primary_company_id),
    retry: false,
  });

  const roleById = new Map(
    (memberships.data?.items ?? []).map((item) => [item.company_id, item.role]),
  );
  const items = companies.data?.items ?? [];
  const primary = company.data;
  const canEditPrimary =
    Boolean(primary) && (user.role === "admin" || roleById.get(primary?.id ?? "") === "owner");
  const products = activeProducts(primary?.products);
  const attributes = activeAttributes(primary?.attributes);
  const facts = primary
    ? [
        ["Город", primary.city],
        ["Тип клиента", primary.client_type],
        ["Выручка", formatRevenue(primary.revenue)],
        ["ИНН", primary.inn],
        ["ОКВЭД", primary.okved_code ? `${primary.okved_code} — ${primary.okved_name ?? ""}`.trim() : ""],
        ["Группа", primary.group_id],
      ].filter(([, value]) => Boolean(value))
    : [];

  return (
    <div className="space-y-6">
      <a
        href={routeHref({ name: "feed" })}
        className="inline-flex items-center gap-1 text-xs font-semibold text-[#b42318] transition hover:underline"
      >
        ← Назад в ленту
      </a>

      <section className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <div className="h-1.5 bg-[#ef3124]" />
        <div className="flex flex-col gap-5 p-6 md:flex-row md:items-start md:justify-between">
          <div className="flex items-start gap-4">
            <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-[#ef3124] text-xl font-black uppercase text-white">
              {user.email.slice(0, 1)}
            </div>
            <div className="min-w-0">
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[#ef3124]">
                Профиль
              </p>
              <h1 className="mt-1 break-all text-lg font-bold tracking-tight text-slate-950 sm:text-2xl">
                {user.email}
              </h1>
              <div className="mt-2 flex min-w-0 flex-wrap items-center gap-2 text-xs">
                <span className="rounded-full bg-emerald-50 px-2.5 py-1 font-medium text-emerald-700">
                  {roleLabels[user.role] ?? user.role}
                </span>
                <span className="min-w-0 text-slate-400">
                  ID: <span className="break-all font-mono">{user.id}</span>
                </span>
              </div>
            </div>
          </div>
          <div className="flex shrink-0 gap-2">
            <a
              href={routeHref({ name: "profile-edit" })}
              className="rounded-xl bg-[#ef3124] px-4 py-2 text-xs font-semibold text-white transition hover:bg-[#d92b20]"
            >
              Изменить
            </a>
            <button
              type="button"
              onClick={onLogout}
              className="rounded-xl border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-600 transition hover:bg-slate-50"
            >
              Выйти
            </button>
          </div>
        </div>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
              Моя компания
            </h2>
            {primary ? (
              <>
                <p className="mt-1 text-xl font-bold text-slate-950">{primary.name}</p>
                <p className="text-xs text-slate-500">
                  {[primary.industry, primary.region].filter(Boolean).join(" · ")}
                </p>
              </>
            ) : (
              <p className="mt-2 max-w-lg text-sm text-slate-500">
                {company.isLoading
                  ? "Загружаем данные компании…"
                  : company.isError
                    ? "Данные компании недоступны."
                    : "Компания не выбрана. Укажите её на странице редактирования профиля."}
              </p>
            )}
          </div>
          {primary && (
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => onOpenCompany(primary.id)}
                className="rounded-xl border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-600 transition hover:bg-slate-50"
              >
                Открыть в ленте
              </button>
              {canEditPrimary && (
                <a
                  href={routeHref({ name: "company-edit", companyId: primary.id })}
                  className="rounded-xl bg-[#ef3124] px-3 py-2 text-xs font-semibold text-white transition hover:bg-[#d92b20]"
                >
                  Изменить
                </a>
              )}
              <a
                href={routeHref({ name: "profile-edit" })}
                className="rounded-xl border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-600 transition hover:bg-slate-50"
              >
                Сменить основную
              </a>
            </div>
          )}
        </div>

        {company.isError && (
          <p className="mt-4 rounded-xl bg-amber-50 p-3 text-sm text-amber-700">
            Компания недоступна. Выберите другую на странице редактирования профиля.
          </p>
        )}

        {primary && (
          <>
            {primary.description && (
              <p className="mt-4 text-sm leading-6 text-slate-600">{primary.description}</p>
            )}
            {facts.length > 0 && (
              <dl className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {facts.map(([label, value]) => (
                  <div key={label} className="rounded-xl bg-slate-50 p-3">
                    <dt className="text-xs text-slate-500">{label}</dt>
                    <dd className="mt-1 break-words text-sm font-semibold text-slate-800">{value}</dd>
                  </div>
                ))}
              </dl>
            )}
            {products.length > 0 && (
              <div className="mt-5">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
                  Продукты банка
                </p>
                <div className="mt-2 flex flex-wrap gap-2">
                  {products.map((product) => (
                    <span
                      key={product}
                      className="rounded-full bg-[#fef2f1] px-3 py-1 text-xs font-medium text-[#b42318]"
                    >
                      {product}
                    </span>
                  ))}
                </div>
              </div>
            )}
            {attributes.length > 0 && (
              <div className="mt-4">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
                  Признаки компании
                </p>
                <div className="mt-2 flex flex-wrap gap-2">
                  {attributes.map(([label, value]) => (
                    <span
                      key={label}
                      className="rounded-full bg-slate-100 px-3 py-1 text-xs text-slate-600"
                    >
                      {label}: <span className="font-semibold text-slate-800">{value}</span>
                    </span>
                  ))}
                </div>
              </div>
            )}
          </>
        )}
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
              Мои компании
            </h2>
            <p className="mt-2 max-w-xl text-xs leading-5 text-slate-500">
              Вы видите только компании, в которых состоите. Владелец компании может добавлять
              и удалять участников.
            </p>
          </div>
          <a
            href={routeHref({ name: "company-new" })}
            className="rounded-xl bg-[#ef3124] px-4 py-2 text-xs font-semibold text-white transition hover:bg-[#d92b20]"
          >
            Добавить компанию
          </a>
        </div>

        {(companies.isLoading || memberships.isLoading) && (
          <div className="mt-4 h-24 animate-pulse rounded-xl bg-slate-100" />
        )}

        {(companies.isError || memberships.isError) && (
          <p className="mt-4 rounded-xl bg-red-50 p-3 text-sm text-red-700">
            Не удалось загрузить компании. Проверьте, что сервисы запущены.
          </p>
        )}

        {!companies.isLoading && !companies.isError && !memberships.isError && (
          <>
            {items.length === 0 ? (
              <div className="mt-4 rounded-xl border border-dashed border-slate-300 p-6 text-center text-sm text-slate-500">
                Вы пока не добавлены ни в одну компанию.{" "}
                <a
                  href={routeHref({ name: "company-new" })}
                  className="font-semibold text-[#b42318] hover:underline"
                >
                  Создайте компанию
                </a>{" "}
                или попросите владельца добавить вас.
              </div>
            ) : (
              <div className="mt-4 space-y-2">
                {items.map((item) => {
                  const role = roleById.get(item.id);
                  return (
                    <div
                      key={item.id}
                      className="flex flex-col gap-3 rounded-xl border border-slate-200 p-4 sm:flex-row sm:items-center sm:justify-between"
                    >
                      <div className="min-w-0">
                        <p className="truncate text-sm font-semibold text-slate-800">{item.name}</p>
                        <p className="mt-0.5 text-xs text-slate-500">
                          {[item.industry, item.region].filter(Boolean).join(" · ")}
                        </p>
                      </div>
                      <div className="flex shrink-0 flex-wrap items-center gap-2">
                        <span
                          className={`rounded-full px-2.5 py-1 text-xs font-medium ${
                            role === "owner"
                              ? "bg-[#fef2f1] text-[#b42318]"
                              : "bg-slate-100 text-slate-600"
                          }`}
                        >
                          {role === "owner" ? "Владелец" : "Участник"}
                        </span>
                        <a
                          href={routeHref({ name: "company-members", companyId: item.id })}
                          className="rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-600 transition hover:bg-slate-50"
                        >
                          Участники
                        </a>
                        {(role === "owner" || user.role === "admin") && (
                          <a
                            href={routeHref({ name: "company-edit", companyId: item.id })}
                            className="rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-600 transition hover:bg-slate-50"
                          >
                            Изменить
                          </a>
                        )}
                        <button
                          type="button"
                          onClick={() => onOpenCompany(item.id)}
                          className="rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-600 transition hover:bg-slate-50"
                        >
                          Открыть
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </>
        )}
      </section>
    </div>
  );
}
