import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../../api/client";
import type { ProfileUpdate, User } from "../../api/types";
import { passwordFieldsEmpty, validateEmail, validatePasswordChange } from "./form";
import { roleLabels } from "./companyContext";

type Props = {
  user: User;
  onUserChange: (user: User) => void;
};

export function ProfileEditPage({ user, onUserChange }: Props) {
  const [email, setEmail] = useState(user.email);
  const [primaryCompanyId, setPrimaryCompanyId] = useState(user.primary_company_id ?? "");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [pending, setPending] = useState(false);

  const companies = useQuery({
    queryKey: ["companies"],
    queryFn: api.companies,
  });

  const options = companies.data?.items ?? [];
  const loading = companies.isLoading;

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setError(null);
    setSaved(false);

    const emailError = validateEmail(email);
    if (emailError) {
      setError(emailError);
      return;
    }

    const changes: ProfileUpdate = {};
    const nextEmail = email.trim();
    if (nextEmail !== user.email) {
      changes.email = nextEmail;
    }
    if (primaryCompanyId !== (user.primary_company_id ?? "")) {
      changes.primary_company_id = primaryCompanyId;
    }

    const passwordFields = {
      current: currentPassword,
      next: newPassword,
      confirm: confirmPassword,
    };
    const wantsPassword = !passwordFieldsEmpty(passwordFields);
    if (wantsPassword) {
      const passwordError = validatePasswordChange(passwordFields);
      if (passwordError) {
        setError(passwordError);
        return;
      }
    }

    setPending(true);
    try {
      let updated = user;
      if (Object.keys(changes).length > 0) {
        updated = await api.updateProfile(changes);
        onUserChange(updated);
        setEmail(updated.email);
        setPrimaryCompanyId(updated.primary_company_id ?? "");
      }
      if (wantsPassword) {
        await api.changePassword(currentPassword, newPassword);
        setCurrentPassword("");
        setNewPassword("");
        setConfirmPassword("");
      }
      setSaved(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Не удалось сохранить изменения");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <a href="#/profile" className="text-xs font-semibold text-[#b42318] hover:underline">
            ← Назад в профиль
          </a>
          <h1 className="mt-2 text-2xl font-bold tracking-tight text-slate-950">
            Редактирование профиля
          </h1>
          <p className="mt-1 text-sm text-slate-500">
            Измените email, основную компанию или пароль.
          </p>
        </div>
        <span className="rounded-full bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-700">
          {roleLabels[user.role] ?? user.role}
        </span>
      </div>

      <form onSubmit={submit} className="space-y-6">
        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <h2 className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
            Учётная запись
          </h2>
          <label className="mt-4 block max-w-md text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
            Email
            <input
              type="email"
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              className="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm font-normal normal-case tracking-normal text-slate-900 outline-none focus:border-[#ef3124]"
            />
          </label>
          <p className="mt-2 text-xs text-slate-400">
            После смены email входите в сервис с новым адресом.
          </p>
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <h2 className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
            Основная компания
          </h2>
          {loading ? (
            <div className="mt-4 h-10 w-full max-w-md animate-pulse rounded-xl bg-slate-100" />
          ) : companies.isError ? (
            <p className="mt-3 rounded-xl bg-red-50 p-3 text-sm text-red-700">
              Не удалось загрузить компании. Проверьте, что сервисы запущены.
            </p>
          ) : options.length === 0 ? (
            <p className="mt-3 max-w-xl text-sm text-slate-500">
              Вы пока не добавлены ни в одну компанию.{" "}
              <a href="#/companies/new" className="font-semibold text-[#b42318] hover:underline">
                Создайте компанию
              </a>{" "}
              или попросите владельца добавить вас.
            </p>
          ) : (
            <label className="mt-4 block max-w-md text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
              Компания
              <select
                value={primaryCompanyId}
                onChange={(event) => setPrimaryCompanyId(event.target.value)}
                className="mt-2 w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm font-normal normal-case tracking-normal text-slate-900 outline-none focus:border-[#ef3124]"
              >
                <option value="">Не выбрана</option>
                {options.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
              </select>
            </label>
          )}
          <p className="mt-2 text-xs text-slate-400">
            Контекст этой компании показывается в профиле и используется в ленте.
          </p>
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <h2 className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
            Пароль
          </h2>
          <p className="mt-2 text-xs text-slate-400">
            Заполните три поля, чтобы сменить пароль. Остальные сессии будут завершены.
          </p>
          <div className="mt-4 grid max-w-2xl gap-4 sm:grid-cols-3">
            <label className="block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
              Текущий
              <input
                type="password"
                autoComplete="current-password"
                value={currentPassword}
                onChange={(event) => setCurrentPassword(event.target.value)}
                className="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm font-normal normal-case tracking-normal text-slate-900 outline-none focus:border-[#ef3124]"
              />
            </label>
            <label className="block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
              Новый
              <input
                type="password"
                autoComplete="new-password"
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                className="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm font-normal normal-case tracking-normal text-slate-900 outline-none focus:border-[#ef3124]"
              />
            </label>
            <label className="block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
              Повторите
              <input
                type="password"
                autoComplete="new-password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
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
        {saved && (
          <p className="rounded-xl bg-emerald-50 p-3 text-sm text-emerald-700">
            Изменения сохранены.{" "}
            <a href="#/profile" className="font-semibold underline">
              Вернуться в профиль
            </a>
          </p>
        )}

        <div className="flex flex-wrap items-center gap-2">
          <button
            type="submit"
            disabled={pending}
            className="rounded-xl bg-[#ef3124] px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-[#d92b20] disabled:cursor-wait disabled:opacity-60"
          >
            {pending ? "Сохраняем…" : "Сохранить"}
          </button>
          <a
            href="#/profile"
            className="rounded-xl border border-slate-200 px-3 py-2.5 text-sm font-semibold text-slate-600 transition hover:bg-slate-50"
          >
            Отмена
          </a>
        </div>
      </form>
    </div>
  );
}
