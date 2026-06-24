import type { LivestreamStore } from "@streamplace/core";
import { Link } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore as useZustandStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import useAvatars from "../../hooks/use-avatars";
import { captureError } from "../../lib/log";
import { useStore } from "../../lib/store";
import { getStreamplaceUrl } from "../../lib/streamplace-url";
import { Player } from "../player/player";

// Replaces the player when the stream is offline. Renders the
// streamer's banner as a blurred background with an offline badge,
// avatar, and handle on top. Sits in the same DOM position as the
// Player (absolute inset-0 inside the VideoSection container) so the
// transition between live and offline is just an opacity swap.
//
// When the stream has been live before, fetches a recommendation via
// the user's PDS and renders a "watch them instead" card linking to
// the recommended streamer's page. The recommendation is shown only
// after the fetch settles, so the user always sees the OFFLINE
// state immediately.
export function PlayerOffline({
  store,
  user,
}: {
  store: LivestreamStore;
  user: string;
}) {
  const { t } = useTranslation("common");
  const profile = useZustandStore(store, (s) => s.profile);
  // Subscribe to the app's agent fields so the effect re-runs when
  // loadOAuthClient finishes setting up the anon (or authed) agent.
  // Without this, a fast-firing WebSocket profile delivery can race
  // the OAuth client init and we end up stuck with no agent.
  const { pdsAgent, anonPDSAgent } = useStore(
    useShallow((s) => ({
      pdsAgent: s.pdsAgent,
      anonPDSAgent: s.anonPDSAgent,
    })),
  );
  const avatars = useAvatars(profile?.did ? [profile.did] : []);
  const detailed = profile?.did ? avatars[profile.did] : null;
  const banner = detailed?.banner;
  const avatar = detailed?.avatar || profile?.avatar;
  const handle = profile?.handle || user;

  const [recommendation, setRecommendation] = useState<{
    did: string;
    source: string;
  } | null>(null);

  useEffect(() => {
    if (!profile?.did) return;
    if (!pdsAgent && !anonPDSAgent) return;
    const getRecommendations = useStore.getState().getRecommendations;
    let mounted = true;
    const fetchRec = async () => {
      try {
        const result = await getRecommendations(profile.did);
        if (!mounted) return;
        const first = result.recommendations?.find(
          (r) =>
            r.$type ===
              "place.stream.live.getRecommendations#livestreamRecommendation" &&
            (r as { did?: string }).did,
        ) as { did?: string; source?: string } | undefined;
        if (first?.did) {
          setRecommendation({ did: first.did, source: first.source ?? "" });
        }
      } catch {
        // Silent: recommendations are best-effort. The OFFLINE state
        // is still useful on its own.
      }
    };
    fetchRec();
    return () => {
      mounted = false;
    };
  }, [profile?.did, pdsAgent, anonPDSAgent]);

  // Look up the recommended streamer's profile so the card can
  // show their avatar and handle.
  const recAvatars = useAvatars(recommendation ? [recommendation.did] : []);
  const recDetailed = recommendation ? recAvatars[recommendation.did] : null;
  const recAvatar = recDetailed?.avatar;
  const recHandle = recDetailed?.handle;

  return (
    <div className="absolute inset-0 overflow-hidden">
      {banner ? (
        <div
          aria-hidden
          className="absolute inset-0 bg-cover bg-center"
          style={{
            backgroundImage: `url(${banner})`,
            filter: "blur(24px)",
            transform: "scale(1.1)",
          }}
        />
      ) : (
        <div
          aria-hidden
          className="absolute inset-0 bg-gradient-to-b from-(--color-bg-elevated) to-(--color-bg)"
        />
      )}
      <div aria-hidden className="absolute inset-0 bg-black/40" />

      <div className="relative flex h-full items-center justify-center p-4">
        {/* Under lg: stacked, 4:3 outer. Video on top, OFFLINE state
            below. */}
        <div className="flex aspect-[4/3] w-full max-w-full flex-col gap-2 lg:hidden">
          {recommendation ? (
            <div className="relative aspect-video w-full shrink-0 overflow-hidden rounded-lg border border-white/10 bg-black">
              <RecommendationEmbed
                did={recommendation.did}
                handle={recHandle}
                avatar={recAvatar}
              />
            </div>
          ) : null}
          <div className="flex min-h-0 flex-1 flex-col items-start justify-center gap-1.5 rounded-xl border border-white/10 bg-black/50 p-3 text-left">
            <span className="rounded-full border border-white/20 bg-white/10 px-2.5 py-0.5 text-[10px] font-semibold tracking-wider text-white uppercase">
              {t("offline")}
            </span>
            <p className="text-sm text-white">
              {recommendation ? (
                <>
                  Looks like <span className="font-semibold">@{handle}</span> is
                  offline, but they recommend checking out:
                </>
              ) : (
                <>
                  Looks like <span className="font-semibold">@{handle}</span> is
                  offline right now. Check back later.
                </>
              )}
            </p>
            {avatar && (
              <div className="mt-0.5 flex items-center gap-2">
                <img
                  src={avatar}
                  alt=""
                  className="h-5 w-5 rounded-full border border-white/20 object-cover"
                />
                <p className="text-xs font-medium text-white/80">@{handle}</p>
              </div>
            )}
          </div>
        </div>

        {/* lg and above: side-by-side. Left text panel 5:3, right
            video panel 16:9. */}
        <div className="hidden h-full w-full items-center justify-center gap-4 lg:flex">
          <div className="flex aspect-[5/3] h-full max-w-[50%] flex-col items-start justify-center gap-2 rounded-xl border border-white/10 bg-black/50 p-4 text-left backdrop-blur-sm">
            <span className="rounded-full border border-white/20 bg-white/10 px-2.5 py-0.5 text-[10px] font-semibold tracking-wider text-white uppercase">
              {t("offline")}
            </span>
            {recommendation ? (
              <p className="text-base text-white">
                Looks like <span className="font-semibold">@{handle}</span> is
                offline, but they recommend checking out:
              </p>
            ) : (
              <p className="text-base text-white">
                Looks like <span className="font-semibold">@{handle}</span> is
                offline right now. Check back later.
              </p>
            )}
            {avatar && (
              <div className="mt-1 flex items-center gap-2">
                <img
                  src={avatar}
                  alt=""
                  className="h-5 w-5 rounded-full border border-white/20 object-cover"
                />
                <p className="text-xs font-medium text-white/80">@{handle}</p>
              </div>
            )}
          </div>

          {recommendation ? (
            <div className="aspect-video h-full max-w-[55%]">
              <RecommendationEmbed
                did={recommendation.did}
                handle={recHandle}
                avatar={recAvatar}
              />
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}

// Embeds a small live-player preview for the recommended streamer.
// Clicking anywhere on the preview navigates to the streamer's page
// (a transparent Link overlay sits on top of the Player so the
// Player's own click-to-play/pause handler never fires).
function RecommendationEmbed({
  did,
  handle,
  avatar,
}: {
  did: string;
  handle?: string;
  avatar?: string;
}) {
  const { playlistUrl, thumbnailUrl } = useMemo(() => {
    const base = getStreamplaceUrl();
    return {
      playlistUrl: `${base}/xrpc/place.stream.playback.getLivePlaylist?streamer=${encodeURIComponent(did)}`,
      thumbnailUrl: `${base}/api/playback/${encodeURIComponent(did)}/stream.jpg`,
    };
  }, [did]);

  return (
    <div className="flex h-full flex-col items-stretch gap-2 rounded-xl border border-white/10 bg-black/50 p-2 backdrop-blur-sm">
      <div className="relative min-h-0 flex-1 overflow-hidden rounded-lg bg-black">
        <Player
          src={playlistUrl}
          poster={thumbnailUrl}
          active
          mode="live"
          onError={(message) =>
            captureError(message, { user: did, source: "recommendation-embed" })
          }
        />
        {/* Transparent Link overlay so clicks navigate to the
            streamer's page instead of toggling the embedded player's
            play/pause. */}
        <Link
          to="/$user"
          params={{ user: handle ?? did }}
          aria-label={handle ? `Watch ${handle}` : "Watch this streamer"}
          className="absolute inset-0 z-10"
        />
      </div>
      <div className="flex shrink-0 items-center gap-2 px-1 text-left">
        {avatar ? (
          <img
            src={avatar}
            alt=""
            className="h-6 w-6 shrink-0 rounded-full border border-white/20 object-cover"
          />
        ) : (
          <div className="h-6 w-6 shrink-0 rounded-full border border-white/20 bg-white/10" />
        )}
        <p className="min-w-0 truncate text-sm text-white">
          Watch <span className="font-semibold">@{handle ?? did}</span>
        </p>
      </div>
    </div>
  );
}
