export type Route =
  | { name: "feed" }
  | { name: "profile" }
  | { name: "profile-edit" }
  | { name: "company-new" }
  | { name: "company-edit"; companyId: string }
  | { name: "company-members"; companyId: string };

function decodeSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

export function parseRoute(hash: string): Route {
  const path = hash.replace(/^#/, "").replace(/\/+$/, "");
  if (path === "/profile") return { name: "profile" };
  if (path === "/profile/edit") return { name: "profile-edit" };
  if (path === "/companies/new") return { name: "company-new" };

  const members = path.match(/^\/companies\/([^/]+)\/members$/);
  if (members) {
    const companyId = decodeSegment(members[1]);
    if (companyId) return { name: "company-members", companyId };
  }

  const edit = path.match(/^\/companies\/([^/]+)\/edit$/);
  if (edit) {
    const companyId = decodeSegment(edit[1]);
    if (companyId) return { name: "company-edit", companyId };
  }
  return { name: "feed" };
}

export function routeHref(route: Route): string {
  switch (route.name) {
    case "profile":
      return "#/profile";
    case "profile-edit":
      return "#/profile/edit";
    case "company-new":
      return "#/companies/new";
    case "company-edit":
      return `#/companies/${encodeURIComponent(route.companyId)}/edit`;
    case "company-members":
      return `#/companies/${encodeURIComponent(route.companyId)}/members`;
    default:
      return "#/";
  }
}
