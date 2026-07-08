import { downloadUrl, latestReleaseUrl, PRODUCT } from "@app-cleaner/shared";
import { APP_VERSION } from "../lib/version";

type SafetyLevel = "safe" | "moderate" | "risky";

interface CategoryCopy {
  name: string;
  group: string;
  safety: SafetyLevel;
  description: string;
}

// Verbatim from packages/engine/core/categories.go — keep in sync (16 total).
const CATEGORIES: CategoryCopy[] = [
  { name: "User Cache Files", group: "System Junk", safety: "moderate", description: "Application caches stored in ~/Library/Caches" },
  { name: "System Log Files", group: "System Junk", safety: "moderate", description: "System and application logs" },
  { name: "Temporary Files", group: "System Junk", safety: "safe", description: "Temporary files in /tmp and /var/folders" },
  { name: "Trash", group: "Storage", safety: "safe", description: "Files in the Trash bin" },
  { name: "Old Downloads", group: "Storage", safety: "risky", description: "Downloads older than 30 days" },
  { name: "Browser Cache", group: "Browsers", safety: "safe", description: "Cache from Chrome, Safari, Firefox, and Arc" },
  { name: "Development Cache", group: "Development", safety: "moderate", description: "npm, yarn, pip, Xcode DerivedData, CocoaPods cache" },
  { name: "Homebrew Cache", group: "Development", safety: "safe", description: "Homebrew download cache and old versions" },
  { name: "Docker", group: "Development", safety: "safe", description: "Unused Docker images, containers, and build cache" },
  { name: "iOS Backups", group: "Storage", safety: "risky", description: "iPhone and iPad backup files" },
  { name: "Mail Attachments", group: "Storage", safety: "risky", description: "Downloaded email attachments from Mail.app" },
  { name: "Language Files", group: "System Junk", safety: "risky", description: "Unused language localizations in applications" },
  { name: "Large Files", group: "Large Files", safety: "risky", description: "Files larger than 500MB for review" },
  { name: "Node Modules", group: "Development", safety: "moderate", description: "Orphaned node_modules in old projects" },
  { name: "Duplicate Files", group: "Storage", safety: "risky", description: "Files with identical content" },
  { name: "Orphaned Launch Agents", group: "System Junk", safety: "moderate", description: "Launch agents pointing to non-existent applications" },
];

const SAFETY_SECTIONS: { level: SafetyLevel; title: string; blurb: string }[] = [
  { level: "safe", title: "Safe to clean", blurb: "Regenerated automatically — remove anytime with no downside." },
  { level: "moderate", title: "Review recommended", blurb: "Reclaimable, but a few apps may need to rebuild state." },
  { level: "risky", title: "Review carefully", blurb: "May contain files you actually want — inspect before deleting." },
];

function CategoryCard({ category }: { category: CategoryCopy }) {
  return (
    <div className="rounded-2xl border border-card-border bg-card p-5">
      <div className="mb-1 flex items-center justify-between gap-2">
        <h3 className="m-0 text-base font-semibold tracking-tight">{category.name}</h3>
        <span className="shrink-0 rounded-full border border-card-border px-2 py-0.5 text-xs text-muted-foreground">
          {category.group}
        </span>
      </div>
      <p className="m-0 text-sm leading-relaxed text-muted-foreground">{category.description}</p>
    </div>
  );
}

function TerminalBlock() {
  return (
    <pre className="overflow-x-auto rounded-2xl border border-card-border bg-terminal-bg p-6 text-left text-[13px] leading-relaxed text-terminal-fg">
      <code>{`# Download the CLI tarball for macOS (universal binary)
curl -L ${downloadUrl("cli", APP_VERSION)} | tar xz
./app-cleaner --help

# ...or build it yourself from source
git clone ${PRODUCT.repo}.git
cd app-cleaner
task build:cli`}</code>
    </pre>
  );
}

export default function Home() {
  return (
    <main className="mx-auto max-w-[1080px] px-6">
      <section className="pb-16 pt-24 text-center">
        <span className="inline-block rounded-full border border-card-border bg-card px-4 py-1.5 text-sm text-muted-foreground">
          Native macOS app · open source · no account, no paywall
        </span>
        <h1 className="mx-auto mb-4 mt-7 text-[clamp(48px,9vw,84px)] font-bold leading-[1.02] tracking-tight">
          {PRODUCT.displayName}
        </h1>
        <p className="mx-auto mb-9 max-w-[640px] text-xl text-muted-foreground">
          Reclaim disk space on your Mac. Scan 16 categories of junk, review every file before it
          goes, and undo any cleanup from a full backup — all from a native app or the terminal.
        </p>
        <div className="mb-5 flex flex-wrap items-center justify-center gap-3">
          <a
            data-testid="download-gui"
            className="inline-flex h-12 cursor-pointer items-center justify-center gap-2 rounded-xl bg-primary px-7 text-[17px] font-semibold text-primary-foreground no-underline transition-opacity hover:opacity-90"
            href={downloadUrl("gui", APP_VERSION)}
          >
            Download for macOS (.dmg)
          </a>
          <a
            className="inline-flex h-12 cursor-pointer items-center justify-center gap-2 rounded-xl border border-card-border px-7 text-[17px] font-semibold no-underline transition-colors hover:bg-card"
            href={latestReleaseUrl()}
          >
            All releases
          </a>
        </div>
        <p className="m-0 text-sm text-muted-foreground">macOS 13+ (Apple Silicon &amp; Intel) · signed &amp; notarized</p>
      </section>

      <section className="border-t border-card-border py-14" id="scan-categories">
        <h2 className="m-0 mb-2 text-center text-[clamp(28px,4vw,40px)] font-bold tracking-tight">
          16 scan categories, ranked by safety
        </h2>
        <p className="mx-auto mb-10 max-w-[640px] text-center text-muted-foreground">
          Every category shows what it finds and how risky it is to remove before you touch
          anything.
        </p>
        {SAFETY_SECTIONS.map((section) => (
          <div className="mb-10" key={section.level}>
            <h3 className="m-0 mb-1 text-lg font-semibold">{section.title}</h3>
            <p className="m-0 mb-4 text-sm text-muted-foreground">{section.blurb}</p>
            <div className="grid grid-cols-[repeat(auto-fit,minmax(240px,1fr))] gap-4">
              {CATEGORIES.filter((c) => c.safety === section.level).map((c) => (
                <CategoryCard category={c} key={c.name} />
              ))}
            </div>
          </div>
        ))}
      </section>

      <section className="border-t border-card-border py-14">
        <h2 className="m-0 mb-10 text-center text-[clamp(28px,4vw,40px)] font-bold tracking-tight">
          More than a junk scanner
        </h2>
        <div className="grid grid-cols-[repeat(auto-fit,minmax(240px,1fr))] gap-5">
          <div className="rounded-2xl border border-card-border bg-card p-6">
            <h3 className="m-0 mb-2 text-lg font-semibold">Undo with backups</h3>
            <p className="m-0 text-sm text-muted-foreground">
              Every cleanup can back up what it removes first. Restore any item from the Backups
              view — nothing is gone for good by accident.
            </p>
          </div>
          <div className="rounded-2xl border border-card-border bg-card p-6">
            <h3 className="m-0 mb-2 text-lg font-semibold">App uninstaller</h3>
            <p className="m-0 text-sm text-muted-foreground">
              Remove an app and its leftover caches, preferences, and support files together,
              with a running-app guard so you never uninstall from under yourself.
            </p>
          </div>
          <div className="rounded-2xl border border-card-border bg-card p-6">
            <h3 className="m-0 mb-2 text-lg font-semibold">Maintenance tasks</h3>
            <p className="m-0 text-sm text-muted-foreground">
              Flush DNS cache, free purgeable disk space, and clear local Time Machine
              snapshots in one click.
            </p>
          </div>
          <div className="rounded-2xl border border-card-border bg-card p-6">
            <h3 className="m-0 mb-2 text-lg font-semibold">Native &amp; open source</h3>
            <p className="m-0 text-sm text-muted-foreground">
              Built with Go &amp; Wails — no Electron, no telemetry. Read every line on GitHub.
            </p>
          </div>
        </div>
      </section>

      <section className="border-t border-card-border py-14" id="cli">
        <h2 className="m-0 mb-2 text-center text-[clamp(28px,4vw,40px)] font-bold tracking-tight">
          Prefer the terminal?
        </h2>
        <p className="mx-auto mb-8 max-w-[640px] text-center text-muted-foreground">
          App Cleaner ships a full-parity terminal CLI with the same interactive picker,
          backups, and maintenance tasks as the GUI — built on the same engine.
        </p>
        <TerminalBlock />
      </section>

      <footer className="border-t border-card-border py-12 pb-16 text-center text-muted-foreground">
        <p className="m-0 mb-2">
          <a className="text-primary no-underline hover:underline" href={PRODUCT.repo}>
            Source on GitHub
          </a>{" "}
          · MIT license
        </p>
        <p className="m-0 text-sm opacity-80">
          A native macOS port of{" "}
          <a className="text-primary no-underline hover:underline" href="https://github.com/gabrielmaialva33/mac-cleaner-cli">
            mac-cleaner-cli
          </a>
          .
        </p>
      </footer>
    </main>
  );
}
