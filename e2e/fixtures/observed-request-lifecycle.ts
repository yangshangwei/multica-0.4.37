export class ObservedRequestLifecycle<Request extends object> {
  private sealed = false;
  private admitted = new WeakSet<Request>();
  private requests = new Map<Request, { done: Promise<void>; finish: () => void }>();
  private readers = new Set<Promise<void>>();
  readonly failures: unknown[] = [];
  excludedRequests = 0;

  admit(request: Request): boolean {
    if (this.sealed) { this.excludedRequests++; return false; }
    this.admitted.add(request);
    let finish!: () => void;
    const done = new Promise<void>((resolve) => { finish = resolve; });
    this.requests.set(request, { done, finish });
    return true;
  }
  includes(request: Request) { return this.admitted.has(request); }
  finish(request: Request) {
    this.requests.get(request)?.finish();
    this.requests.delete(request);
  }
  observe(reader: Promise<void>) {
    const observed = reader.catch((error: unknown) => { this.failures.push(error); });
    this.readers.add(observed);
    void observed.finally(() => this.readers.delete(observed));
  }
  seal() { this.sealed = true; }
  async drain() {
    while (this.requests.size || this.readers.size) {
      await Promise.all([...this.requests.values()].map((request) => request.done).concat([...this.readers]));
    }
    if (this.failures.length) throw new AggregateError(this.failures, "Observed response readers failed");
  }
}
