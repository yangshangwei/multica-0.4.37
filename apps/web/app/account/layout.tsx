import type { ReactNode } from "react";
import { AccountRoute } from "@/platform/account-route";

export default function AccountLayout({ children }: { children: ReactNode }) {
  return <AccountRoute>{children}</AccountRoute>;
}
