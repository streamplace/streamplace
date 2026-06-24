import type { LivestreamStore } from "@streamplace/core";
import { Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore as useZustandStore } from "zustand";
import useAvatars from "../../hooks/use-avatars";
import { useStore } from "../../lib/store";

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
    if (!profile?.did) {
      console.log("[PlayerOffline] no profile.did yet, skipping fetch");
      return;
    }
    const getRecommendations = useStore.getState().getRecommendations;
    let mounted = true;
    const fetchRec = async () => {
      try {
        console.log(
          "[PlayerOffline] fetching recommendations for",
          profile.did,
        );
        const result = await getRecommendations(profile.did);
        if (!mounted) return;
        console.log(
          "[PlayerOffline] got recommendations:",
          result.recommendations?.length,
        );
        const first = result.recommendations?.find(
          (r) =>
            r.$type ===
              "place.stream.live.getRecommendations#livestreamRecommendation" &&
            (r as { did?: string }).did,
        ) as { did?: string; source?: string } | undefined;
        console.log("[PlayerOffline] first recommendation:", first);
        if (first?.did) {
          setRecommendation({ did: first.did, source: first.source ?? "" });
        }
      } catch (err) {
        console.error("[PlayerOffline] recommendations fetch failed:", err);
      }
    };
    fetchRec();
    return () => {
      mounted = false;
    };
  }, [profile?.did]);

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

      <div className="relative flex h-full flex-col items-center justify-center gap-3 px-6 text-center">
        <span className="rounded-full border border-white/20 bg-black/40 px-2.5 py-0.5 text-[10px] font-semibold tracking-wider text-white/80 uppercase">
          {t("offline")}
        </span>
        {avatar && (
          <img
            src={avatar}
            alt=""
            className="h-16 w-16 rounded-full border-2 border-white/20 object-cover"
          />
        )}
        <p className="text-lg font-medium text-white">@{handle}</p>
        {recommendation ? (
          <Link
            to="/$user"
            params={{ user: recHandle ?? recommendation.did }}
            className="mt-2 flex items-center gap-3 rounded-lg border border-white/10 bg-black/40 px-3 py-2 text-left text-white/90 transition-colors hover:bg-black/60"
          >
            {recAvatar ? (
              <img
                src={recAvatar}
                alt=""
                className="h-8 w-8 rounded-full border border-white/20 object-cover"
              />
            ) : (
              <div className="h-8 w-8 rounded-full border border-white/20 bg-white/10" />
            )}
            <span className="text-sm">
              Watch @{recHandle ?? recommendation.did} instead
            </span>
          </Link>
        ) : (
          <p className="text-sm text-white/60">
            {t("user-not-streaming-check-back", { user: handle })}
          </p>
        )}
      </div>
    </div>
  );
}
