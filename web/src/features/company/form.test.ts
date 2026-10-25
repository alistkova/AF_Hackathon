import { describe, expect, it } from "vitest";
import { companyFormValues, companyPayload, emptyCompanyForm, validateCompanyForm } from "./form";

describe("validateCompanyForm", () => {
  it("accepts a filled and a minimal form", () => {
    expect(validateCompanyForm({ ...emptyCompanyForm, name: "Ромашка" })).toBeNull();
    expect(
      validateCompanyForm({
        ...emptyCompanyForm,
        name: "Ромашка",
        inn: "7701234567",
        revenue: "1000000",
      }),
    ).toBeNull();
  });

  it("rejects invalid values", () => {
    expect(validateCompanyForm(emptyCompanyForm)).toBe("Укажите название компании");
    expect(validateCompanyForm({ ...emptyCompanyForm, name: "Ромашка", inn: "123" })).toBe(
      "ИНН должен содержать 10 или 12 цифр",
    );
    expect(validateCompanyForm({ ...emptyCompanyForm, name: "Ромашка", inn: "77o1234567" })).toBe(
      "ИНН должен содержать 10 или 12 цифр",
    );
    expect(validateCompanyForm({ ...emptyCompanyForm, name: "Ромашка", revenue: "много" })).toBe(
      "Выручка должна быть числом",
    );
  });
});

describe("companyPayload", () => {
  it("trims values and drops empties", () => {
    expect(
      companyPayload({
        ...emptyCompanyForm,
        name: "  Ромашка ",
        inn: " 7701234567 ",
        industry: " ",
      }),
    ).toEqual({
      name: "Ромашка",
      inn: "7701234567",
      industry: undefined,
      region: undefined,
      city: undefined,
      description: undefined,
      revenue: undefined,
    });
  });
});

describe("companyFormValues", () => {
  it("prefills the edit form from a company", () => {
    expect(
      companyFormValues({
        id: "cmp_1",
        name: "Магнит",
        inn: "5063471190",
        industry: "Ритейл",
        region: "Краснодар",
        description: "Сеть магазинов",
        city: "Краснодар",
        revenue: "2500000000000",
        created_at: "2026-10-22T10:00:00Z",
      }),
    ).toEqual({
      name: "Магнит",
      inn: "5063471190",
      industry: "Ритейл",
      region: "Краснодар",
      city: "Краснодар",
      description: "Сеть магазинов",
      revenue: "2500000000000",
    });
  });

  it("fills missing optional fields with empty strings", () => {
    const values = companyFormValues({
      id: "cmp_1",
      name: "Ромашка",
      inn: "",
      industry: "",
      region: "",
      description: "",
      created_at: "2026-10-22T10:00:00Z",
    });
    expect(values.city).toBe("");
    expect(values.revenue).toBe("");
  });
});
