import { useTranslation } from "react-i18next";

// Inline "stream offline" overlay rendered on top of the video element
// when the streamer is not currently live. Distinct from the
// page-level offline state in routes/$user.tsx (which replaces the
// whole player); this is for the in-page "stream is offline, the
// page is still here" case.
export function UserOffline({ user }: { user: string }) {
  const { t } = useTranslation("common");
  return (
    <div className="absolute inset-0 flex items-center justify-center bg-black/55 px-4">
      <div className="flex max-w-sm flex-col items-center rounded-2xl border border-white/10 bg-black/45 px-6 py-5 text-center shadow-2xl backdrop-blur-md">
        <span className="mb-3 flex items-center gap-2 rounded-full border border-white/15 bg-white/8 px-2.5 py-1 text-[11px] font-semibold tracking-[0.16em] text-white/70 uppercase">
          <span className="h-1.5 w-1.5 rounded-full bg-(--color-accent)" />
          {t("offline")}
        </span>
        <div className="font-display text-xl font-semibold text-white/95">
          {t("stream-offline")}
        </div>
        <div className="mt-1 text-sm text-white/65">
          {t("user-not-streaming", { user })}
        </div>
      </div>
    </div>
  );
}
