// PDS host selector shown during signup.
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
  const [showOtherHosts, setShowOtherHosts] = useState(false);
  const [defaultHost, ...otherHosts] = SHUFFLED_PDS_HOSTS;

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
    setShowOtherHosts(false);
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
      <DialogContent
        showCloseButton={false}
        className="flex max-h-[calc(100svh-2rem)] max-w-md flex-col gap-0 overflow-hidden p-0"
      >
        <div className="min-h-0 flex-1 overflow-y-auto px-6 pt-6 pb-4">
          <DialogHeader>
            <DialogTitle>{t("pds-selector-title")}</DialogTitle>
            <DialogDescription>
              {t("pds-selector-description")}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-2">
            <HostOption
              host={defaultHost}
              selected={!useCustom && selectedHost === defaultHost.value}
              onSelect={() => {
                setSelectedHost(defaultHost.value);
                setUseCustom(false);
              }}
            />

            {!showOtherHosts && (
              <button
                type="button"
                onClick={() => setShowOtherHosts(true)}
                className="flex min-h-11 w-full items-center justify-center rounded-lg px-3 text-sm font-medium text-(--color-accent) hover:bg-(--color-bg-overlay)"
              >
                {t("pds-selector-show-other-hosts", {
                  count: otherHosts.length,
                })}
              </button>
            )}

            {showOtherHosts && (
              <>
                {otherHosts.map((host) => (
                  <HostOption
                    key={host.value}
                    host={host}
                    selected={!useCustom && selectedHost === host.value}
                    onSelect={() => {
                      setSelectedHost(host.value);
                      setUseCustom(false);
                    }}
                  />
                ))}

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
                  className="inline-flex min-h-11 items-center gap-1 text-sm text-(--color-accent) hover:underline"
                >
                  {t("pds-selector-learn-more")}
                  <ExternalLink className="size-3.5" />
                </button>

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
              </>
            )}

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

            {loginError && (
              <p className="text-sm text-(--color-danger)">{loginError}</p>
            )}
          </div>
        </div>

        <DialogFooter className="mt-0 shrink-0 border-t border-(--color-border) bg-(--color-bg-elevated) px-6 py-4">
          <Button className="h-11" variant="secondary" onClick={handleCancel}>
            {t("cancel")}
          </Button>
          <Button className="h-11" onClick={handleSubmit} disabled={!canSubmit}>
            {t("continue")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function HostOption({
  host,
  selected,
  onSelect,
}: {
  host: PdsHost;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={
        "min-h-11 w-full rounded-lg border px-3 py-2 text-left transition-colors " +
        (selected
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
        {selected && (
          <Check className="size-5 shrink-0 text-(--color-accent)" />
        )}
      </div>
    </button>
  );
}
