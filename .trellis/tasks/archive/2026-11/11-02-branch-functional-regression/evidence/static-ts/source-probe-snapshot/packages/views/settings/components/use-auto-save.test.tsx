import { StrictMode } from "react";
import { act, render, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useAutoSave } from "./use-auto-save";

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

function AutoSaveHarness({
  value,
  onSave,
  onSuccess,
}: {
  value: string;
  onSave: (value: string) => Promise<void>;
  onSuccess: (value: string) => void;
}) {
  useAutoSave({
    value,
    savedValue: "",
    onSave,
    onSuccess,
    delay: 650,
    isEqual: (left, right) => left === right,
  });
  return null;
}

describe("useAutoSave success feedback", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("reports success after the persisted value resolves", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const onSave = vi.fn(async () => undefined);
    const onSuccess = vi.fn();
    render(
      <AutoSaveHarness value="saved" onSave={onSave} onSuccess={onSuccess} />,
    );

    act(() => vi.advanceTimersByTime(650));

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith("saved");
      expect(onSuccess).toHaveBeenCalledWith("saved");
    });
  });

  it("still reports success under StrictMode's double-invoked mount", async () => {
    // StrictMode runs setup/cleanup/setup on mount. A mount effect that only
    // clears the mounted flag in cleanup leaves it false for the component's
    // life, so a successful save is silently dropped: neither the "saved"
    // status nor onSuccess ever fires and the indicator spins forever.
    const onSave = vi.fn(async () => undefined);
    const onSuccess = vi.fn();
    render(
      <StrictMode>
        <AutoSaveHarness value="saved" onSave={onSave} onSuccess={onSuccess} />
      </StrictMode>,
    );

    await waitFor(() => expect(onSave).toHaveBeenCalledWith("saved"));
    await waitFor(() => expect(onSuccess).toHaveBeenCalledWith("saved"));
  });

  it("waits for the latest queued value before reporting success", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const first = deferred<void>();
    const second = deferred<void>();
    const onSave = vi
      .fn<(value: string) => Promise<void>>()
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise);
    const onSuccess = vi.fn();
    const { rerender } = render(
      <AutoSaveHarness value="first" onSave={onSave} onSuccess={onSuccess} />,
    );

    act(() => vi.advanceTimersByTime(650));
    await waitFor(() => expect(onSave).toHaveBeenCalledWith("first"));

    rerender(
      <AutoSaveHarness value="second" onSave={onSave} onSuccess={onSuccess} />,
    );
    act(() => vi.advanceTimersByTime(650));

    await act(async () => first.resolve());
    await waitFor(() => expect(onSave).toHaveBeenCalledWith("second"));
    expect(onSuccess).not.toHaveBeenCalled();

    await act(async () => second.resolve());
    await waitFor(() => expect(onSuccess).toHaveBeenCalledWith("second"));
    expect(onSuccess).toHaveBeenCalledTimes(1);
  });

  it("lets a confirmed caller await failure and retry without clearing its target", async () => {
    const pending = deferred<void>();
    const onSave = vi.fn().mockReturnValueOnce(pending.promise).mockResolvedValue(undefined);
    const { result } = renderHook(() => useAutoSave({
      value: "original", savedValue: "original", onSave,
      isEqual: (left, right) => left === right,
    }));
    let settled = false;
    let save!: Promise<unknown>;
    act(() => {
      save = Promise.resolve(result.current.saveNow("removed")).then((saved) => {
        settled = true;
        return saved;
      });
    });
    await act(async () => { await Promise.resolve(); });
    expect(settled).toBe(false);

    await act(async () => pending.reject(new Error("offline")));
    await expect(save).resolves.toBe(false);
    expect(result.current.status).toBe("error");
    await act(async () => { expect(await result.current.saveNow("removed")).toBe(true); });
    expect(onSave).toHaveBeenCalledTimes(2);
  });

  it("settles a queued confirmation only after its own serialized write finishes", async () => {
    const first = deferred<void>();
    const second = deferred<void>();
    const onSave = vi.fn().mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    const { result } = renderHook(() => useAutoSave({
      value: "original", savedValue: "original", onSave,
      isEqual: (left, right) => left === right,
    }));
    let settled = false;
    let confirmation!: Promise<unknown>;
    act(() => {
      void result.current.saveNow("edited");
      confirmation = Promise.resolve(result.current.saveNow("removed")).then((saved) => {
        settled = true;
        return saved;
      });
    });
    await act(async () => { await Promise.resolve(); });
    expect(settled).toBe(false);
    expect(onSave).toHaveBeenCalledTimes(1);
    await act(async () => first.resolve());
    expect(onSave).toHaveBeenLastCalledWith("removed");
    expect(settled).toBe(false);
    await act(async () => second.resolve());
    await expect(confirmation).resolves.toBe(true);
  });

  it("waits before restoring the previous value when an earlier write is still in flight", async () => {
    const first = deferred<void>();
    const onSave = vi.fn().mockReturnValueOnce(first.promise).mockResolvedValue(undefined);
    const { result } = renderHook(() => useAutoSave({
      value: "original", savedValue: "original", onSave,
      isEqual: (left, right) => left === right,
    }));
    let restore!: Promise<boolean>;
    act(() => {
      void result.current.saveNow("added");
      restore = result.current.saveNow("original");
    });
    expect(onSave).toHaveBeenCalledTimes(1);
    await act(async () => first.resolve());
    await expect(restore).resolves.toBe(true);
    expect(onSave).toHaveBeenLastCalledWith("original");
  });
});
