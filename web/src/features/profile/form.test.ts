import { describe, expect, it } from "vitest";
import { passwordFieldsEmpty, validateEmail, validatePasswordChange } from "./form";

describe("validateEmail", () => {
  it("accepts normal addresses", () => {
    expect(validateEmail("analyst@example.com")).toBeNull();
    expect(validateEmail("  analyst@example.com  ")).toBeNull();
  });

  it("rejects malformed addresses", () => {
    expect(validateEmail("")).toBe("Укажите email");
    expect(validateEmail("not-an-email")).toBe("Укажите корректный email");
    expect(validateEmail("a@b")).toBe("Укажите корректный email");
  });
});

describe("passwordFieldsEmpty", () => {
  it("detects an untouched password block", () => {
    expect(passwordFieldsEmpty({ current: "", next: "", confirm: "" })).toBe(true);
    expect(passwordFieldsEmpty({ current: "", next: "next-secret", confirm: "" })).toBe(false);
  });
});

describe("validatePasswordChange", () => {
  it("accepts a valid change", () => {
    expect(
      validatePasswordChange({ current: "secret-pass", next: "next-secret", confirm: "next-secret" }),
    ).toBeNull();
  });

  it("rejects invalid changes", () => {
    expect(validatePasswordChange({ current: "", next: "next-secret", confirm: "next-secret" })).toBe(
      "Укажите текущий пароль",
    );
    expect(validatePasswordChange({ current: "secret-pass", next: "short", confirm: "short" })).toBe(
      "Новый пароль должен быть не короче 8 символов",
    );
    expect(
      validatePasswordChange({
        current: "secret-pass",
        next: "secret-pass",
        confirm: "secret-pass",
      }),
    ).toBe("Новый пароль совпадает с текущим");
    expect(
      validatePasswordChange({ current: "secret-pass", next: "next-secret", confirm: "mismatch" }),
    ).toBe("Пароли не совпадают");
  });
});
