import { useTranslation } from "react-i18next";

// Inline "stream offline" overlay rendered on top of the video element
// when the streamer is not currently live. Distinct from the
// page-level offline state in routes/$user.tsx (which replaces the
// whole player) — this is for the in-page "stream is offline, the
// page is still here" case.
export function UserOffline({ user }: { user: string }) {
  const { t } = useTranslation("common");
  return (
    <div className="absolute inset-0 flex items-center justify-center bg-black/60">
      <div className="text-center px-6">
        <div className="text-lg font-medium text-white/90">
          {t("stream-offline")}
        </div>
        <div className="text-sm text-white/60 mt-1">
          {t("user-not-streaming", { user })}
        </div>
      </div>
    </div>
  );
}
