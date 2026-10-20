import { useQuery } from "@tanstack/react-query";
import { api } from "../../api/client";

type Props = {
  companyId: string;
};

export function CompanyProfile({ companyId }: Props) {
  const company = useQuery({
    queryKey: ["company", companyId],
    queryFn: () => api.company(companyId),
    enabled: Boolean(companyId),
  });

  if (company.isLoading) {
    return <div className="h-40 animate-pulse rounded-2xl bg-white/70" />;
  }

  if (company.isError || !company.data) {
    return (
      <div className="rounded-2xl border border-red-200 bg-red-50 p-5 text-sm text-red-700">
        Профиль компании недоступен.
      </div>
    );
  }

  const item = company.data;

  return (
    <section className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div className="h-1.5 bg-[#ef3124]" />
      <div className="flex flex-col gap-5 p-6 md:flex-row md:items-start md:justify-between">
        <div className="max-w-2xl">
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[#ef3124]">
            {item.industry}
          </p>
          <h1 className="mt-2 text-3xl font-bold tracking-tight text-slate-950">{item.name}</h1>
          <p className="mt-3 text-sm leading-6 text-slate-600">{item.description}</p>
        </div>
        <dl className="grid shrink-0 grid-cols-2 gap-3 text-sm md:min-w-64">
          <div className="rounded-xl bg-slate-50 p-3">
            <dt className="text-xs text-slate-500">Регион</dt>
            <dd className="mt-1 font-semibold text-slate-800">{item.region || "—"}</dd>
          </div>
          <div className="rounded-xl bg-slate-50 p-3">
            <dt className="text-xs text-slate-500">ИНН</dt>
            <dd className="mt-1 font-mono text-xs font-semibold text-slate-800">
              {item.inn || "—"}
            </dd>
          </div>
        </dl>
      </div>
    </section>
  );
}
