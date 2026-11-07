/** String enums intentionally remain open for installed clients on newer servers. */
export interface ChangelogItem {
  text: string;
  commit?: string;
}

export interface ChangelogSection {
  category: string;
  items: ChangelogItem[];
}

export interface ChangelogRelease {
  id: string;
  version: string;
  title: string;
  publishedAt: string | null;
  status: string;
  source: string;
  commit: string | null;
  baseCommit: string | null;
  sections: ChangelogSection[];
}

export interface ChangelogFeed {
  schemaVersion: 1;
  generatedAt: string;
  releases: ChangelogRelease[];
  serverVersion: string | null;
  feedSource: string;
  isStale: boolean;
  warning: string | null;
}
