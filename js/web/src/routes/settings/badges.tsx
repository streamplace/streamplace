import { BackLink } from "@/components/settings/back-link";
import { createFileRoute } from "@tanstack/react-router";
import { Check } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { usePDSAgent } from "../../lib/store/hooks";

export const Route = createFileRoute("/settings/badges")({
  component: BadgeSelectionManager,
});

interface BadgeIssuanceView {
  issuanceUri: string;
  issuanceCid?: string;
  badgeType: string;
  name?: string;
  description?: string;
  imageUrl?: string;
  issuer: string;
  selected?: boolean;
}

interface BadgeSlot {
  available: BadgeIssuanceView[];
  selected?: BadgeIssuanceView;
}

function BadgeSelectionManager() {
  const { t } = useTranslation("settings");
  const agent = usePDSAgent();

  const [loading, setLoading] = useState(true);
  const [streamerSlot, setStreamerSlot] = useState<BadgeSlot | null>(null);
  const [userSlot, setUserSlot] = useState<BadgeSlot | null>(null);
  const [toggling, setToggling] = useState<string | null>(null);
  const togglingRef = useRef<string | null>(null);

  const load = useCallback(async () => {
    if (!agent) return;
    try {
      setLoading(true);
      const res = await agent.place.stream.badge.getIssuedBadges({});
      setStreamerSlot(res.data.streamer as BadgeSlot | null);
      setUserSlot(res.data.user as BadgeSlot | null);
    } catch (error: any) {
      toast.error(error.message || "Failed to load badges");
    } finally {
      setLoading(false);
    }
  }, [agent]);

  useEffect(() => {
    load();
  }, [load]);

  const handleToggle = useCallback(
    async (badge: BadgeIssuanceView, slot: "streamer" | "global") => {
      if (!agent?.did || togglingRef.current) return;
      togglingRef.current = badge.issuanceUri;
      setToggling(badge.issuanceUri);

      try {
        let currentRecord: Record<string, any> = {
          $type: "place.stream.chat.profile",
        };
        let swapCid: string | undefined;

        try {
          const getRes = await agent.com.atproto.repo.getRecord({
            repo: agent.did,
            collection: "place.stream.chat.profile",
            rkey: "self",
          });
          currentRecord = getRes.data.value as Record<string, any>;
          swapCid = getRes.data.cid;
        } catch {
          // no profile yet
        }

        const currentBadges: Record<string, any> =
          (currentRecord.badges as Record<string, any>) ?? {};
        const isCurrentlySelected = badge.selected ?? false;
        const ref = { uri: badge.issuanceUri, cid: badge.issuanceCid ?? "" };

        let newBadges: Record<string, any>;
        if (slot === "streamer") {
          const currentStreamer: Array<{
            streamer: string;
            badge: { uri: string; cid: string };
          }> = (currentBadges.streamer as any[]) ?? [];
          newBadges = {
            ...currentBadges,
            streamer: isCurrentlySelected
              ? currentStreamer.filter((s) => s.badge.uri !== badge.issuanceUri)
              : [
                  ...currentStreamer.filter((s) => s.streamer !== badge.issuer),
                  { streamer: badge.issuer, badge: ref },
                ],
          };
        } else {
          newBadges = {
            ...currentBadges,
            global: isCurrentlySelected ? undefined : ref,
          };
        }

        await agent.com.atproto.repo.putRecord({
          repo: agent.did,
          collection: "place.stream.chat.profile",
          rkey: "self",
          record: { ...currentRecord, badges: newBadges },
          swapRecord: swapCid,
        });

        // Optimistic update
        if (slot === "streamer") {
          setStreamerSlot((prev) => {
            if (!prev) return prev;
            const available = prev.available.map((b) =>
              b.issuanceUri === badge.issuanceUri
                ? { ...b, selected: !isCurrentlySelected }
                : b.issuer === badge.issuer
                  ? { ...b, selected: false }
                  : b,
            );
            return {
              available,
              selected: isCurrentlySelected
                ? undefined
                : available.find((b) => b.issuanceUri === badge.issuanceUri),
            };
          });
        } else {
          setUserSlot((prev) => {
            if (!prev) return prev;
            const available = prev.available.map((b) =>
              b.issuanceUri === badge.issuanceUri
                ? { ...b, selected: !isCurrentlySelected }
                : { ...b, selected: false },
            );
            return {
              available,
              selected: isCurrentlySelected
                ? undefined
                : available.find((b) => b.issuanceUri === badge.issuanceUri),
            };
          });
        }
      } catch (error: any) {
        toast.error(error.message || "Failed to update badge");
      } finally {
        togglingRef.current = null;
        setToggling(null);
      }
    },
    [agent],
  );

  const hasStreamerBadges = (streamerSlot?.available?.length ?? 0) > 0;
  const hasUserBadges = (userSlot?.available?.length ?? 0) > 0;

  return (
    <div className="space-y-6">
      <BackLink to="/settings/account" label={t("account")} />

      <h1 className="text-xl font-semibold">{t("badges")}</h1>

      {loading ? (
        <div className="text-sm text-[var(--color-fg-muted)]">Loading…</div>
      ) : !hasStreamerBadges && !hasUserBadges ? (
        <div className="text-center py-8 text-sm text-[var(--color-fg-muted)]">
          {t("badges-empty-state")}
        </div>
      ) : (
        <div className="space-y-6">
          {hasStreamerBadges && (
            <section>
              <h2 className="text-sm font-medium uppercase tracking-wide text-[var(--color-fg-muted)] mb-2">
                {t("badges-streamer-section")}
              </h2>
              <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] divide-y divide-[var(--color-border)]">
                {streamerSlot!.available.map((badge) => (
                  <BadgeRow
                    key={badge.issuanceUri}
                    badge={badge}
                    onToggle={() => handleToggle(badge, "streamer")}
                    toggling={toggling === badge.issuanceUri}
                  />
                ))}
              </div>
            </section>
          )}

          {hasUserBadges && (
            <section>
              <h2 className="text-sm font-medium uppercase tracking-wide text-[var(--color-fg-muted)] mb-2">
                {t("badges-cosmetic-section")}
              </h2>
              <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] divide-y divide-[var(--color-border)]">
                {userSlot!.available.map((badge) => (
                  <BadgeRow
                    key={badge.issuanceUri}
                    badge={badge}
                    onToggle={() => handleToggle(badge, "global")}
                    toggling={toggling === badge.issuanceUri}
                  />
                ))}
              </div>
            </section>
          )}
        </div>
      )}
    </div>
  );
}

function BadgeRow({
  badge,
  onToggle,
  toggling,
}: {
  badge: BadgeIssuanceView;
  onToggle: () => void;
  toggling: boolean;
}) {
  const { t } = useTranslation("settings");
  const isSelected = badge.selected ?? false;
  const badgeName = badge.name ?? badge.badgeType.split("#")[1];

  return (
    <button
      type="button"
      onClick={onToggle}
      disabled={toggling}
      className="flex items-center gap-3 w-full px-3 py-2.5 hover:bg-[var(--color-bg)] transition-colors text-left disabled:opacity-50"
    >
      {badge.imageUrl ? (
        <img src={badge.imageUrl} alt="" className="w-6 h-6 rounded" />
      ) : (
        <div className="w-6 h-6 rounded bg-[var(--color-bg)]" />
      )}

      <div className="flex-1 min-w-0">
        <div className="text-sm font-medium">{badgeName}</div>
        {badge.description && (
          <div className="text-xs text-[var(--color-fg-muted)] truncate">
            {badge.description}
          </div>
        )}
        <div className="text-xs text-[var(--color-fg-muted)]">
          {t("badges-issued-by", { issuer: badge.issuer })}
        </div>
      </div>

      {toggling ? (
        <div className="w-5 h-5 rounded-full border-2 border-[var(--color-accent)] border-t-transparent animate-spin" />
      ) : isSelected ? (
        <div className="w-5 h-5 rounded-full bg-[var(--color-accent)] flex items-center justify-center">
          <Check size={12} className="text-white" />
        </div>
      ) : (
        <div className="w-5 h-5 rounded-full border-2 border-[var(--color-border)]" />
      )}
    </button>
  );
}
