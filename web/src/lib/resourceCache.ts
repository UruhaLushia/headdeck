type Loader = (path: string, signal: AbortSignal) => Promise<unknown>;

export class ResourceCache {
  private data = new Map<string, { value: unknown; updated: number }>();
  private pending = new Map<
    string,
    { controller: AbortController; promise: Promise<unknown>; invalidated: boolean }
  >();
  private listeners = new Set<() => void>();
  private loader: Loader;

  constructor(loader: Loader) {
    this.loader = loader;
  }

  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };

  get<T>(path: string): T | undefined {
    return this.data.get(path)?.value as T | undefined;
  }

  updatedAt(path: string): number {
    return this.data.get(path)?.updated ?? Date.now();
  }

  set<T>(path: string, value: T | undefined) {
    this.data.delete(path);
    if (value !== undefined) this.data.set(path, { value, updated: Date.now() });
    while (this.data.size > 64) this.data.delete(this.data.keys().next().value!);
    for (const listener of this.listeners) listener();
  }

  async fetch(path: string, revalidate = false): Promise<unknown> {
    const existing = this.pending.get(path);
    if (existing) {
      if (revalidate) existing.invalidated = true;
      return existing.promise;
    }
    if (this.pending.size >= 64) throw new Error("Too many pending requests");
    const controller = new AbortController();
    const promise = this.load(path, controller.signal).finally(() => {
      if (this.pending.get(path)?.controller === controller) this.pending.delete(path);
    });
    this.pending.set(path, { controller, promise, invalidated: false });
    return promise;
  }

  private async load(path: string, signal: AbortSignal): Promise<unknown> {
    for (;;) {
      const entry = this.pending.get(path);
      if (entry) entry.invalidated = false;
      const initial = this.get(path);
      const next = await this.loader(path, signal);
      if (signal.aborted) return next;
      if (this.pending.get(path)?.invalidated) continue;
      const current = this.get<{ revision?: number }>(path);
      const revision =
        next && typeof next === "object" && "revision" in next ? next.revision : undefined;
      if (
        current === initial ||
        (typeof current?.revision === "number" &&
          typeof revision === "number" &&
          current.revision <= revision)
      )
        this.set(path, next);
      return next;
    }
  }

  cancel() {
    for (const entry of this.pending.values()) entry.controller.abort();
    this.pending.clear();
  }
}
