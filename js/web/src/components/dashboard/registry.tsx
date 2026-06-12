import type { LivestreamStore } from "@streamplace/core";
import {
  Activity,
  AlertTriangle,
  Globe,
  MessageSquare,
  Radio,
  SquareDashed,
  Video,
  type LucideIcon,
} from "lucide-react";
import type { ComponentType } from "react";
import { useTranslation } from "react-i18next";
import { ChatPanelWidget } from "./chat-panel";
import { MultistreamStatusWidget } from "./multistream-status";
import { ProblemsWidget } from "./problems";
import { StreamHealthWidget } from "./stream-health";
import { StreamInfoWidget } from "./stream-info";
import { StreamMonitorWidget } from "./stream-monitor";

export interface WidgetMeta {
  component: ComponentType<{ store: LivestreamStore; user?: string }>;
  title: string;
  icon: LucideIcon;
  color: string;
  minWidth: number;
  minHeight: number;
}

/** Placeholder shown for empty slots after deleting a widget. */
function BlankWidget() {
  const { t } = useTranslation("settings");
  return (
    <div className="m-2 flex h-full items-center justify-center rounded-md border-2 border-dashed border-[var(--color-border)] text-xs text-[var(--color-fg-muted)]">
      {t("drop-widget-here", { defaultValue: "Drop a widget here" })}
    </div>
  );
}

export const WIDGET_REGISTRY: Record<string, WidgetMeta> = {
  blank: {
    component: BlankWidget,
    title: "Empty",
    icon: SquareDashed,
    color: "#6b7280",
    minWidth: 120,
    minHeight: 80,
  },
  "stream-monitor": {
    component: StreamMonitorWidget,
    title: "Stream Monitor",
    icon: Video,
    color: "#3b82f6",
    minWidth: 320,
    minHeight: 180,
  },
  "stream-health": {
    component: StreamHealthWidget,
    title: "Stream Health",
    icon: Activity,
    color: "#22c55e",
    minWidth: 240,
    minHeight: 200,
  },
  chat: {
    component: ChatPanelWidget,
    title: "Chat",
    icon: MessageSquare,
    color: "#ec4899",
    minWidth: 280,
    minHeight: 300,
  },
  "stream-info": {
    component: StreamInfoWidget,
    title: "Stream Info",
    icon: Radio,
    color: "#f97316",
    minWidth: 280,
    minHeight: 400,
  },
  multistream: {
    component: MultistreamStatusWidget,
    title: "Multistream",
    icon: Globe,
    color: "#06b6d4",
    minWidth: 240,
    minHeight: 120,
  },
  problems: {
    component: ProblemsWidget,
    title: "Problems",
    icon: AlertTriangle,
    color: "#f59e0b",
    minWidth: 240,
    minHeight: 100,
  },
};

export const WIDGET_KEYS = Object.keys(WIDGET_REGISTRY);

export function getWidgetMeta(key: string): WidgetMeta | undefined {
  return WIDGET_REGISTRY[key];
}
