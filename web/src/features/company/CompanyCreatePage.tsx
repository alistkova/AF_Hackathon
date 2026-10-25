import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "../../api/client";
import type { Company } from "../../api/types";
import { routeHref } from "../../shared/hash";
import { CompanyForm } from "./CompanyForm";
import {
  companyPayload,
  emptyCompanyForm,
  validateCompanyForm,
  type CompanyFormValues,
} from "./form";

type Props = {
  onCreated: (company: Company) => void | Promise<void>;
};

export function CompanyCreatePage({ onCreated }: Props) {
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function submit(values: CompanyFormValues) {
    setError(null);

    const problem = validateCompanyForm(values);
    if (problem) {
      setError(problem);
      return;
    }

    setPending(true);
    try {
      const created = await api.createCompany(companyPayload(values));
      await queryClient.invalidateQueries({ queryKey: ["companies"] });
      await queryClient.invalidateQueries({ queryKey: ["memberships"] });
      await onCreated(created);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Не удалось создать компанию");
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
        <h1 className="text-2xl font-bold tracking-tight text-slate-950">Новая компания</h1>
        <p className="mt-1 text-sm text-slate-500">
          Вы станете владельцем компании и сможете добавить сотрудников.
        </p>
      </div>

      <CompanyForm
        initial={emptyCompanyForm}
        submitLabel="Создать компанию"
        pending={pending}
        error={error}
        onSubmit={submit}
        cancelHref={routeHref({ name: "profile" })}
      />
    </div>
  );
}
