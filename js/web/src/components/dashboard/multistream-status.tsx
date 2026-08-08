import { cn } from "@/lib/utils";
import type { LivestreamStore } from "@streamplace/core";
import { Globe, Loader2 } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { place } from "streamplace";
import { usePDSAgent } from "../../lib/store/hooks";

interface MultistreamTarget {
  uri: string;
  record: {
    [x: string]: unknown;
    name?: string;
    url?: string;
    active: boolean;
    createdAt: string;
  };
  latestEvent?: {
    status: string;
  };
}

function getTargetName(target: MultistreamTarget): string {
  if (target.record.name) return target.record.name;
  if (target.record.url) {
    try {
      return new URL(target.record.url).host;
    } catch {
      return "Untitled Target";
    }
  }
  return "Untitled Target";
}

function getTargetHostname(target: MultistreamTarget): string | null {
  if (!target.record.url) return null;
  try {
    return new URL(target.record.url).host.split(":")[0];
  } catch {
    return null;
  }
}

function statusColor(target: MultistreamTarget): string {
  if (!target.record.active) return "text-(--color-fg-muted)";
  switch (target.latestEvent?.status) {
    case "active":
      return "text-green-400";
    case "error":
      return "text-red-400";
    case "pending":
      return "text-amber-400 animate-pulse";
    default:
      return "text-(--color-fg-muted)";
  }
}

/**
 * Lists multistream targets with active/inactive toggles and connection
 * status.
 */
export function MultistreamStatusWidget({ store }: { store: LivestreamStore }) {
  const { t } = useTranslation("common");
  const agent = usePDSAgent();
  const [targets, setTargets] = useState<MultistreamTarget[]>([]);
  const [loading, setLoading] = useState(true);
  const [toggling, setToggling] = useState<Set<string>>(new Set());

  const loadTargets = useCallback(async () => {
    if (!agent) return;
    try {
      setLoading(true);
      const response = await agent.client.call(
        place.stream.multistream.listTargets,
        {
          limit: 50,
        },
      );
      setTargets(response.targets as unknown as MultistreamTarget[]);
    } catch (error) {
      console.error("Failed to load multistream targets:", error);
      setTargets([]);
    } finally {
      setLoading(false);
    }
  }, [agent]);

  const toggleTarget = useCallback(
    async (target: MultistreamTarget, newActive: boolean) => {
      if (!agent) return;
      try {
        setToggling((prev) => new Set(prev).add(target.uri));
        await agent.client.call(place.stream.multistream.putTarget, {
          multistreamTarget: {
            ...target.record,
            $type: "place.stream.multistream.target" as const,
            active: newActive,
          } as any,
          rkey: target.uri.split("/").pop() || "",
        });
        await loadTargets();
      } catch (error) {
        console.error("Failed to toggle multistream target:", error);
      } finally {
        setToggling((prev) => {
          const next = new Set(prev);
          next.delete(target.uri);
          return next;
        });
      }
    },
    [agent, loadTargets],
  );

  useEffect(() => {
    loadTargets();
  }, [loadTargets]);

  if (loading && targets.length === 0) {
    return (
      <div className="rounded-lg border border-(--color-border) bg-(--color-bg-elevated) p-4">
        <div className="flex items-center gap-2 text-sm text-(--color-fg-muted)">
          <Loader2 className="size-4 animate-spin" />
          {t("loading-multistream", {
            defaultValue: "Loading multistream…",
          })}
        </div>
      </div>
    );
  }

  if (targets.length === 0) {
    return (
      <div className="rounded-lg border border-(--color-border) bg-(--color-bg-elevated) p-4">
        <div className="flex items-center gap-2 text-sm text-(--color-fg-muted)">
          <Globe className="size-4" />
          {t("no-multistream-targets", {
            defaultValue: "No multistream targets configured",
          })}
        </div>
      </div>
    );
  }

  return (
    <div className="h-full overflow-hidden rounded-lg border border-(--color-border) bg-(--color-bg-elevated)">
      <div className="flex items-center gap-2 border-b border-(--color-border) px-4 py-3">
        <Globe className="size-4 text-(--color-fg-muted)" />
        <h3 className="text-sm font-semibold">
          {t("multistream", { defaultValue: "Multistream" })}
        </h3>
        {loading && (
          <Loader2 className="size-3 animate-spin text-(--color-fg-muted)" />
        )}
      </div>

      <div className="divide-y divide-(--color-border)">
        {targets.map((target) => {
          const name = getTargetName(target);
          const hostname = getTargetHostname(target);
          const isToggling = toggling.has(target.uri);

          return (
            <div
              key={target.uri}
              className="flex items-center justify-between px-4 py-2.5"
            >
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="truncate text-sm">{name}</span>
                  {target.record.name && hostname && (
                    <span className="truncate text-xs text-(--color-fg-muted)">
                      {hostname}
                    </span>
                  )}
                </div>
                {target.latestEvent && (
                  <div
                    className={cn(
                      "text-[11px] capitalize",
                      statusColor(target),
                    )}
                  >
                    {target.latestEvent.status}
                  </div>
                )}
              </div>

              <button
                type="button"
                role="switch"
                aria-checked={target.record.active}
                disabled={isToggling}
                onClick={() => toggleTarget(target, !target.record.active)}
                className={cn(
                  "relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors",
                  "focus-visible:ring-ring focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none",
                  "disabled:cursor-not-allowed disabled:opacity-50",
                  target.record.active
                    ? "bg-(--color-accent)"
                    : "bg-(--color-bg)",
                )}
              >
                <span
                  className={cn(
                    "pointer-events-none inline-block size-4 rounded-full bg-white shadow-sm ring-0 transition-transform",
                    target.record.active ? "translate-x-4" : "translate-x-0",
                  )}
                />
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );
}
