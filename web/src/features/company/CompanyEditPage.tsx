import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../api/client";
import type { User } from "../../api/types";
import { routeHref } from "../../shared/hash";
import { CompanyForm } from "./CompanyForm";
import {
  companyFormValues,
  companyPayload,
  validateCompanyForm,
  type CompanyFormValues,
} from "./form";

type Props = {
  companyId: string;
  user: User;
};

export function CompanyEditPage({ companyId, user }: Props) {
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const company = useQuery({
    queryKey: ["company", companyId],
    queryFn: () => api.company(companyId),
    retry: false,
  });
  const memberships = useQuery({
    queryKey: ["memberships"],
    queryFn: api.myMemberships,
  });

  const isOwner =
    user.role === "admin" ||
    (memberships.data?.items ?? []).some(
      (item) => item.company_id === companyId && item.role === "owner",
    );
  const loading = company.isLoading || memberships.isLoading;
  const failed = company.isError || memberships.isError;

  async function submit(values: CompanyFormValues) {
    setError(null);

    const problem = validateCompanyForm(values);
    if (problem) {
      setError(problem);
      return;
    }

    setPending(true);
    try {
      await api.updateCompany(companyId, companyPayload(values));
      await queryClient.invalidateQueries({ queryKey: ["company", companyId] });
      await queryClient.invalidateQueries({ queryKey: ["companies"] });
      window.location.hash = routeHref({ name: "profile" });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Не удалось сохранить компанию");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="space-y-6">
      <a
        href={routeHref({ name: "profile" })}
        className="inline-flex items-center gap-1 text-xs font-semibold text-[#b42318] transition hover:underline"
      >
        ← Назад в профиль
      </a>

      <div>
        <h1 className="text-2xl font-bold tracking-tight text-slate-950">Редактирование компании</h1>
        <p className="mt-1 text-sm text-slate-500">
          {company.data?.name ?? "Загружаем данные компании…"}
        </p>
      </div>

      {loading && <div className="h-64 animate-pulse rounded-2xl bg-white/70" />}

      {!loading && failed && (
        <div className="rounded-2xl border border-red-200 bg-red-50 p-5 text-sm text-red-700">
          Компания недоступна.{" "}
          <a href={routeHref({ name: "profile" })} className="font-semibold underline">
            Вернуться в профиль
          </a>
        </div>
      )}

      {!loading && !failed && !isOwner && (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 p-5 text-sm text-amber-800">
          Редактировать компанию может только её владелец.{" "}
          <a href={routeHref({ name: "profile" })} className="font-semibold underline">
            Вернуться в профиль
          </a>
        </div>
      )}

      {!loading && !failed && isOwner && company.data && (
        <CompanyForm
          initial={companyFormValues(company.data)}
          submitLabel="Сохранить изменения"
          pending={pending}
          error={error}
          onSubmit={submit}
          cancelHref={routeHref({ name: "profile" })}
        />
      )}
    </div>
  );
}
