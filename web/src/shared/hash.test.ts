import { describe, expect, it } from "vitest";
import { parseRoute, routeHref } from "./hash";

describe("parseRoute", () => {
  it("maps profile hashes", () => {
    expect(parseRoute("#/profile")).toEqual({ name: "profile" });
    expect(parseRoute("#/profile/")).toEqual({ name: "profile" });
    expect(parseRoute("#/profile/edit")).toEqual({ name: "profile-edit" });
    expect(parseRoute("#/profile/edit/")).toEqual({ name: "profile-edit" });
  });

  it("maps company hashes", () => {
    expect(parseRoute("#/companies/new")).toEqual({ name: "company-new" });
    expect(parseRoute("#/companies/cmp_123/members")).toEqual({
      name: "company-members",
      companyId: "cmp_123",
    });
    expect(parseRoute("#/companies/cmp_123/members/")).toEqual({
      name: "company-members",
      companyId: "cmp_123",
    });
    expect(parseRoute("#/companies/cmp_123/edit")).toEqual({
      name: "company-edit",
      companyId: "cmp_123",
    });
  });

  it("falls back to the feed", () => {
    expect(parseRoute("")).toEqual({ name: "feed" });
    expect(parseRoute("#/")).toEqual({ name: "feed" });
    expect(parseRoute("#/companies")).toEqual({ name: "feed" });
    expect(parseRoute("#/companies/cmp_1")).toEqual({ name: "feed" });
    expect(parseRoute("#/profile/unknown")).toEqual({ name: "feed" });
  });
});

describe("routeHref", () => {
  it("round-trips every route", () => {
    expect(routeHref({ name: "feed" })).toBe("#/");
    expect(routeHref({ name: "profile" })).toBe("#/profile");
    expect(routeHref({ name: "profile-edit" })).toBe("#/profile/edit");
    expect(routeHref({ name: "company-new" })).toBe("#/companies/new");
    expect(routeHref({ name: "company-edit", companyId: "cmp_1" })).toBe("#/companies/cmp_1/edit");
    expect(routeHref({ name: "company-members", companyId: "cmp_1" })).toBe("#/companies/cmp_1/members");
  });
});
