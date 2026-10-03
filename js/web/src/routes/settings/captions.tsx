import { CaptionSettings } from "@/components/captions/caption-settings";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

export const Route = createFileRoute("/settings/captions")({
  component: CaptionsPage,
});
function CaptionsPage() {
  const { t } = useTranslation("settings");
  return (
    <div className="mx-auto w-full max-w-2xl space-y-6 p-4">
      <h1 className="text-xl font-semibold">{t("captions")}</h1>
      <CaptionSettings />
    </div>
  );
}
