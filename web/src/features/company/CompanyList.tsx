import { useQuery } from "@tanstack/react-query";
import { api } from "../../api/client";

type Props = {
  selectedId: string;
  onSelect: (id: string) => void;
};

export function CompanyList({ selectedId, onSelect }: Props) {
  const companies = useQuery({
    queryKey: ["companies"],
    queryFn: api.companies,
  });

  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-3 shadow-sm">
      <div className="flex items-center justify-between px-2 py-2">
        <h2 className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
          Компании
        </h2>
        <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-500">
          {companies.data?.items.length ?? 0}
        </span>
      </div>

      {companies.isLoading && <p className="px-2 py-4 text-sm text-slate-500">Загружаем…</p>}

      {companies.isError && (
        <p className="px-2 py-4 text-sm text-red-600">
          Не удалось загрузить компании. Проверьте, что сервисы запущены.
        </p>
      )}

      <div className="mt-1 space-y-1">
        {companies.data?.items.map((company) => {
          const active = company.id === selectedId;
          return (
            <button
              key={company.id}
              type="button"
              onClick={() => onSelect(company.id)}
              className={`w-full rounded-xl px-3 py-3 text-left transition ${
                active
                  ? "bg-[#fef2f1] text-[#b42318]"
                  : "text-slate-700 hover:bg-slate-50 hover:text-slate-950"
              }`}
            >
              <span className="block text-sm font-semibold">{company.name}</span>
              <span className="mt-0.5 block text-xs text-slate-500">
                {company.industry} · {company.region}
              </span>
            </button>
          );
        })}
      </div>
    </section>
  );
}
