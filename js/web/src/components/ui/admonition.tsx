import { cn } from "@/lib/utils";
import {
  ExternalLink,
  LucideAlertOctagon,
  LucideAlertTriangle,
  LucideInfo,
  LucideLightbulb,
  LucideNotebookPen,
} from "lucide-react";
import { createContext, useContext } from "react";
import { Card } from "./card";

const AdmonitionContext = createContext<{
  type: string;
  size: "sm" | "md" | "lg";
} | null>(null);

function useAdmonitionContext() {
  const ctx = useContext(AdmonitionContext);
  if (!ctx) {
    throw new Error(
      "Admonition subcomponents must be used within an <Admonition>",
    );
  }
  return ctx;
}

const ADMONITION_SIZES = {
  sm: "text-sm",
  md: "text-base",
  lg: "text-lg",
} as const;

// Admonition component based off Card
function Admonition({
  type,
  children,
  size = "md",
}: {
  type: string;
  children: React.ReactNode;
  size?: "sm" | "md" | "lg";
}) {
  const sizes = {
    sm: "py-2 px-3",
    md: "py-2 px-3",
    lg: "py-4 px-6",
  };
  const colors = {
    note: "bg-blue-50 text-blue-800 dark:bg-blue-400/20 dark:text-blue-100 border-blue-400/40",
    tip: "bg-green-50 text-green-800 dark:bg-green-400/20 dark:text-green-100 border-green-400/40",
    info: "bg-blue-50 text-blue-800 dark:bg-blue-400/20 dark:text-blue-100 border-blue-400/40",
    warning:
      "bg-yellow-50 text-yellow-800 dark:bg-yellow-400/20 dark:text-yellow-100 border-yellow-400/40",
    danger:
      "bg-red-50 text-red-800 dark:bg-red-900/20 dark:text-red-100 border-red-400/40",
  } as const;

  const iconSize = {
    sm: "size-10 -mt-2",
    md: "size-14 -mt-3",
    lg: "size-18 -mt-4",
  };

  const Icon =
    {
      note: LucideNotebookPen,
      tip: LucideLightbulb,
      info: LucideInfo,
      warning: LucideAlertTriangle,
      danger: LucideAlertOctagon,
    }[type as keyof typeof colors] || LucideInfo;

  return (
    <AdmonitionContext.Provider value={{ type, size }}>
      <Card
        className={`flex flex-col items-start ${colors[type as keyof typeof colors] || colors.info} border ${sizes[size]}`}
      >
        <div className="flex items-start gap-2">
          <Icon className={cn(iconSize[size])} />
          <div>{children}</div>
        </div>
      </Card>
    </AdmonitionContext.Provider>
  );
}

function AdmonitionTitle({ children }: { children: React.ReactNode }) {
  const ctx = useAdmonitionContext();
  return (
    <h3 className={cn("text-base font-semibold", ADMONITION_SIZES[ctx.size])}>
      {children}
    </h3>
  );
}

function AdmonitionDescription({ children }: { children: React.ReactNode }) {
  const ctx = useAdmonitionContext();
  const size = {
    sm: "text-sm",
    md: "text-sm",
    lg: "text-base",
  }[ctx.size];
  const colors = {
    note: "text-blue-800/80 dark:text-blue-100/80",
    tip: "text-green-800/80 dark:text-green-100/80",
    info: "text-blue-800/80 dark:text-blue-100/80",
    warning: "text-yellow-800/80 dark:text-yellow-100/80",
    danger: "text-red-800/80 dark:text-red-100/80",
  } as const;
  return <p className={cn(colors[ctx.type], size)}>{children}</p>;
}

function AdmonitionLink({
  children,
  href,
  external = true,
}: {
  children: React.ReactNode;
  href: string;
  external?: boolean;
}) {
  const ctx = useAdmonitionContext();
  const size = {
    sm: "text-sm",
    md: "text-sm",
    lg: "text-base",
  }[ctx.size];
  const iconSize = {
    sm: "size-4 mb-1",
    md: "size-4 mb-1",
    lg: "size-6 mb-1.5",
  }[ctx.size];
  const colors = {
    note: "text-blue-700/80 dark:text-blue-200/80",
    tip: "text-green-700/80 dark:text-green-200/80",
    info: "text-blue-700/80 dark:text-blue-200/80",
    warning: "text-yellow-700/80 dark:text-yellow-200/80",
    danger: "text-red-700/80 dark:text-red-200/80",
  } as const;
  return (
    <a
      href={href}
      target={external ? "_blank" : undefined}
      rel={external ? "noopener noreferrer" : undefined}
      className={cn(
        "hover:underline font-medium",
        colors[ctx.type as keyof typeof colors],
        size,
      )}
    >
      {children} <ExternalLink className={cn("inline", iconSize)} />
    </a>
  );
}

Admonition.Link = AdmonitionLink;

Admonition.Description = AdmonitionDescription;

Admonition.Title = AdmonitionTitle;

export { Admonition };
