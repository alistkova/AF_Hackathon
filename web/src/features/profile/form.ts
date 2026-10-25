export type PasswordFields = {
  current: string;
  next: string;
  confirm: string;
};

export function passwordFieldsEmpty(fields: PasswordFields): boolean {
  return !fields.current && !fields.next && !fields.confirm;
}

export function validateEmail(value: string): string | null {
  const email = value.trim();
  if (!email) return "Укажите email";
  if (email.length > 254 || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    return "Укажите корректный email";
  }
  return null;
}

export function validatePasswordChange(fields: PasswordFields): string | null {
  if (!fields.current) return "Укажите текущий пароль";
  if (fields.next.length < 8) return "Новый пароль должен быть не короче 8 символов";
  if (fields.next.length > 72) return "Новый пароль слишком длинный";
  if (fields.next === fields.current) return "Новый пароль совпадает с текущим";
  if (fields.next !== fields.confirm) return "Пароли не совпадают";
  return null;
}
