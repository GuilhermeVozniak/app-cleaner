export const PRODUCT = {
  name: "app-cleaner",
  displayName: "App Cleaner",
  repo: "https://github.com/GuilhermeVozniak/app-cleaner",
  site: "https://app-cleaner.vozniak.dev",
} as const;

export type AssetKind = "gui" | "cli";

const ASSET_NAME_BUILDERS: Record<AssetKind, (version: string) => string> = {
  gui: (version) => `app-cleaner_${version}_darwin_universal.dmg`,
  cli: (version) => `app-cleaner-cli_${version}_darwin_universal.tar.gz`,
};

export function releaseAssetName(kind: AssetKind, version: string): string {
  return ASSET_NAME_BUILDERS[kind](version);
}

export function downloadUrl(kind: AssetKind, version: string): string {
  return `${PRODUCT.repo}/releases/download/v${version}/${releaseAssetName(kind, version)}`;
}

export function latestReleaseUrl(): string {
  return `${PRODUCT.repo}/releases/latest`;
}
