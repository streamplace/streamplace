import { useDashboardStore } from "@/components/dashboard/dashboard-store-context";
import { Admonition } from "@/components/ui/admonition";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { CUSTOM_LICENSE, LICENSE_OPTIONS } from "@/lib/content-licenses";
import { CONTENT_WARNINGS } from "@/lib/content-warnings";
import { useSession } from "@/lib/session";
import { useStore } from "@/lib/store";
import { useKeyRecords } from "@/lib/store/hooks";
import { cn } from "@/lib/utils";
import { createFileRoute } from "@tanstack/react-router";
import {
  Clipboard,
  ExternalLink,
  Key,
  Loader2,
  Shield,
  Tags,
  Trash2,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { place } from "streamplace";
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
  { to: "/dashboard/stream#stream-key", icon: Key, labelKey: "stream-key" },
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
      <div className="sticky top-0 z-30 -mx-4 bg-(--color-bg)/80 px-4 py-2 backdrop-blur-md lg:hidden">
        <h2 className="font-display mb-2 text-lg font-semibold tracking-tight">
          {t("stream-settings", { defaultValue: "Stream Settings" })}
        </h2>

        <div className="relative -mx-4 px-4">
          <div className="pointer-events-none absolute top-0 bottom-0 left-0 z-10 w-5 bg-linear-to-r from-(--color-bg) to-transparent" />
          <div className="pointer-events-none absolute top-0 right-0 bottom-0 z-10 w-5 bg-linear-to-l from-(--color-bg) to-transparent" />

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
      <div className="hidden shrink-0 lg:block lg:w-56">
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
        <section id="stream-key" className="scroll-mt-20">
          <StreamKeySection />
        </section>
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
    <div className="mx-auto w-full max-w-2xl space-y-6 py-4">
      <h2 className="font-display text-lg font-semibold">
        {t("metadata", { defaultValue: "Metadata" })}
      </h2>

      {!userDid ? (
        <p className="text-sm text-(--color-fg-muted)">
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
                        ? "border-(--color-accent) bg-(--color-accent)/10"
                        : "border-(--color-border) hover:border-(--color-border-strong)",
                    )}
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => toggleWarning(cw.value)}
                      className="focus:ring-ring mt-0.5 size-4 rounded border-(--color-border) text-(--color-accent)"
                    />
                    <div className="min-w-0">
                      <div className="text-sm font-medium">{cw.label}</div>
                      <div className="text-[11px] text-(--color-fg-muted)">
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
                className="focus:ring-ring h-9 w-full rounded-md border border-(--color-border) bg-(--color-bg) px-3 text-sm text-(--color-fg) focus:ring-1 focus:outline-none"
              />
            </Field>
            <Field label={t("license", { defaultValue: "License" })}>
              <select
                value={licenseSelect}
                onChange={(e) => setLicenseSelect(e.target.value)}
                className="focus:ring-ring h-9 w-full rounded-md border border-(--color-border) bg-(--color-bg) px-3 text-sm text-(--color-fg) focus:ring-1 focus:outline-none"
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
                  className="focus:ring-ring h-9 w-full rounded-md border border-(--color-border) bg-(--color-bg) px-3 text-sm text-(--color-fg) focus:ring-1 focus:outline-none"
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
                className="focus:ring-ring w-full resize-none rounded-md border border-(--color-border) bg-(--color-bg) px-3 py-2 text-sm text-(--color-fg) focus:ring-1 focus:outline-none"
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
                  className="focus:ring-ring w-full resize-none rounded-md border border-(--color-border) bg-(--color-bg) px-3 py-2 font-mono text-sm text-(--color-fg) placeholder:text-(--color-fg-muted) focus:ring-1 focus:outline-none"
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
                  className="focus:ring-ring h-9 w-full rounded-md border border-(--color-border) bg-(--color-bg) px-3 text-sm text-(--color-fg) focus:ring-1 focus:outline-none"
                />
              </Field>
            )}
          </SubSection>

          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            className="flex h-9 items-center gap-2 rounded-md bg-(--color-accent) px-4 font-medium text-(--color-accent-fg) transition-colors hover:bg-(--color-accent-hover) disabled:cursor-not-allowed disabled:opacity-50"
          >
            {saving && <Loader2 className="size-4 animate-spin" />}
            {t("save", { defaultValue: "Save" })}
          </button>

          <p className="flex items-center gap-1 text-xs text-(--color-fg-muted)">
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

function StreamKeySection() {
  const { t } = useTranslation("common");
  const createStreamKeyRecord = useStore((s) => s.createStreamKeyRecord);
  const deleteStreamKeyRecord = useStore((s) => s.deleteStreamKeyRecord);
  const getStreamKeyRecords = useStore((s) => s.getStreamKeyRecords);
  const pdsAgent = useStore((s) => s.pdsAgent);
  const keyObj = useKeyRecords();
  const keyRecords = keyObj?.records ?? null;
  const newKey = useStore((s) => s.newKey);
  const [creating, setCreating] = useState(false);
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [showDeleteAllDialog, setShowDeleteAllDialog] = useState(false);
  const [deletingKeys, setDeletingKeys] = useState<Set<string>>(new Set());
  const [visibleCount, setVisibleCount] = useState(10);
  const [ingestUrls, setIngestUrls] = useState<{ type: string; url: string }[]>(
    [],
  );

  useEffect(() => {
    const timeout = setTimeout(() => {
      getStreamKeyRecords();
    }, 500);
    return () => clearTimeout(timeout);
  }, [getStreamKeyRecords]);

  useEffect(() => {
    if (!pdsAgent) return;
    pdsAgent.client
      .call(place.stream.ingest.getIngestUrls, {})
      .then((res) => {
        setIngestUrls(
          res.ingests
            .map((i: any) => ({ type: i.type || "unknown", url: i.url }))
            .filter((i) => i.url),
        );
      })
      .catch(() => {});
  }, [pdsAgent]);

  const handleCreate = useCallback(async () => {
    setShowCreateDialog(false);
    setCreating(true);
    try {
      await createStreamKeyRecord(true);
    } catch (err: any) {
      toast.error(err.message || t("failed-to-create-key"));
    } finally {
      setCreating(false);
    }
  }, [createStreamKeyRecord, t]);

  const handleDelete = useCallback(
    async (rkey: string) => {
      if (deletingKeys.has(rkey)) return;
      setDeletingKeys((prev) => new Set(prev).add(rkey));
      try {
        await deleteStreamKeyRecord(rkey);
      } catch (err: any) {
        toast.error(err.message || t("failed-to-delete-key"));
      } finally {
        setDeletingKeys((prev) => {
          const next = new Set(prev);
          next.delete(rkey);
          return next;
        });
      }
    },
    [deletingKeys, deleteStreamKeyRecord, t],
  );

  const handleDeleteAll = useCallback(async () => {
    setShowDeleteAllDialog(false);
    const rkeys =
      keyRecords?.records.map((r) => r.uri.split("/").pop() as string) ?? [];
    if (rkeys.length === 0) return;
    setDeletingKeys(new Set(rkeys));
    try {
      await deleteStreamKeyRecord(undefined, rkeys);
    } catch (err: any) {
      toast.error(err.message || t("failed-delete-keys"));
    }
    setDeletingKeys(new Set());
  }, [keyRecords, deleteStreamKeyRecord, t]);

  const handleCopy = useCallback(
    (text: string) => {
      navigator.clipboard.writeText(text);
      toast.success(t("copied-to-clipboard"));
    },
    [t],
  );

  const visibleKeys = keyRecords?.records.slice(0, visibleCount) ?? [];
  const hasMore = keyRecords && keyRecords.records.length > visibleCount;

  return (
    <div className="space-y-6">
      <div>
        <h2 className="font-display text-lg font-semibold">
          {t("stream-key", { defaultValue: "Stream Key" })}
        </h2>
        <p className="text-sm text-(--color-fg-muted)">
          {t("stream-key-help", {
            defaultValue:
              "Your stream key identifies you to Streamplace. You can have more than one stream key at a time, and it's a good idea to prune old keys that you aren't using.",
          })}
        </p>
      </div>

      {/* Newly created key */}
      {newKey && (
        <div className="space-y-3">
          <Admonition type="warning" size="sm">
            <Admonition.Title>
              {t("stream-key-save-now", {
                defaultValue: "Please save this stream key somewhere safe.",
              })}
            </Admonition.Title>
            <Admonition.Description>
              {t("stream-key-info-description", {
                defaultValue:
                  "For security reasons, you won't be able to view it again through your account. If you lose this stream key, you'll need to generate a new one.",
              })}
            </Admonition.Description>
          </Admonition>

          <div className="rounded-lg border border-(--color-border) bg-(--color-bg-elevated) p-4">
            <p className="mb-1 text-[11px] font-medium text-(--color-fg-muted)">
              {t("stream-key-label", { defaultValue: "Stream Key" })}
            </p>
            <div className="flex items-center gap-2">
              <code className="min-w-0 flex-1 font-mono text-xs leading-relaxed break-all">
                {newKey.privateKey}
              </code>
              <Button
                size="icon"
                variant="ghost"
                onClick={() => handleCopy(newKey.privateKey)}
                className="shrink-0"
              >
                <Clipboard className="size-3.5" />
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Existing keys */}
      {
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium">
              {t("your-stream-pubkeys", { defaultValue: "Your Keys" })}
            </p>
            <div className="flex items-center gap-2">
              <Button
                onClick={() => setShowCreateDialog(true)}
                disabled={creating}
                variant="secondary"
              >
                {creating && <Loader2 className="size-4 animate-spin" />}
                {t("create-stream-key", { defaultValue: "Create Stream Key" })}
              </Button>
              <Button
                variant="destructive"
                onClick={() => setShowDeleteAllDialog(true)}
                disabled={deletingKeys.size > 0}
              >
                <Trash2 size={14} />
                {t("delete-all-keys", { defaultValue: "Delete All" })}
              </Button>
            </div>
          </div>

          <div className="divide-y divide-(--color-border) rounded-lg border border-(--color-border) bg-(--color-bg-elevated)">
            {visibleKeys.map((keyRecord) => {
              const rkey = keyRecord.uri.split("/").pop() as string;
              const value = keyRecord.value as {
                signingKey?: string;
                createdAt?: string;
                createdBy?: string;
              };
              const isDeleting = deletingKeys.has(rkey);

              return (
                <div
                  key={rkey}
                  className="flex items-center justify-between gap-3 px-3 py-2.5"
                  style={{ opacity: isDeleting ? 0.5 : 1 }}
                >
                  <div className="min-w-0 flex-1 space-y-0.5">
                    {value.signingKey && (
                      <div className="truncate font-mono text-xs">
                        {value.signingKey}
                      </div>
                    )}
                    {value.createdAt && (
                      <div className="text-xs text-(--color-fg-muted)">
                        {value.createdBy || "Created"} &middot;{" "}
                        {new Date(value.createdAt).toLocaleDateString()}
                      </div>
                    )}
                  </div>
                  <Button
                    size="icon"
                    variant="ghost"
                    onClick={() => handleDelete(rkey)}
                    disabled={isDeleting}
                  >
                    <X size={14} />
                  </Button>
                </div>
              );
            })}
          </div>

          {hasMore && (
            <button
              type="button"
              onClick={() => setVisibleCount((c) => c + 10)}
              className="w-full rounded-md border border-(--color-border) py-2 text-sm text-(--color-fg-muted) transition-colors hover:bg-(--color-bg-elevated)"
            >
              {t("load-more", { defaultValue: "Load more" })} (
              {keyRecords.records.length - visibleCount}{" "}
              {t("remaining", { defaultValue: "remaining" })})
            </button>
          )}

          <p className="text-xs text-(--color-fg-muted)">
            {t("keys-count", {
              count: keyRecords?.records.length || 0,
              defaultValue: "{$count} keys",
            })}
          </p>
        </div>
      }

      {/* Go Live instructions */}
      {keyRecords && keyRecords.records.length > 0 && (
        <GoLiveSection ingestUrls={ingestUrls} />
      )}

      {/* Create dialog */}
      <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("create-key-title", {
                defaultValue: "Are you sure about creating a new stream key?",
              })}
            </DialogTitle>
            <DialogDescription className="font-semibold">
              <span className="text-red-400">
                {t("create-key-do-not-share", {
                  defaultValue:
                    "DO NOT SHARE THIS KEY OR SHOW IT ON STREAM, as ANYONE with this key may be able to stream from your account!",
                })}
              </span>{" "}
              <span className="text-red-400">
                {t("create-key-description-staff", {
                  defaultValue:
                    "Staff will never, ever ask for your stream key!",
                })}
              </span>
              <br />
              <br />
              <span>
                {t("create-key-description-delete-if-exposed", {
                  defaultValue:
                    "If you think your stream key has been exposed, delete it immediately and create a new one!",
                })}
              </span>
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <DialogClose>
              <Button variant="outline">
                {t("cancel", { defaultValue: "Cancel" })}
              </Button>
            </DialogClose>
            <Button onClick={handleCreate} variant="destructive">
              {creating && <Loader2 className="size-4 animate-spin" />}
              {t("create-stream-key", { defaultValue: "Create Stream Key" })}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete all dialog */}
      <Dialog open={showDeleteAllDialog} onOpenChange={setShowDeleteAllDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("delete-all-keys-title", {
                defaultValue: "Delete all stream keys?",
              })}
            </DialogTitle>
            <DialogDescription>
              {t("delete-all-keys-description", {
                defaultValue:
                  "This will permanently remove all stream keys. Any streaming software configured with these keys will stop working.",
                count: keyRecords?.records.length ?? 0,
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <DialogClose>
              <Button variant="outline">
                {t("cancel", { defaultValue: "Cancel" })}
              </Button>
            </DialogClose>
            <button
              type="button"
              onClick={handleDeleteAll}
              className="inline-flex h-9 items-center justify-center rounded-md bg-red-500 px-4 text-sm font-medium text-white transition-colors hover:bg-red-600"
            >
              {t("delete-all-keys-btn", { defaultValue: "Delete All" })}
            </button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function GoLiveSection({
  ingestUrls,
}: {
  ingestUrls: { type: string; url: string }[];
}) {
  const { t } = useTranslation("common");
  const [mode, setMode] = useState<"rtmp" | "whip">("rtmp");

  const handleCopy = useCallback((text: string) => {
    navigator.clipboard.writeText(text);
  }, []);

  const rtmpIngest = ingestUrls.find((i) => i.type === "rtmp");
  const whipIngest = ingestUrls.find((i) => i.type === "whip");

  const activeIngest = mode === "rtmp" ? rtmpIngest : whipIngest;

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold">
        {t("go-live", { defaultValue: "Go Live" })}
      </h3>
      <div className="space-y-4 rounded-lg border border-(--color-border) bg-(--color-bg-elevated) p-4">
        <Tabs value={mode} onValueChange={(v) => setMode(v as "rtmp" | "whip")}>
          <TabsList variant="default">
            <TabsTrigger value="rtmp">RTMP</TabsTrigger>
            <TabsTrigger value="whip">WHIP</TabsTrigger>
          </TabsList>
        </Tabs>

        {activeIngest ? (
          <div className="space-y-2">
            <p className="text-[11px] font-medium text-(--color-fg-muted)">
              {t("server-url", { defaultValue: "Server URL" })}
            </p>
            <div className="flex items-center gap-2">
              <code className="min-w-0 flex-1 font-mono text-xs leading-relaxed break-all">
                {activeIngest.url}
              </code>
              <Button
                size="icon"
                variant="ghost"
                onClick={() => handleCopy(activeIngest.url)}
                className="shrink-0"
              >
                <Clipboard className="size-3.5" />
              </Button>
            </div>
          </div>
        ) : (
          <p className="text-xs text-(--color-fg-muted)">
            {t("no-ingest", {
              defaultValue: `${mode.toUpperCase()} ingest not available.`,
            })}
          </p>
        )}

        <div className="space-y-2">
          <p className="text-[11px] font-medium text-(--color-fg-muted)">
            {t("obs-instructions", { defaultValue: "OBS Settings" })}
          </p>
          <div className="space-y-2 rounded-md bg-(--color-bg) p-3 font-mono text-xs">
            <div>
              <span className="text-(--color-fg-muted)">
                Settings &gt; Stream
              </span>
              <ul className="mt-1 space-y-0.5 pl-3">
                <li>
                  Service = <span className="text-(--color-fg)">Custom...</span>
                </li>
                <li>
                  Server ={" "}
                  <span className="text-(--color-fg)">
                    {activeIngest?.url || "N/A"}
                  </span>
                </li>
                {mode === "rtmp" ? (
                  <li>
                    Stream Key ={" "}
                    <span className="text-(--color-fg)">(your stream key)</span>
                  </li>
                ) : (
                  <li>
                    Bearer Token ={" "}
                    <span className="text-(--color-fg)">(your stream key)</span>
                  </li>
                )}
              </ul>
            </div>
            <div>
              <span className="text-(--color-fg-muted)">
                Settings &gt; Output (Advanced)
              </span>
              <ul className="mt-1 space-y-0.5 pl-3">
                <li>
                  B-frames = <span className="text-(--color-fg)">Off</span>
                </li>
                <li>
                  Keyframe Interval ={" "}
                  <span className="text-(--color-fg)">1 second</span>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </div>
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
        <p className="mt-1 text-sm text-(--color-fg-muted)">
          {t("moderation-help", {
            defaultValue:
              "Add/remove stream moderators. Moderators can hide chat messages and time out users in your chat.",
          })}
        </p>
      </div>
      <div className="rounded-lg border border-dashed border-(--color-border) p-6 text-sm text-(--color-fg-muted)">
        {t("moderation-coming-soon", {
          defaultValue:
            "Moderator management hooks aren't on the web yet. This section is a placeholder.",
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
        {help && <p className="mt-1 text-xs text-(--color-fg-muted)">{help}</p>}
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
      <label className="text-xs font-medium text-(--color-fg-muted)">
        {label}
      </label>
      {children}
      {help && <p className="text-[11px] text-(--color-fg-muted)">{help}</p>}
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
        className="focus:ring-ring size-4 rounded border-(--color-border) text-(--color-accent)"
      />
      <span>{label}</span>
    </label>
  );
}
