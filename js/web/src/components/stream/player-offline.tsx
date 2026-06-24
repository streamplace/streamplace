import type { LivestreamStore } from "@streamplace/core";
import { useTranslation } from "react-i18next";
import { useStore } from "zustand";
import useAvatars from "../../hooks/use-avatars";

// Replaces the player when the stream is offline. Renders the
// streamer's banner as a blurred background with an offline badge,
// avatar, and handle on top. Sits in the same DOM position as the
// Player (absolute inset-0 inside the VideoSection container) so the
// transition between live and offline is just an opacity swap.
export function PlayerOffline({
  store,
  user,
}: {
  store: LivestreamStore;
  user: string;
}) {
  const { t } = useTranslation("common");
  const profile = useStore(store, (s) => s.profile);
  const avatars = useAvatars(profile?.did ? [profile.did] : []);
  const detailed = profile?.did ? avatars[profile.did] : null;
  const banner = detailed?.banner;
  const avatar = detailed?.avatar || profile?.avatar;
  const handle = profile?.handle || user;

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
        <p className="text-sm text-white/60">
          {t("user-not-streaming-check-back", { user: handle })}
        </p>
      </div>
    </div>
  );
}
