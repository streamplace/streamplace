import { Link } from "@tanstack/react-router";
import { ChevronLeft } from "lucide-react";

interface BackLinkProps {
  to: string;
  label: string;
}

function BackLink({ to, label }: BackLinkProps) {
  return (
    <Link
      to={to}
      className="inline-flex items-center gap-1.5 text-sm text-[var(--color-fg-muted)] transition-colors hover:text-[var(--color-fg)]"
    >
      <ChevronLeft size={16} />
      {label}
    </Link>
  );
}

export { BackLink };
