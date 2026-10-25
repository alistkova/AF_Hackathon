export function formatDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat("ru-RU", {
    day: "numeric",
    month: "long",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function formatRevenue(value: string | undefined): string {
  if (!value) return "";
  const numeric = Number(value.replace(/\s/g, "").replace(",", "."));
  if (!Number.isFinite(numeric)) return value;
  return new Intl.NumberFormat("ru-RU", {
    style: "currency",
    currency: "RUB",
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(numeric);
}
