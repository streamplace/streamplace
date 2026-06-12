import { useDashboardStore } from "@/components/dashboard/dashboard-store-context";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { CUSTOM_LICENSE, LICENSE_OPTIONS } from "@/lib/content-licenses";
import { CONTENT_WARNINGS } from "@/lib/content-warnings";
import { useSession } from "@/lib/session";
import { useStore } from "@/lib/store";
import { cn } from "@/lib/utils";
import { createFileRoute } from "@tanstack/react-router";
import { ExternalLink, Loader2, Shield, Tags } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { useStore as useZustandStore } from "zustand";

export const Route = createFileRoute("/dashboard/stream/")({
  component: StreamSettingsPage,
});

interface NavItem {
  to: string;
  icon: React.ComponentType<{ className?: string }>;
  labelKey: string;
}

const NAV_ITEMS: NavItem[] = [
  { to: "/dashboard/stream#metadata", icon: Tags, labelKey: "metadata" },
  { to: "/dashboard/stream#moderation", icon: Shield, labelKey: "moderation" },
];

const linkClass = cn(
  "flex items-center gap-2",
  "px-3 py-1.5 lg:rounded-md transition-colors -mr-0.5 lg:mr-0",
  "whitespace-nowrap",
  "text-(--color-fg-muted) hover:text-(--color-fg) hover:bg-(--color-bg-elevated)",
  "[&.active]:text-(--color-fg) [&.active]:font-medium",
  "lg:[&.active]:bg-(--color-bg-elevated)",
  "border-b [&.active]:border-b-(--color-fg)",
  "lg:border-b-0 lg:[&.active]:border-0",
);

export function StreamSettingsPage() {
  const { t } = useTranslation("common");

  return (
    <div className="mx-auto flex max-w-screen flex-col gap-6 px-4 py-6 lg:flex-row lg:gap-8 lg:px-6 lg:py-10">
      {/* Mobile: floating sticky nav */}
      <div className="sticky top-0 z-30 -mx-4 bg-[var(--color-bg)]/80 px-4 py-2 backdrop-blur-md lg:hidden">
        <h2 className="font-display mb-2 text-lg font-semibold tracking-tight">
          {t("stream-settings", { defaultValue: "Stream Settings" })}
        </h2>

        <div className="relative -mx-4 px-4">
          <div className="pointer-events-none absolute top-0 bottom-0 left-0 z-10 w-5 bg-gradient-to-r from-[var(--color-bg)] to-transparent" />
          <div className="pointer-events-none absolute top-0 right-0 bottom-0 z-10 w-5 bg-gradient-to-l from-[var(--color-bg)] to-transparent" />

          <nav
            className="-mx-5 flex scrollbar-none gap-0.5 overflow-x-auto px-5"
            style={{ scrollbarWidth: "none", msOverflowStyle: "none" }}
          >
            {NAV_ITEMS.map((item) => {
              const Icon = item.icon;
              return (
                <a key={item.to} href={item.to} className={linkClass}>
                  <Icon className="size-4" />
                  {t(item.labelKey, { defaultValue: item.labelKey })}
                </a>
              );
            })}
          </nav>
        </div>
      </div>

      {/* Desktop: sidebar */}
      <div className="hidden shrink-0 lg:block lg:w-52">
        <nav className="sticky top-6 flex flex-col gap-0.5 self-start">
          <p className="font-display px-2 pb-3 text-2xl font-semibold">
            {t("stream-settings", { defaultValue: "Stream Settings" })}
          </p>
          {NAV_ITEMS.map((item) => {
            const Icon = item.icon;
            return (
              <a key={item.to} href={item.to} className={linkClass}>
                <Icon className="size-4" />
                {t(item.labelKey, { defaultValue: item.labelKey })}
              </a>
            );
          })}
        </nav>
      </div>

      {/* Content: both sections stacked */}
      <div className="mt-0.75 w-full min-w-0 flex-1 space-y-10 md:w-screen md:max-w-md xl:max-w-xl">
        <section id="metadata" className="scroll-mt-20">
          <MetadataSection />
        </section>
        <section id="moderation" className="scroll-mt-20">
          <ModerationSection />
        </section>
      </div>
    </div>
  );
}

function MetadataSection() {
  const { t } = useTranslation("common");
  const createContentMetadata = useStore((s) => s.createContentMetadata);
  const updateContentMetadata = useStore((s) => s.updateContentMetadata);
  const getContentMetadata = useStore((s) => s.getContentMetadata);
  const { did: userDid } = useSession();
  const liveStore = useDashboardStore();
  const livestream = useZustandStore(liveStore, (s) => s.livestream);

  const [selectedWarnings, setSelectedWarnings] = useState<Set<string>>(
    new Set(),
  );
  const [contentRights, setContentRights] = useState<{
    copyrightYear?: number;
    copyrightNotice?: string;
    license?: string;
    creditLine?: string;
  }>({});
  const [customLicense, setCustomLicense] = useState<string>("");
  const [licenseSelect, setLicenseSelect] = useState<string>("");
  const [allowAllDistribute, setAllowAllDistribute] = useState(false);
  const [allowedBroadcasters, setAllowedBroadcasters] = useState("");
  const [archiveIndefinite, setArchiveIndefinite] = useState(false);
  const [deleteAfter, setDeleteAfter] = useState<string>("300");
  const [initialized, setInitialized] = useState(false);
  const [saving, setSaving] = useState(false);

  // Load existing metadata
  useEffect(() => {
    if (initialized) return;
    setInitialized(true);
    if (userDid) {
      void getContentMetadata({ userDid });
    }
  }, [initialized, getContentMetadata, userDid]);

  // Hydrate from existing metadata once we have data
  const lastRecord = useStore((s) => s.lastCreatedRecord) as any;
  useEffect(() => {
    if (!lastRecord) return;
    const record = lastRecord.record;
    if (!record) return;
    if (record.contentWarnings?.warnings) {
      setSelectedWarnings(new Set(record.contentWarnings.warnings));
    }
    if (record.contentRights) {
      setContentRights({
        copyrightYear: record.contentRights.copyrightYear,
        copyrightNotice: record.contentRights.copyrightNotice,
        license: record.contentRights.license,
        creditLine: record.contentRights.creditLine,
      });
      // Determine if the license is a custom one
      const knownValues = new Set(LICENSE_OPTIONS.map((l) => l.value));
      if (record.contentRights.license) {
        if (knownValues.has(record.contentRights.license)) {
          setLicenseSelect(record.contentRights.license);
        } else {
          setLicenseSelect(CUSTOM_LICENSE);
          setCustomLicense(record.contentRights.license);
        }
      }
    }
    if (record.distributionPolicy) {
      const ap = record.distributionPolicy.allowedBroadcasters;
      if (ap && ap.length === 1 && ap[0] === "*") {
        setAllowAllDistribute(true);
        setAllowedBroadcasters("");
      } else {
        setAllowAllDistribute(false);
        setAllowedBroadcasters((ap ?? []).join("\n"));
      }
      if (record.distributionPolicy.deleteAfter === -1) {
        setArchiveIndefinite(true);
      } else {
        setArchiveIndefinite(false);
        setDeleteAfter(
          record.distributionPolicy.deleteAfter?.toString() ?? "300",
        );
      }
    }
  }, [lastRecord]);

  // Also hydrate from the per-user livestream's content warnings
  useEffect(() => {
    if (!livestream) return;
    const cw = (livestream.record as any)?.contentWarnings?.warnings as
      | string[]
      | undefined;
    if (cw && selectedWarnings.size === 0) {
      setSelectedWarnings(new Set(cw));
    }
  }, [livestream, selectedWarnings.size]);

  const toggleWarning = useCallback((value: string) => {
    setSelectedWarnings((prev) => {
      const next = new Set(prev);
      if (next.has(value)) {
        next.delete(value);
      } else {
        next.add(value);
      }
      return next;
    });
  }, []);

  const handleSave = useCallback(async () => {
    if (!userDid) return;
    setSaving(true);
    try {
      // Strip empty content rights fields
      const filteredRights = Object.fromEntries(
        Object.entries(contentRights).filter(
          ([, v]) => v !== undefined && v !== null && v !== "",
        ),
      ) as typeof contentRights;
      // Handle custom license
      if (licenseSelect === CUSTOM_LICENSE) {
        if (customLicense.trim()) {
          filteredRights.license = customLicense.trim();
        } else {
          delete filteredRights.license;
        }
      } else if (licenseSelect) {
        filteredRights.license = licenseSelect;
      } else {
        delete filteredRights.license;
      }
      // Coerce copyrightYear to int
      if (filteredRights.copyrightYear) {
        const n = parseInt(String(filteredRights.copyrightYear), 10);
        if (isNaN(n)) delete filteredRights.copyrightYear;
        else filteredRights.copyrightYear = n;
      }

      // Build distribution policy
      const broadcasters = allowAllDistribute
        ? ["*"]
        : allowedBroadcasters
            .split("\n")
            .map((s) => s.trim())
            .filter(Boolean);
      const distPolicy: {
        deleteAfter?: number;
        allowedBroadcasters?: string[];
      } = {};
      if (archiveIndefinite) {
        distPolicy.deleteAfter = -1;
      } else if (deleteAfter) {
        const n = parseInt(deleteAfter, 10);
        if (!isNaN(n) && n > 0) distPolicy.deleteAfter = n;
      }
      if (broadcasters.length > 0) {
        distPolicy.allowedBroadcasters = broadcasters;
      }

      const rkey = livestream?.uri.split("/").pop();
      const livestreamRef =
        rkey && livestream
          ? { uri: livestream.uri, cid: (livestream.cid as string) ?? "" }
          : undefined;

      const params = {
        contentWarnings: Array.from(selectedWarnings),
        contentRights: filteredRights,
        distributionPolicy: distPolicy,
      };

      if (livestreamRef) {
        await updateContentMetadata({ rkey, livestreamRef, ...params });
      } else {
        await createContentMetadata(params);
      }
      toast.success(t("metadata-saved", { defaultValue: "Metadata saved" }));
    } catch (error) {
      console.error("Error saving metadata:", error);
      toast.error(
        t("metadata-save-failed", {
          defaultValue: "Failed to save metadata",
        }),
      );
    } finally {
      setSaving(false);
    }
  }, [
    userDid,
    livestream,
    selectedWarnings,
    contentRights,
    licenseSelect,
    customLicense,
    allowAllDistribute,
    allowedBroadcasters,
    archiveIndefinite,
    deleteAfter,
    createContentMetadata,
    updateContentMetadata,
    t,
  ]);

  const currentYear = useMemo(() => new Date().getFullYear(), []);

  return (
    <div className="space-y-6">
      <h2 className="font-display text-lg font-semibold">
        {t("metadata", { defaultValue: "Metadata" })}
      </h2>

      {!userDid ? (
        <p className="text-sm text-[var(--color-fg-muted)]">
          {t("login-required", {
            defaultValue: "Please log in to manage streams.",
          })}
        </p>
      ) : (
        <>
          <SubSection
            title={t("content-warnings", {
              defaultValue: "Content Warnings",
            })}
            required
            help={t("content-warnings-help", {
              defaultValue:
                "You're required to flag your stream with themes that viewers may want a heads-up about.",
            })}
          >
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              {CONTENT_WARNINGS.map((cw) => {
                const checked = selectedWarnings.has(cw.value);
                return (
                  <label
                    key={cw.value}
                    className={cn(
                      "flex cursor-pointer items-start gap-2 rounded-md border p-3 transition-colors",
                      checked
                        ? "border-[var(--color-accent)] bg-[var(--color-accent)]/10"
                        : "border-[var(--color-border)] hover:border-[var(--color-border-strong)]",
                    )}
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => toggleWarning(cw.value)}
                      className="mt-0.5 size-4 rounded border-[var(--color-border)] text-[var(--color-accent)] focus:ring-[var(--color-ring)]"
                    />
                    <div className="min-w-0">
                      <div className="text-sm font-medium">{cw.label}</div>
                      <div className="text-[11px] text-[var(--color-fg-muted)]">
                        {cw.description}
                      </div>
                    </div>
                  </label>
                );
              })}
            </div>
          </SubSection>

          {/* ── Content Rights ────────────────────────────────────── */}
          <SubSection
            title={t("content-rights", {
              defaultValue: "Content Rights",
            })}
            optional
            help={t("content-rights-help", {
              defaultValue:
                "Optional copyright and license information for your stream.",
            })}
          >
            <Field
              label={t("copyright-year", {
                defaultValue: "Copyright Year",
              })}
            >
              <input
                type="number"
                value={contentRights.copyrightYear ?? ""}
                onChange={(e) =>
                  setContentRights({
                    ...contentRights,
                    copyrightYear: e.target.value
                      ? parseInt(e.target.value, 10)
                      : undefined,
                  })
                }
                placeholder={currentYear.toString()}
                className="h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 text-sm text-[var(--color-fg)] focus:ring-1 focus:ring-[var(--color-ring)] focus:outline-none"
              />
            </Field>
            <Field label={t("license", { defaultValue: "License" })}>
              <select
                value={licenseSelect}
                onChange={(e) => setLicenseSelect(e.target.value)}
                className="h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 text-sm text-[var(--color-fg)] focus:ring-1 focus:ring-[var(--color-ring)] focus:outline-none"
              >
                <option value="">
                  {t("select-license", { defaultValue: "Select a license…" })}
                </option>
                {LICENSE_OPTIONS.map((l) => (
                  <option key={l.value} value={l.value}>
                    {l.label}
                  </option>
                ))}
                <option value={CUSTOM_LICENSE}>
                  {t("custom-license", { defaultValue: "Custom…" })}
                </option>
              </select>
            </Field>
            {licenseSelect === CUSTOM_LICENSE && (
              <Field
                label={t("custom-license-url", {
                  defaultValue: "Custom License URL/Text",
                })}
              >
                <Input
                  type="text"
                  value={customLicense}
                  onChange={(e) => setCustomLicense(e.target.value)}
                  placeholder="https://… or text"
                  className="h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 text-sm text-[var(--color-fg)] focus:ring-1 focus:ring-[var(--color-ring)] focus:outline-none"
                />
              </Field>
            )}
            <Field
              label={t("copyright-notice", {
                defaultValue: "Copyright Notice",
              })}
            >
              <Textarea
                value={contentRights.copyrightNotice ?? ""}
                onChange={(e) =>
                  setContentRights({
                    ...contentRights,
                    copyrightNotice: e.target.value,
                  })
                }
                placeholder="© 2025 Your Name"
                rows={2}
                className="w-full resize-none rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-fg)] focus:ring-1 focus:ring-[var(--color-ring)] focus:outline-none"
              />
            </Field>
            <Field label={t("credit-line", { defaultValue: "Credit Line" })}>
              <Textarea
                value={contentRights.creditLine ?? ""}
                onChange={(e) =>
                  setContentRights({
                    ...contentRights,
                    creditLine: e.target.value,
                  })
                }
                placeholder="Your Name"
                rows={2}
              />
            </Field>
          </SubSection>

          {/* ── Distribution ──────────────────────────────────────── */}
          <SubSection
            title={t("distribution", {
              defaultValue: "Distribution",
            })}
            optional
            help={t("distribution-help", {
              defaultValue:
                "Control who can redistribute your stream and for how long archives are kept.",
            })}
          >
            <Toggle
              checked={allowAllDistribute}
              onChange={setAllowAllDistribute}
              label={t("allow-everyone-distribute", {
                defaultValue: "Allow everyone to distribute your content",
              })}
            />
            {!allowAllDistribute && (
              <Field
                label={t("allowed-broadcasters", {
                  defaultValue: "Allowed Broadcasters",
                })}
                help={t("allowed-broadcasters-help", {
                  defaultValue:
                    "Enter the DIDs of the broadcasters you want to allow, one per line.",
                })}
              >
                <textarea
                  value={allowedBroadcasters}
                  onChange={(e) => setAllowedBroadcasters(e.target.value)}
                  placeholder="did:plc:abc123…&#10;did:plc:def456…"
                  rows={4}
                  className="w-full resize-none rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 font-mono text-sm text-[var(--color-fg)] placeholder:text-[var(--color-fg-muted)] focus:ring-1 focus:ring-[var(--color-ring)] focus:outline-none"
                />
              </Field>
            )}
            <Toggle
              checked={archiveIndefinite}
              onChange={setArchiveIndefinite}
              label={t("allow-everyone-archive", {
                defaultValue:
                  "Allow everyone to archive your content indefinitely",
              })}
            />
            {!archiveIndefinite && (
              <Field
                label={t("delete-after", {
                  defaultValue: "Delete After",
                })}
                help={t("delete-after-help", {
                  defaultValue: "Duration in seconds (e.g. 300 for 5 minutes).",
                })}
              >
                <input
                  type="number"
                  value={deleteAfter}
                  onChange={(e) => setDeleteAfter(e.target.value)}
                  className="h-9 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 text-sm text-[var(--color-fg)] focus:ring-1 focus:ring-[var(--color-ring)] focus:outline-none"
                />
              </Field>
            )}
          </SubSection>

          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            className="flex h-9 items-center gap-2 rounded-md bg-[var(--color-accent)] px-4 font-medium text-[var(--color-accent-fg)] transition-colors hover:bg-[var(--color-accent-hover)] disabled:cursor-not-allowed disabled:opacity-50"
          >
            {saving && <Loader2 className="size-4 animate-spin" />}
            {t("save", { defaultValue: "Save" })}
          </button>

          <p className="flex items-center gap-1 text-xs text-[var(--color-fg-muted)]">
            <ExternalLink className="size-3" />
            {t("metadata-learn-more", {
              defaultValue: "Learn more about content metadata",
            })}
          </p>
        </>
      )}
    </div>
  );
}

function ModerationSection() {
  const { t } = useTranslation("common");
  return (
    <div className="space-y-6">
      <h2 className="font-display text-lg font-semibold">
        {t("moderation", { defaultValue: "Moderation" })}
      </h2>
      <div>
        <p className="mt-1 text-sm text-[var(--color-fg-muted)]">
          {t("moderation-help", {
            defaultValue:
              "Add/remove stream moderators. Moderators can hide chat messages and time out users in your chat.",
          })}
        </p>
      </div>
      <div className="rounded-lg border border-dashed border-[var(--color-border)] p-6 text-sm text-[var(--color-fg-muted)]">
        {t("moderation-coming-soon", {
          defaultValue:
            "Moderator management hooks aren't on the web yet — this section is a placeholder.",
        })}
      </div>
    </div>
  );
}

function SubSection({
  title,
  required,
  optional,
  help,
  children,
}: {
  title: string;
  required?: boolean;
  optional?: boolean;
  help?: string;
  children: React.ReactNode;
}) {
  const { t } = useTranslation("common");
  return (
    <div className="space-y-3">
      <div>
        <h3 className="flex items-center gap-2 text-sm font-semibold">
          {title}
          {required && (
            <span className="bg-muted text-foreground rounded-full px-1.5 text-xs">
              {t("required", { defaultValue: "Required" })}
            </span>
          )}
          {optional && (
            <span className="bg-muted text-muted-foreground rounded-full px-1.5 text-xs">
              {t("optional", { defaultValue: "Optional" })}
            </span>
          )}
        </h3>
        {help && (
          <p className="mt-1 text-xs text-[var(--color-fg-muted)]">{help}</p>
        )}
      </div>
      <div className="space-y-3">{children}</div>
    </div>
  );
}

function Field({
  label,
  help,
  children,
}: {
  label: string;
  help?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      <label className="text-xs font-medium text-[var(--color-fg-muted)]">
        {label}
      </label>
      {children}
      {help && (
        <p className="text-[11px] text-[var(--color-fg-muted)]">{help}</p>
      )}
    </div>
  );
}

function Toggle({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
}) {
  return (
    <label className="flex cursor-pointer items-center gap-2 text-sm">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="size-4 rounded border-[var(--color-border)] text-[var(--color-accent)] focus:ring-[var(--color-ring)]"
      />
      <span>{label}</span>
    </label>
  );
}
