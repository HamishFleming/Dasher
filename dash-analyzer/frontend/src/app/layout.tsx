import type { Metadata } from "next";
import { Providers } from "@/components/providers";
import { Shell } from "@/components/shell";
import "leaflet/dist/leaflet.css";
import "./globals.css";
export const metadata: Metadata = {
  title: { default: "Explore · Dash Analyzer", template: "%s · Dash Analyzer" },
  description:
    "Local Melbourne delivery planning, venue observations and a private delivery ledger.",
};
export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en-AU">
      <body>
        <a href="#main-content" className="skip-link">
          Skip to content
        </a>
        <Providers>
          <Shell>{children}</Shell>
        </Providers>
      </body>
    </html>
  );
}
