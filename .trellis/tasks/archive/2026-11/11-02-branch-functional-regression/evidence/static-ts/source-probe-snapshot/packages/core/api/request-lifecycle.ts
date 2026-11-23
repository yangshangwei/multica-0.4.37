export interface ApiRequestOptions {
  signal?: AbortSignal;
  timeoutMs?: number;
}

export const RECOVERY_REQUEST_TIMEOUT_MS = 10_000;

/** Keep cancellation and deadlines active until the complete body is consumed. */
export async function withRequestDeadline<T>(
  options: ApiRequestOptions | undefined,
  endpointSignal: AbortSignal,
  request: (signal: AbortSignal) => Promise<T>,
): Promise<T> {
  const deadline = new AbortController();
  const signal = AbortSignal.any([
    endpointSignal,
    deadline.signal,
    ...(options?.signal ? [options.signal] : []),
  ]);
  signal.throwIfAborted();
  const timer = options?.timeoutMs !== undefined
    ? setTimeout(() => deadline.abort(new DOMException("Request timed out", "TimeoutError")), options.timeoutMs)
    : undefined;
  let onAbort: (() => void) | undefined;
  try {
    return await new Promise<T>((resolve, reject) => {
      onAbort = () => reject(signal.reason);
      signal.addEventListener("abort", onAbort, { once: true });
      // A response/body implementation can finish after abort. The transport
      // checks this same signal before any session side effects or publication.
      request(signal).then(resolve, reject);
    });
  } finally {
    if (timer !== undefined) clearTimeout(timer);
    if (onAbort) signal.removeEventListener("abort", onAbort);
  }
}
