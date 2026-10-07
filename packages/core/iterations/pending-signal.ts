// In-process notification for durable command drafts. Server state stays in Query.
let revision = 0;
const listeners = new Set<() => void>();
export const pendingIterationRevision = () => revision;
export function subscribePendingIterations(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
export function notifyPendingIterations() {
  revision += 1;
  for (const listener of listeners) listener();
}
