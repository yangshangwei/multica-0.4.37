#!/usr/bin/env node
import { atomicWrite, isMain, parseArguments, readFeed, runMain } from "./changelog-lib.mjs";

export function publishChangelog(input, destination) {
  const { bytes, feed } = readFeed(input);
  atomicWrite(destination, bytes);
  return feed;
}

if (isMain(import.meta.url)) runMain(() => {
  if (process.argv.includes("--help")) {
    process.stdout.write("Usage: node scripts/publish-changelog.mjs --input JSON --destination CHANGELOG_FILE\nThe destination directory must already exist. Maximum feed size: 2 MiB.\n");
    return;
  }
  const options = parseArguments(process.argv.slice(2), ["input", "destination"], ["input", "destination"]);
  const feed = publishChangelog(options.input, options.destination);
  process.stdout.write(`Published ${feed.releases.length} changelog entries\n`);
});
