export const productLabels: Record<string, string> = {
  rko: "РКО",
  guarantees: "Гарантии",
  loans: "Кредиты",
  leasing: "Лизинг",
  deposits: "Депозиты",
  payroll: "Зарплатный проект",
  factoring: "Факторинг",
};

export const attributeLabels: Record<string, string> = {
  org_type: "Организационная форма",
  okfs_name: "Форма собственности",
  company_age_years: "Возраст компании",
  eco_mark: "Эко-маркировка",
  negative_mark: "Негативная маркировка",
  workers_fns: "Сотрудники (ФНС)",
  workers_spark: "Сотрудники (СПАРК)",
  fts_flag: "ФТС",
  fts_country: "Страна ФТС",
};

export const roleLabels: Record<string, string> = {
  admin: "Администратор",
  analyst: "Аналитик",
};

function isEnabled(value: string): boolean {
  const normalized = value.trim().toLowerCase();
  return normalized !== "" && normalized !== "0" && normalized !== "false" && normalized !== "нет";
}

export function activeProducts(products?: Record<string, string>): string[] {
  if (!products) return [];
  return Object.entries(products)
    .filter(([, value]) => isEnabled(value))
    .map(([key]) => productLabels[key] ?? key);
}

export function activeAttributes(attributes?: Record<string, string>): Array<[string, string]> {
  if (!attributes) return [];
  return Object.entries(attributes)
    .filter(([, value]) => value.trim() !== "")
    .map(([key, value]) => [attributeLabels[key] ?? key, key === "fts_flag" && isEnabled(value) ? "есть" : value]);
}
