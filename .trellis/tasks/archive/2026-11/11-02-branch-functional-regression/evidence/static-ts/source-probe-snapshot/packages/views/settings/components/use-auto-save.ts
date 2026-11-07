"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { SettingsSaveStatus } from "./settings-layout";

interface UseAutoSaveOptions<T> {
  value: T;
  savedValue: T;
  onSave: (value: T) => Promise<void>;
  onSuccess?: (value: T) => void;
  onError?: (error: unknown) => void;
  enabled?: boolean;
  delay?: number;
  isEqual: (left: T, right: T) => boolean;
  isValid?: (value: T) => boolean;
}

interface AutoSaveResult<T> {
  status: SettingsSaveStatus;
  flush: () => void;
  /** False means validation, a failed write, or replacement by a newer save. */
  saveNow: (value: T) => Promise<boolean>;
}

const alwaysValid = () => true;

/**
 * Debounces text-heavy settings while serializing requests. If a user edits
 * again during an in-flight save, only the latest queued value is persisted
 * next, so a slower response can never overwrite a newer request.
 */
export function useAutoSave<T>({
  value,
  savedValue,
  onSave,
  onSuccess,
  onError,
  enabled = true,
  delay = 650,
  isEqual,
  isValid = alwaysValid,
}: UseAutoSaveOptions<T>): AutoSaveResult<T> {
  const [status, setStatus] = useState<SettingsSaveStatus>("idle");
  const mountedRef = useRef(true);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const savingRef = useRef(false);
  const queuedRef = useRef<{
    value: T;
    resolve: (saved: boolean) => void;
  } | null>(null);
  const latestValueRef = useRef(value);
  const persistedRef = useRef(savedValue);
  const observedSavedRef = useRef(savedValue);
  const enabledRef = useRef(enabled);
  const onSaveRef = useRef(onSave);
  const onSuccessRef = useRef(onSuccess);
  const onErrorRef = useRef(onError);
  const isEqualRef = useRef(isEqual);
  const isValidRef = useRef(isValid);

  latestValueRef.current = value;
  enabledRef.current = enabled;
  onSaveRef.current = onSave;
  onSuccessRef.current = onSuccess;
  onErrorRef.current = onError;
  isEqualRef.current = isEqual;
  isValidRef.current = isValid;

  if (!isEqual(savedValue, observedSavedRef.current)) {
    observedSavedRef.current = savedValue;
    persistedRef.current = savedValue;
  }

  const runSave = useCallback(async (next: T): Promise<boolean> => {
    if (!enabledRef.current || !isValidRef.current(next)) {
      return false;
    }
    if (savingRef.current) {
      return new Promise<boolean>((resolve) => {
        queuedRef.current?.resolve(false);
        queuedRef.current = { value: next, resolve };
      });
    }
    if (isEqualRef.current(next, persistedRef.current)) {
      return true;
    }

    savingRef.current = true;
    let succeeded = false;
    if (mountedRef.current) setStatus("saving");
    try {
      await onSaveRef.current(next);
      persistedRef.current = next;
      succeeded = true;
    } catch (error) {
      if (mountedRef.current) setStatus("error");
      onErrorRef.current?.(error);
    } finally {
      savingRef.current = false;
      const queued = queuedRef.current;
      queuedRef.current = null;
      if (queued && !isEqualRef.current(queued.value, persistedRef.current)) {
        void runSave(queued.value).then(queued.resolve);
      } else {
        queued?.resolve(true);
        if (succeeded && mountedRef.current) {
          setStatus("saved");
          onSuccessRef.current?.(next);
        }
      }
    }
    return succeeded;
  }, []);

  const saveNow = useCallback(
    (next: T) => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
      return runSave(next);
    },
    [runSave],
  );

  const flush = useCallback(() => {
    void saveNow(latestValueRef.current);
  }, [saveNow]);

  useEffect(() => {
    if (timerRef.current) clearTimeout(timerRef.current);

    if (!enabled || !isValid(value) ||
      (!savingRef.current && isEqual(value, persistedRef.current))) {
      timerRef.current = null;
      if (!enabled || !isValid(value)) setStatus("idle");
      return;
    }

    setStatus("saving");
    timerRef.current = setTimeout(() => {
      timerRef.current = null;
      void runSave(latestValueRef.current);
    }, delay);

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
  }, [delay, enabled, isEqual, isValid, runSave, value]);

  useEffect(() => {
    // Restore the invariant on every mount. Under React StrictMode the initial
    // setup/cleanup/setup cycle would otherwise leave mountedRef pinned to false
    // for the component's whole life, stranding status at "saving".
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  return { status, flush, saveNow };
}
