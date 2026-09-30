import { useEffect, useState } from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { SettingsTab } from "@multica/views/settings";
import { useT } from "@multica/views/i18n";
import { toast } from "sonner";
import type { CloseBehavior } from "../../../shared/close-behavior";

/**
 * desktop-behavior-settings-tab.tsx
 *
 * Settings tab hosting the close-behavior preference (quit / minimize /
 * ask). Uses the same SettingsTab wrapper as runtime-config-settings-tab
 * and other desktop settings to keep visual rhythm.
 *
 * Lives in the desktop renderer, not packages/views — this is platform
 * plumbing, not business UI, so it should not be renderable from web.
 */
export function DesktopBehaviorSettingsTab() {
  const { t } = useT("desktop");
  const [behavior, setBehavior] = useState<CloseBehavior | null>(null);
  const [traySupported, setTraySupported] = useState<boolean | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const [b, s] = await Promise.all([
        window.closeBehaviorAPI.get(),
        window.closeBehaviorAPI.isTraySupported(),
      ]);
      if (cancelled) return;
      setBehavior(b);
      setTraySupported(s);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const onChange = async (next: CloseBehavior) => {
    if (next === behavior) return;
    setBehavior(next);
    setSaving(true);
    try {
      const result = await window.closeBehaviorAPI.set(next);
      if (!result.ok) {
        toast.error(result.reason ?? "persist_failed");
        // Main exposes only the last successfully persisted preference.
        setBehavior(await window.closeBehaviorAPI.get());
      } else {
        toast.success(t(($) => $.close_behavior.saved), {
          id: "desktop-close-behavior-saved",
        });
      }
    } finally {
      setSaving(false);
    }
  };

  if (behavior === null || traySupported === null) {
    return (
      <SettingsTab
        title={t(($) => $.close_behavior.title)}
        description={t(($) => $.close_behavior.description)}
      >
        <div />
      </SettingsTab>
    );
  }

  const items = [
    { value: "quit" as const, label: t(($) => $.close_behavior.quit) },
    { value: "minimize" as const, label: t(($) => $.close_behavior.minimize) },
    { value: "ask" as const, label: t(($) => $.close_behavior.ask) },
  ];
  const availableItems = items.filter((item) => traySupported || item.value !== "minimize");

  return (
    <SettingsTab
      title={t(($) => $.close_behavior.title)}
      description={t(($) => $.close_behavior.description)}
    >
      <div className="space-y-3">
        <Select
          value={behavior}
          onValueChange={(v) => void onChange(v as CloseBehavior)}
          disabled={saving}
          items={items}
        >
          <SelectTrigger size="sm" className="w-full max-w-md">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {availableItems.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {!traySupported && (
          <p className="text-caption text-muted-foreground">
            {t(($) => $.close_behavior.unsupported)}
          </p>
        )}
      </div>
    </SettingsTab>
  );
}
