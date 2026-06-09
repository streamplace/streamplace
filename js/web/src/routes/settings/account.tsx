import { CardMenuSection } from "@/components/ui/card";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { LogOut } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useSession } from "../../lib/session";
import { useUserProfile } from "../../lib/store/hooks";

export const Route = createFileRoute("/settings/account")({
  component: AccountSettings,
});

function AccountSettings() {
  const { t } = useTranslation("settings");
  const { state, signOut } = useSession();
  const navigate = useNavigate();
  const userProfile = useUserProfile();

  if (state.status !== "authenticated" || !userProfile) {
    return (
      <div className="space-y-6">
        <div className="text-sm text-[var(--color-fg-muted)]">
          Please log in to access this page.
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Profile header */}
      <div className="flex flex-col items-center gap-3 py-4">
        {userProfile.avatar && (
          <img
            src={userProfile.avatar}
            alt=""
            className="w-20 h-20 rounded-full"
          />
        )}
        <h1 className="text-xl font-semibold font-display">@{userProfile.handle}</h1>
      </div>

      <CardMenuSection>
        <a
          href={`https://bsky.app/profile/${userProfile.handle}`}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center justify-between px-3 py-2.5 hover:bg-[var(--color-bg)] transition-colors"
        >
          <span className="text-sm">{t("edit-profile-bluesky")}</span>
          <svg
            width="16"
            height="16"
            viewBox="0 0 16 16"
            fill="none"
            className="text-[var(--color-fg-muted)]"
          >
            <path
              d="M12 9V13a1 1 0 01-1 1H3a1 1 0 01-1-1V5a1 1 0 011-1h4M10 2h4v4M14 2L7 9"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </a>
      </CardMenuSection>

      <CardMenuSection>
        <Link
          to="/settings/badges"
          className="flex items-center justify-between px-3 py-2.5 hover:bg-[var(--color-bg)] transition-colors"
        >
          <span className="text-sm">{t("badges")}</span>
          <svg
            width="16"
            height="16"
            viewBox="0 0 16 16"
            fill="none"
            className="text-[var(--color-fg-muted)]"
          >
            <path
              d="M6 3l5 5-5 5"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </Link>
      </CardMenuSection>

      <CardMenuSection>
        <button
          type="button"
          onClick={() => {
            void signOut();
            navigate({ to: "/settings" });
          }}
          className="flex items-center gap-3 px-3 py-2.5 w-full hover:bg-[var(--color-bg)] transition-colors text-left"
        >
          <LogOut size={20} className="text-[var(--color-fg-muted)]" />
          <span className="text-sm">{t("log-out")}</span>
        </button>
      </CardMenuSection>
    </div>
  );
}
