import { describe, expect, it } from "vitest";
import { downloadUrl, latestReleaseUrl, PRODUCT, releaseAssetName } from "./index";

describe("releaseAssetName", () => {
  it("builds the exact gui asset name for version 1.0.0", () => {
    expect(releaseAssetName("gui", "1.0.0")).toBe("app-cleaner_1.0.0_darwin_universal.dmg");
  });

  it("builds the exact cli asset name for version 1.0.0", () => {
    expect(releaseAssetName("cli", "1.0.0")).toBe(
      "app-cleaner-cli_1.0.0_darwin_universal.tar.gz",
    );
  });
});

describe("downloadUrl", () => {
  it("builds a tagged gui release asset URL", () => {
    expect(downloadUrl("gui", "1.0.0")).toBe(
      `${PRODUCT.repo}/releases/download/v1.0.0/app-cleaner_1.0.0_darwin_universal.dmg`,
    );
  });

  it("builds a tagged cli release asset URL", () => {
    expect(downloadUrl("cli", "1.0.0")).toBe(
      `${PRODUCT.repo}/releases/download/v1.0.0/app-cleaner-cli_1.0.0_darwin_universal.tar.gz`,
    );
  });
});

describe("latestReleaseUrl", () => {
  it("points at the latest release page", () => {
    expect(latestReleaseUrl()).toBe(`${PRODUCT.repo}/releases/latest`);
  });
});
