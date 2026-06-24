// PDS host selector shown during signup. Mirrors
// js/app/components/login/pds-host-selector-modal.tsx but uses the
// web's Dialog primitive and Tailwind classes instead of React Native
// styles. The host list itself lives in @streamplace/core so the app
// and web share it.
//
// The flow: user clicks "Sign Up" in the login modal → login modal
// closes and this one opens → user picks a PDS (or enters a custom
// URL) and confirms → onSubmit(pdsHost) is called and the modal
// closes. The caller is responsible for setting the PDS and starting
// the login round-trip.
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useStore } from "@/lib/store";
import { SHUFFLED_PDS_HOSTS, type PdsHost } from "@streamplace/core";
import { Check, ExternalLink } from "lucide-react";
import { useState } from "react";
import { Trans, useTranslation } from "react-i18next";

interface PdsHostSelectorModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (pdsHost: string) => void;
}

export function PdsHostSelectorModal({
  open,
  onOpenChange,
  onSubmit,
}: PdsHostSelectorModalProps) {
  const { t } = useTranslation("common");
  const [selectedHost, setSelectedHost] = useState<string | null>(
    SHUFFLED_PDS_HOSTS[0].value,
  );
  const [customHost, setCustomHost] = useState("");
  const [useCustom, setUseCustom] = useState(false);
  const [handlePolicyChecked, setHandlePolicyChecked] = useState(false);

  const loginError = useStore((s) => s.loginState.error);
  const setLoginError = useStore((s) => s.setLoginError);

  const selectedHostObj: PdsHost =
    SHUFFLED_PDS_HOSTS.find((host) => host.value === selectedHost) ??
    SHUFFLED_PDS_HOSTS[0];

  const handleCancel = () => {
    setSelectedHost(SHUFFLED_PDS_HOSTS[0].value);
    setCustomHost("");
    setUseCustom(false);
    setHandlePolicyChecked(false);
    setLoginError(null);
    onOpenChange(false);
  };

  const handleSubmit = () => {
    const hostToUse = useCustom ? customHost : selectedHost;
    if (!hostToUse) return;
    onSubmit(hostToUse);
    handleCancel();
  };

  const handleLearnMore = () => {
    window.open(
      "https://atproto.com/guides/self-hosting",
      "_blank",
      "noopener",
    );
  };
  const handleTOS = () => {
    window.open(selectedHostObj.terms, "_blank", "noopener");
  };
  const handlePrivacy = () => {
    window.open(selectedHostObj.privacy, "_blank", "noopener");
  };
  const handleHandlePolicy = () => {
    if (selectedHostObj.handlePolicyDocs) {
      window.open(selectedHostObj.handlePolicyDocs, "_blank", "noopener");
    }
  };

  const canSubmit =
    (useCustom ? customHost.trim().length > 0 : true) &&
    (handlePolicyChecked || !selectedHostObj.handlePolicyDocs);

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (next) return;
        handleCancel();
      }}
    >
      <DialogContent showCloseButton={false} className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("pds-selector-title")}</DialogTitle>
          <DialogDescription>{t("pds-selector-description")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-2">
          {SHUFFLED_PDS_HOSTS.map((host) => {
            const isSelected = !useCustom && selectedHost === host.value;
            return (
              <button
                key={host.value}
                type="button"
                onClick={() => {
                  setSelectedHost(host.value);
                  setUseCustom(false);
                }}
                className={
                  "w-full rounded-lg border px-3 py-2 text-left transition-colors " +
                  (isSelected
                    ? "border-(--color-accent) bg-(--color-accent)/5"
                    : "border-(--color-border) hover:border-(--color-border-strong)")
                }
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="font-medium">{host.label}</div>
                    <div className="mt-0.5 text-sm text-(--color-fg-muted)">
                      {host.description}
                    </div>
                  </div>
                  {isSelected && (
                    <Check className="size-5 shrink-0 text-(--color-accent)" />
                  )}
                </div>
              </button>
            );
          })}

          <button
            type="button"
            onClick={() => setUseCustom(true)}
            className={
              "w-full rounded-lg border px-3 py-2 text-left transition-colors " +
              (useCustom
                ? "border-(--color-accent) bg-(--color-accent)/5"
                : "border-(--color-border) hover:border-(--color-border-strong)")
            }
          >
            <div className="flex items-center justify-between gap-2">
              <div className="min-w-0 flex-1">
                <div className="font-medium">
                  {t("pds-selector-custom-label")}
                </div>
                <div className="mt-0.5 text-sm text-(--color-fg-muted)">
                  {t("pds-selector-custom-description")}
                </div>
              </div>
              {useCustom && (
                <Check className="size-5 shrink-0 text-(--color-accent)" />
              )}
            </div>
          </button>

          {useCustom && (
            <div className="pt-2">
              <label className="block">
                <span className="text-sm text-(--color-fg-muted)">
                  {t("pds-selector-custom-url-label")}
                </span>
                <input
                  type="url"
                  value={customHost}
                  onChange={(e) => setCustomHost(e.target.value)}
                  placeholder={t("pds-selector-custom-url-placeholder")}
                  autoCapitalize="none"
                  autoCorrect="false"
                  className="mt-1 h-10 w-full rounded-md border border-(--color-border) bg-(--color-bg-elevated) px-3 transition-colors focus:border-(--color-accent) focus:outline-none"
                />
              </label>
            </div>
          )}

          <button
            type="button"
            onClick={handleLearnMore}
            className="inline-flex items-center gap-1 text-sm text-(--color-accent) hover:underline"
          >
            {t("pds-selector-learn-more")}
            <ExternalLink className="size-3.5" />
          </button>

          <div className="space-y-2 rounded-md border border-(--color-border) bg-(--color-bg-overlay) p-3 text-sm text-(--color-fg-muted)">
            <p>{t("pds-selector-info")}</p>
            {!useCustom && (
              <p>
                <Trans
                  i18nKey="pds-selector-read-policies"
                  values={{ label: selectedHostObj.label }}
                  components={{
                    tosLink: (
                      <button
                        type="button"
                        onClick={handleTOS}
                        className="text-secondary hover:underline"
                      />
                    ),
                    privacyLink: (
                      <button
                        type="button"
                        onClick={handlePrivacy}
                        className="text-secondary hover:underline"
                      />
                    ),
                  }}
                />
              </p>
            )}
            <p>{t("pds-selector-different-policies")}</p>
          </div>

          {!useCustom && selectedHostObj.handlePolicyDocs && (
            <label className="flex cursor-pointer items-start gap-2 rounded-md border border-(--color-border) p-3">
              <input
                type="checkbox"
                checked={handlePolicyChecked}
                onChange={(e) => setHandlePolicyChecked(e.target.checked)}
                className="mt-0.5"
              />
              <span className="text-sm">
                <Trans
                  i18nKey="pds-selector-handle-policy-checkbox"
                  components={{
                    policyLink: (
                      <button
                        type="button"
                        onClick={handleHandlePolicy}
                        className="text-secondary hover:underline"
                      />
                    ),
                  }}
                />
              </span>
            </label>
          )}

          {loginError && (
            <p className="text-sm text-(--color-danger)">{loginError}</p>
          )}
        </div>

        <DialogFooter>
          <Button variant="secondary" onClick={handleCancel}>
            {t("cancel")}
          </Button>
          <Button onClick={handleSubmit} disabled={!canSubmit}>
            {t("continue")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
