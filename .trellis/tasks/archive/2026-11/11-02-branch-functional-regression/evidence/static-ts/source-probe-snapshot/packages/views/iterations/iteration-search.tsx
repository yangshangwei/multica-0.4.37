"use client";

import { Search } from "lucide-react";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";

export function IterationSearch({ id, label, placeholder = label, value, onChange }: {
  id: string;
  label: string;
  placeholder?: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return <div className="relative w-48 max-w-full min-w-0">
    <Label htmlFor={id} className="sr-only">{label}</Label>
    <Search aria-hidden className="pointer-events-none absolute inset-y-0 left-2.5 my-auto size-3.5 text-muted-foreground" />
    <Input id={id} placeholder={placeholder} value={value} onChange={(event) => onChange(event.target.value)} className="border-transparent bg-muted/40 pl-8 text-body hover:bg-muted/60 focus-visible:bg-background pointer-coarse:min-h-11 dark:bg-muted/40" />
  </div>;
}
