import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../api/client";
import type { User } from "../../api/types";
import { routeHref } from "../../shared/hash";

type Props = {
  companyId: string;
  user: User;
  onOpenCompany: (companyId: string) => void;
};

export function CompanyMembersPage({ companyId, user, onOpenCompany }: Props) {
  const queryClient = useQueryClient();
  const [email, setEmail] = useState("");
  const [notice, setNotice] = useState<string | null>(null);

  const company = useQuery({
    queryKey: ["company", companyId],
    queryFn: () => api.company(companyId),
    retry: false,
  });
  const members = useQuery({
    queryKey: ["company-members", companyId],
    queryFn: () => api.companyMembers(companyId),
    retry: false,
  });

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: ["company-members", companyId] });
    await queryClient.invalidateQueries({ queryKey: ["memberships"] });
    await queryClient.invalidateQueries({ queryKey: ["companies"] });
  };

  const add = useMutation({
    mutationFn: (value: string) => api.addCompanyMember(companyId, value),
    onSuccess: async () => {
      setEmail("");
      setNotice("Участник добавлен");
      await invalidate();
    },
  });
  const remove = useMutation({
    mutationFn: (userId: string) => api.removeCompanyMember(companyId, userId),
    onSuccess: async () => {
      setNotice("Участник удалён");
      await invalidate();
    },
  });
  const updateRole = useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: "owner" | "member" }) =>
      api.updateCompanyMember(companyId, userId, role),
    onSuccess: async () => {
      setNotice("Роль обновлена");
      await invalidate();
    },
  });

  const items = members.data?.items ?? [];
  const isOwner =
    user.role === "admin" ||
    items.some((member) => member.user_id === user.id && member.role === "owner");
  const loading = members.isLoading;
  const failed = members.isError;

  function submit(event: React.FormEvent) {
    event.preventDefault();
    setNotice(null);
    const value = email.trim();
    if (!value) return;
    add.mutate(value);
  }

  return (
    <div className="space-y-6">
      <a
        href={routeHref({ name: "profile" })}
        className="inline-flex items-center gap-1 text-xs font-semibold text-[#b42318] transition hover:underline"
      >
        ← Назад в профиль
      </a>

      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-950">
            {company.data?.name ?? "Участники компании"}
          </h1>
          <p className="mt-1 text-sm text-slate-500">
            {company.data
              ? [company.data.industry, company.data.region].filter(Boolean).join(" · ")
              : "Управление доступом сотрудников"}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {isOwner && (
            <a
              href={routeHref({ name: "company-edit", companyId })}
              className="rounded-xl bg-[#ef3124] px-3 py-2 text-xs font-semibold text-white transition hover:bg-[#d92b20]"
            >
              Изменить компанию
            </a>
          )}
          <button
            type="button"
            onClick={() => onOpenCompany(companyId)}
            className="rounded-xl border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-600 transition hover:bg-slate-50"
          >
            Открыть в ленте
          </button>
        </div>
      </div>

      {loading && <div className="h-40 animate-pulse rounded-2xl bg-white/70" />}

      {failed && (
        <div className="rounded-2xl border border-red-200 bg-red-50 p-5 text-sm text-red-700">
          Компания недоступна или вы не являетесь её участником.{" "}
          <a href={routeHref({ name: "profile" })} className="font-semibold underline">
            Вернуться в профиль
          </a>
        </div>
      )}

      {!loading && !failed && (
        <>
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h2 className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
                Участники
              </h2>
              <span className="rounded-full bg-slate-100 px-3 py-1 text-xs text-slate-500">
                {items.length}
              </span>
            </div>

            <div className="mt-4 space-y-2">
              {items.map((member) => (
                <div
                  key={member.user_id}
                  className="flex flex-col gap-3 rounded-xl border border-slate-200 p-4 sm:flex-row sm:items-center sm:justify-between"
                >
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold text-slate-800">
                      {member.email}
                      {member.user_id === user.id && (
                        <span className="ml-2 text-xs font-normal text-slate-400">это вы</span>
                      )}
                    </p>
                    <p className="mt-0.5 text-xs text-slate-500">
                      {member.role === "owner" ? "Владелец компании" : "Участник компании"}
                    </p>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <span
                      className={`rounded-full px-2.5 py-1 text-xs font-medium ${
                        member.role === "owner"
                          ? "bg-[#fef2f1] text-[#b42318]"
                          : "bg-slate-100 text-slate-600"
                      }`}
                    >
                      {member.role === "owner" ? "Владелец" : "Участник"}
                    </span>
                    {isOwner && (
                      <button
                        type="button"
                        onClick={() =>
                          updateRole.mutate({
                            userId: member.user_id,
                            role: member.role === "owner" ? "member" : "owner",
                          })
                        }
                        disabled={updateRole.isPending}
                        className="rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-600 transition hover:bg-slate-50 disabled:cursor-wait disabled:opacity-60"
                      >
                        {member.role === "owner" ? "Сделать участником" : "Сделать владельцем"}
                      </button>
                    )}
                    {isOwner && member.role !== "owner" && (
                      <button
                        type="button"
                        onClick={() => remove.mutate(member.user_id)}
                        disabled={remove.isPending}
                        className="rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-semibold text-red-600 transition hover:bg-red-50 disabled:cursor-wait disabled:opacity-60"
                      >
                        Удалить
                      </button>
                    )}
                  </div>
                </div>
              ))}
              {items.length === 0 && (
                <p className="rounded-xl border border-dashed border-slate-300 p-4 text-sm text-slate-500">
                  В компании пока нет участников.
                </p>
              )}
            </div>
          </section>

          {isOwner ? (
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <h2 className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
                Добавить участника
              </h2>
              <p className="mt-2 text-xs text-slate-500">
                Пользователь должен быть зарегистрирован в сервисе.
              </p>
              <form onSubmit={submit} className="mt-4 flex flex-col gap-2 sm:flex-row">
                <input
                  type="email"
                  required
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  placeholder="colleague@example.com"
                  className="w-full max-w-md rounded-xl border border-slate-200 px-3 py-2 text-sm text-slate-900 outline-none focus:border-[#ef3124]"
                />
                <button
                  type="submit"
                  disabled={add.isPending}
                  className="rounded-xl bg-[#ef3124] px-4 py-2 text-sm font-semibold text-white transition hover:bg-[#d92b20] disabled:cursor-wait disabled:opacity-60"
                >
                  {add.isPending ? "Добавляем…" : "Добавить"}
                </button>
              </form>
              {add.isError && (
                <p className="mt-3 text-sm text-red-600">
                  {add.error instanceof Error ? add.error.message : "Не удалось добавить участника"}
                </p>
              )}
              {remove.isError && (
                <p className="mt-3 text-sm text-red-600">
                  {remove.error instanceof Error ? remove.error.message : "Не удалось удалить участника"}
                </p>
              )}
              {updateRole.isError && (
                <p className="mt-3 text-sm text-red-600">
                  {updateRole.error instanceof Error
                    ? updateRole.error.message
                    : "Не удалось изменить роль участника"}
                </p>
              )}
              {notice && <p className="mt-3 text-sm text-emerald-600">{notice}</p>}
            </section>
          ) : (
            <p className="rounded-xl bg-slate-100 p-4 text-sm text-slate-500">
              Добавлять, удалять и менять роли участников может только владелец компании.
            </p>
          )}
        </>
      )}
    </div>
  );
}
