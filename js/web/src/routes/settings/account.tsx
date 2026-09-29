import { Button } from "@/components/ui/button";
import { CardMenuSection } from "@/components/ui/card";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { ChevronRight, ExternalLink, LogOut } from "lucide-react";
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
        <div className="text-sm text-(--color-fg-muted)">
          {t("please-log-in-to-access-this-page", {
            defaultValue: "Please log in to access this page.",
          })}
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
            className="h-20 w-20 rounded-full"
          />
        )}
        <h1 className="font-display text-xl font-semibold">
          @{userProfile.handle}
        </h1>
      </div>

      <CardMenuSection>
        <a
          href={`https://bsky.app/profile/${userProfile.handle}`}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center justify-between px-3 py-2.5 transition-colors hover:bg-(--color-bg)"
        >
          <span className="text-sm">{t("edit-profile-bluesky")}</span>
          <ExternalLink className="size-4 text-(--color-fg-muted)" />
        </a>
      </CardMenuSection>

      <CardMenuSection>
        <Link
          to="/settings/badges"
          className="flex items-center justify-between px-3 py-2.5 transition-colors hover:bg-(--color-bg)"
        >
          <span className="text-sm">{t("badges")}</span>
          <ChevronRight className="size-4 text-(--color-fg-muted)" />
        </Link>
      </CardMenuSection>

      <CardMenuSection>
        <Button
          type="button"
          variant="ghost"
          onClick={async () => {
            await signOut();
            navigate({ to: "/settings" });
          }}
          className="h-auto w-full justify-start gap-3 rounded-none px-3 py-2.5 text-left font-normal"
        >
          <LogOut size={20} className="text-(--color-fg-muted)" />
          <span className="text-sm">{t("log-out")}</span>
        </Button>
      </CardMenuSection>
    </div>
  );
}
