import type { Project } from "../types/project";

/** Serializes one editor's saves against the revision that editor adopted. */
export class ProjectDescriptionSave {
  private pending: string | undefined;
  private running = false;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private stopped = false;
  revision: number;
  body: string;
  constructor(project: Pick<Project, "description" | "description_revision">,
    private readonly save: (body: string, revision: number) => Promise<Project>,
    private readonly changed: (error?: unknown) => void) {
    this.revision = project.description_revision ?? 0;
    this.body = project.description ?? "";
  }
  enqueue(body: string, delay = 0) {
    this.pending = body;
    if (this.timer) clearTimeout(this.timer);
    if (delay > 0) this.timer = setTimeout(() => { this.timer = undefined; void this.drain(); }, delay);
    else void this.drain();
  }
  adopt(body: string, revision: number) { this.body = body; this.revision = revision; this.stopped = false; }
  dispose() { this.stopped = true; this.pending = undefined; if (this.timer) clearTimeout(this.timer); }
  private async drain() {
    if (this.running || this.stopped || this.pending === undefined || !this.revision) return;
    this.running = true;
    while (this.pending !== undefined && !this.stopped) {
      const body: string = this.pending; this.pending = undefined;
      if (body === this.body) continue;
      try {
        const result = await this.save(body, this.revision);
        if (this.stopped) break;
        this.body = result.description ?? ""; this.revision = result.description_revision ?? this.revision;
        this.changed();
      } catch (error) {
        this.pending ??= body;
        this.stopped = true;
        this.changed(error);
      }
    }
    this.running = false;
  }
}
