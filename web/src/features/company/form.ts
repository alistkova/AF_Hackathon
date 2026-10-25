import type { Company, CompanyCreateInput } from "../../api/types";

export type CompanyFormValues = {
  name: string;
  inn: string;
  industry: string;
  region: string;
  city: string;
  description: string;
  revenue: string;
};

export const emptyCompanyForm: CompanyFormValues = {
  name: "",
  inn: "",
  industry: "",
  region: "",
  city: "",
  description: "",
  revenue: "",
};

export function validateCompanyForm(values: CompanyFormValues): string | null {
  const name = values.name.trim();
  if (!name) return "Укажите название компании";
  if (name.length > 200) return "Название компании слишком длинное";

  const inn = values.inn.trim();
  if (inn && !/^\d{10}$|^\d{12}$/.test(inn)) {
    return "ИНН должен содержать 10 или 12 цифр";
  }

  const revenue = values.revenue.trim();
  if (revenue && !/^\d+$/.test(revenue)) {
    return "Выручка должна быть числом";
  }
  return null;
}

export function companyPayload(values: CompanyFormValues): CompanyCreateInput {
  return {
    name: values.name.trim(),
    inn: values.inn.trim() || undefined,
    industry: values.industry.trim() || undefined,
    region: values.region.trim() || undefined,
    city: values.city.trim() || undefined,
    description: values.description.trim() || undefined,
    revenue: values.revenue.trim() || undefined,
  };
}

export function companyFormValues(company: Company): CompanyFormValues {
  return {
    name: company.name,
    inn: company.inn ?? "",
    industry: company.industry ?? "",
    region: company.region ?? "",
    city: company.city ?? "",
    description: company.description ?? "",
    revenue: company.revenue ?? "",
  };
}
