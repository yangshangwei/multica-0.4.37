import type { ReactNode } from "react";
import { AccountRoute } from "@/platform/account-route";
import { WebAccountShell } from "@/platform/account-shell";

export default function AccountLayout({ children }: { children: ReactNode }) {
  return <AccountRoute><WebAccountShell>{children}</WebAccountShell></AccountRoute>;
}
