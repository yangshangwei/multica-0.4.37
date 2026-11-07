import type { NavigationAdapter } from "../../navigation";

export function getRequestedIntegration(searchParams: URLSearchParams): string | null {
  const tab = searchParams.get("tab");
  if (tab === "github" || tab === "lark") return tab;
  return tab === "integrations" ? searchParams.get("integration") || null : null;
}

export function buildSettingsHref(
  location: Pick<NavigationAdapter, "pathname" | "searchParams" | "hash">,
  tab: string,
  integration?: string | null,
): string {
  const params = new URLSearchParams(location.searchParams);
  params.set("tab", tab);
  if (integration) {
    params.set("integration", integration);
  } else {
    params.delete("integration");
  }
  return `${location.pathname}?${params.toString()}${location.hash}`;
}
