import { GetHome } from '../../wailsjs/go/main/App';

let cachedHome = '';
// Resolved once at module load. Until the promise settles (and permanently
// under vitest, where the binding stub resolves null) paths render uncontracted.
void GetHome().then((h) => {
  cachedHome = h || '';
});

/** The user's home directory as reported by the backend ('' until resolved). */
export function homeDir(): string {
  return cachedHome;
}

/**
 * Contract a home-prefixed absolute path to "~/…" for display — applied BEFORE
 * middleTruncate so the elision budget is spent on the interesting tail.
 * Non-home paths (including docker: pseudo-paths) pass through untouched.
 */
export function contractHome(path: string, home: string): string {
  if (!home) return path;
  if (path === home) return '~';
  const prefix = home.endsWith('/') ? home : `${home}/`;
  return path.startsWith(prefix) ? `~/${path.slice(prefix.length)}` : path;
}
