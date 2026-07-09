import { PRODUCT } from "@app-cleaner/shared";
import type { Metadata } from "next";
import type { ReactNode } from "react";
import "./globals.css";

const title = "App Cleaner — reclaim disk space on your Mac";
const description =
  "A native, open-source Mac cleaner: 16 scan categories, undoable backups, an app uninstaller, and maintenance tools. Also available as a full-parity terminal CLI.";

export const metadata: Metadata = {
  metadataBase: new URL(PRODUCT.site),
  title,
  description,
  alternates: { canonical: "/" },
  openGraph: {
    type: "website",
    url: PRODUCT.site,
    siteName: PRODUCT.displayName,
    title,
    description,
  },
  twitter: {
    card: "summary_large_image",
    title,
    description,
  },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
