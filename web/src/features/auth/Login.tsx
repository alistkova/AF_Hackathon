import { useState } from "react";
import { api } from "../../api/client";
import type { User } from "../../api/types";

type Props = {
  onSuccess: (user: User) => void;
};

export function Login({ onSuccess }: Props) {
  const [email, setEmail] = useState("demo-client@alfa.local");
  const [password, setPassword] = useState("demo-password");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setPending(true);
    setError(null);
    try {
      const user = await api.login(email.trim(), password);
      onSuccess(user);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Не удалось войти");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-[#f5f7fa] px-4">
      <form
        onSubmit={submit}
        className="w-full max-w-sm rounded-2xl border border-slate-200 bg-white p-6 shadow-sm"
      >
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-[#ef3124] text-lg font-black text-white">
            А
          </div>
          <div>
            <p className="text-sm font-bold tracking-tight text-slate-950">Альфа Хак</p>
            <p className="text-xs text-slate-500">Новостная аналитика</p>
          </div>
        </div>

        <label className="mt-6 block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
          Email
          <input
            type="email"
            required
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            className="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm font-normal normal-case tracking-normal text-slate-900 outline-none focus:border-[#ef3124]"
          />
        </label>

        <label className="mt-4 block text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
          Пароль
          <input
            type="password"
            required
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            className="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm font-normal normal-case tracking-normal text-slate-900 outline-none focus:border-[#ef3124]"
          />
        </label>

        {error && <p className="mt-4 text-sm text-red-600">{error}</p>}

        <button
          type="submit"
          disabled={pending}
          className="mt-6 w-full rounded-xl bg-[#ef3124] px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-[#d92b20] disabled:cursor-wait disabled:opacity-60"
        >
          {pending ? "Входим…" : "Войти"}
        </button>

        <p className="mt-4 text-xs leading-5 text-slate-500">
          Демо-доступ: <span className="font-mono">demo-client@alfa.local</span> видит только свои
          компании, <span className="font-mono">demo-admin@alfa.local</span> — все.
        </p>
      </form>
    </div>
  );
}
