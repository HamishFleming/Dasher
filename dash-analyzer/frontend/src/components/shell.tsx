"use client";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  ArrowUpRight,
  Map,
  Layers3,
  ReceiptText,
  SlidersHorizontal,
  Navigation,
  MapPin,
  ShieldCheck,
} from "lucide-react";
import { useSettings } from "./providers";
const links = [
  { href: "/", label: "Explore", icon: Map },
  { href: "/collection", label: "Venue intelligence", icon: Layers3 },
  { href: "/history", label: "Delivery ledger", icon: ReceiptText },
  { href: "/settings", label: "Scoring settings", icon: SlidersHorizontal },
];
export function Shell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { settings, ready } = useSettings();
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <Link className="brand" href="/">
          <span className="brand-mark">
            <Navigation size={22} fill="currentColor" />
          </span>
          <span>
            dash<span className="brand-light"> / analyzer</span>
          </span>
        </Link>
        <div className="sidebar-caption">YOUR LOCAL DRIVING COMPANION</div>
        <nav aria-label="Main navigation">
          {links.map(({ href, label, icon: Icon }) => (
            <Link
              key={href}
              href={href}
              className={"nav-link " + (pathname === href ? "active" : "")}
              aria-current={pathname === href ? "page" : undefined}
            >
              <Icon size={18} />
              {label}
              {pathname === href && <span className="nav-dot" />}
            </Link>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="local-badge">
            <ShieldCheck size={17} />
            <span>Local by design</span>
          </div>
          <p>Your delivery history stays on this machine.</p>
          <a
            href="https://www.openstreetmap.org/copyright"
            target="_blank"
            rel="noreferrer"
          >
            OpenStreetMap contributors <ArrowUpRight size={13} />
          </a>
        </div>
      </aside>
      <div className="workspace">
        <header className="topbar">
          <span className="breadcrumb">
            Workspace <span>/</span>{" "}
            {links.find((l) => l.href === pathname)?.label || "Dashboard"}
          </span>
          <Link href="/settings" className="location-pill">
            <MapPin size={14} />
            <span className="location-coordinates">
              {ready
                ? `${settings.lat.toFixed(4)}, ${settings.lon.toFixed(4)}`
                : "Melbourne"}
            </span>
            <span className="pill-divider" />
            <span>Melbourne</span>
          </Link>
        </header>
        <main id="main-content">{children}</main>
        <footer className="app-footer">
          <span>
            Planning estimates, never live DoorDash demand or predicted
            earnings.
          </span>
          <span>Melbourne time · AUD · km</span>
        </footer>
      </div>
    </div>
  );
}
