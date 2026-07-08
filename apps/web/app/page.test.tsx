import { downloadUrl, latestReleaseUrl } from "@app-cleaner/shared";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { APP_VERSION } from "../lib/version";
import Home from "./page";

describe("Home", () => {
  it("renders the product name as the main heading", () => {
    render(<Home />);
    expect(screen.getByRole("heading", { level: 1, name: "App Cleaner" })).toBeInTheDocument();
  });

  it("links the primary download button to the shared gui asset URL", () => {
    render(<Home />);
    expect(screen.getByTestId("download-gui")).toHaveAttribute(
      "href",
      downloadUrl("gui", APP_VERSION),
    );
  });

  it("links 'All releases' to the shared latest-release URL", () => {
    render(<Home />);
    expect(screen.getByRole("link", { name: "All releases" })).toHaveAttribute(
      "href",
      latestReleaseUrl(),
    );
  });

  it("renders all 16 scan categories", () => {
    render(<Home />);
    expect(screen.getAllByRole("heading", { level: 3 }).length).toBeGreaterThanOrEqual(16);
  });
});
