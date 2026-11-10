import { AlertCircle, ArrowUpRight, type LucideIcon } from "lucide-react";
export function PageHeading({
  eyebrow,
  title,
  description,
  children,
}: {
  eyebrow: string;
  title: string;
  description: string;
  children?: React.ReactNode;
}) {
  return (
    <div className="page-heading">
      <div>
        <div className="eyebrow">{eyebrow}</div>
        <h1>
          {title}
          <span className="title-dot">.</span>
        </h1>
        <p>{description}</p>
      </div>
      <div className="heading-actions">{children}</div>
    </div>
  );
}
export function ErrorNotice({ message }: { message: string }) {
  return message ? (
    <div className="error-notice" role="alert">
      <AlertCircle size={16} />
      {message}
    </div>
  ) : null;
}
export function Stat({
  label,
  value,
  detail,
  icon: Icon,
}: {
  label: string;
  value: string;
  detail: string;
  icon: LucideIcon;
}) {
  return (
    <div className="stat">
      <div className="stat-top">
        {label}
        <Icon size={17} />
      </div>
      <div className="stat-value">{value}</div>
      <div className="stat-detail">{detail}</div>
    </div>
  );
}
export function Empty({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="empty">
      <ArrowUpRight size={25} />
      <h3>{title}</h3>
      <p>{children}</p>
    </div>
  );
}
