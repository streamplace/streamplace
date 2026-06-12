import { place } from "streamplace";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { createFileRoute } from "@tanstack/react-router";
import { Check, ChevronLeft, ImagePlus, Plus, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { usePDSAgent } from "../../lib/store/hooks";

export const Route = createFileRoute("/settings/badge-issuer")({
  component: BadgeIssuerPanel,
});

type BadgeType =
  | "place.stream.badge.defs#vip"
  | "place.stream.badge.defs#event";

const BADGE_TYPE_OPTIONS: { label: string; value: BadgeType }[] = [
  { label: "VIP", value: "place.stream.badge.defs#vip" },
  { label: "Event", value: "place.stream.badge.defs#event" },
];

type PanelView = "main" | "create" | "issue";

interface BadgeDefItem {
  uri: string;
  cid: string;
  value: {
    name: string;
    description?: string;
    badgeType: string;
    image?: { ref: { toString(): string } };
  };
}

function getDidFromAtUri(uri: string) {
  const parts = uri.split("/");
  return parts.length >= 3 ? parts[2] : null;
}

function BadgeIssuerPanel() {
  const { t } = useTranslation("settings");
  const agent = usePDSAgent();

  const [view, setView] = useState<PanelView>("main");
  const [working, setWorking] = useState(false);
  const [lastResult, setLastResult] = useState<{
    label: string;
    uri: string;
  } | null>(null);

  const [defs, setDefs] = useState<BadgeDefItem[]>([]);
  const [loadingDefs, setLoadingDefs] = useState(false);

  const [selectedDef, setSelectedDef] = useState<BadgeDefItem | null>(null);
  const [recipientDid, setRecipientDid] = useState("");

  const [createName, setCreateName] = useState("");
  const [createDescription, setCreateDescription] = useState("");
  const [createBadgeType, setCreateBadgeType] = useState<BadgeType>(
    "place.stream.badge.defs#vip",
  );
  const [createImageUri, setCreateImageUri] = useState<string | null>(null);
  const [createImageBlob, setCreateImageBlob] = useState<Blob | null>(null);

  const loadDefs = useCallback(async () => {
    if (!agent?.did) return;
    setLoadingDefs(true);
    try {
      const res = await agent.client.list(place.stream.badge.def, {
        repo: agent.did as any,
        limit: 100,
      });
      setDefs(
        (
          res.records as unknown as {
            uri: string;
            cid: string;
            value: BadgeDefItem["value"];
          }[]
        ).map(({ uri, cid, value }) => ({ uri, cid: cid ?? "", value })),
      );
    } catch (error: any) {
      toast.error(error.message || "Failed to load badge definitions");
    } finally {
      setLoadingDefs(false);
    }
  }, [agent]);

  useEffect(() => {
    if (agent?.did) loadDefs();
  }, [agent, loadDefs]);

  const pickImage = useCallback(() => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "image/png,image/jpeg,image/gif,image/webp";
    input.onchange = (e: any) => {
      const file = e.target?.files?.[0];
      if (file) {
        if (file.size > 262144) {
          toast.error("Image must be under 256KB");
          return;
        }
        const blob = new Blob([file], { type: file.type });
        const url = URL.createObjectURL(blob);
        setCreateImageUri(url);
        setCreateImageBlob(blob);
      }
    };
    input.click();
  }, []);

  const handleCreateDef = async () => {
    if (!agent?.did || !createName.trim() || working) return;
    setWorking(true);
    try {
      let imageBlob: any | undefined;
      if (createImageBlob) {
        const uploaded = await agent.uploadBlob(createImageBlob, {
          encoding: createImageBlob.type,
        });
        imageBlob = uploaded.data.blob;
      }

      await agent.client.create(
        place.stream.badge.def,
        {
          name: createName.trim(),
          description: createDescription.trim() || undefined,
          badgeType: createBadgeType,
          image: imageBlob,
          createdAt: new Date().toISOString() as any,
        } as any,
        { repo: agent.did as any },
      );

      setLastResult({
        label: "Badge definition created",
        uri: createName.trim(),
      });
      setCreateName("");
      setCreateDescription("");
      setCreateImageUri(null);
      setCreateImageBlob(null);
      toast.success("Badge definition created");
      loadDefs();
      setView("main");
    } catch (error: any) {
      toast.error(error.message || "Failed to create badge definition");
    } finally {
      setWorking(false);
    }
  };

  const handleIssueBadge = async () => {
    if (!agent?.did || !selectedDef || !recipientDid.trim() || working) return;
    setWorking(true);
    try {
      await agent.client.create(
        place.stream.badge.issuance,
        {
          did: recipientDid.trim(),
          badge: { uri: selectedDef.uri, cid: selectedDef.cid },
          createdAt: new Date().toISOString() as any,
        } as any,
        { repo: agent.did as any },
      );
      setLastResult({
        label: `Issued to ${recipientDid.trim()}`,
        uri: recipientDid.trim(),
      });
      setRecipientDid("");
      toast.success("Badge issued");
      setSelectedDef(null);
      setView("main");
    } catch (error: any) {
      toast.error(error.message || "Failed to issue badge");
    } finally {
      setWorking(false);
    }
  };

  // Issue view
  if (view === "issue" && selectedDef) {
    return (
      <div className="space-y-6">
        <button
          type="button"
          onClick={() => {
            setSelectedDef(null);
            setView("main");
          }}
          className="flex items-center gap-2 text-sm text-(--color-fg-muted) transition-colors hover:text-(--color-fg)"
        >
          <ChevronLeft size={16} />
          {t("issue-badges-back-to-definitions")}
        </button>

        <div>
          <h1 className="font-display text-lg font-semibold">
            {t("issue-badges-issue-badge")}
          </h1>
          <p className="text-sm text-(--color-fg-muted)">
            {t("issue-badges-issue-badge-description", {
              name: selectedDef.value.name,
            })}
          </p>
        </div>

        <BadgeDefRow def={selectedDef} />

        <div className="space-y-3">
          <label className="block text-xs font-medium text-(--color-fg-muted)">
            {t("issue-badges-recipient-did")}
          </label>
          <Input
            value={recipientDid}
            onChange={(e) => setRecipientDid(e.target.value)}
            placeholder={t("issue-badges-recipient-did-placeholder")}
          />
          <Button
            onClick={handleIssueBadge}
            disabled={!recipientDid.trim() || working}
            className="w-full"
          >
            {working ? "Issuing…" : t("issue-badges-issue-badge")}
          </Button>
        </div>
      </div>
    );
  }

  // Create view
  if (view === "create") {
    return (
      <div className="space-y-6">
        <button
          type="button"
          onClick={() => setView("main")}
          className="flex items-center gap-2 text-sm text-(--color-fg-muted) transition-colors hover:text-(--color-fg)"
        >
          <ChevronLeft size={16} />
          {t("issue-badges-back-to-definitions")}
        </button>

        <div>
          <h1 className="font-display text-lg font-semibold">
            {t("issue-badges-create-definition")}
          </h1>
          <p className="text-sm text-(--color-fg-muted)">
            {t("issue-badges-create-definition-description")}
          </p>
        </div>

        <div className="space-y-4">
          {/* Badge type */}
          <div>
            <label className="mb-2 block text-xs font-medium text-(--color-fg-muted)">
              {t("issue-badges-badge-type")}
            </label>
            <div className="flex gap-2">
              {BADGE_TYPE_OPTIONS.map(({ label, value }) => (
                <Button
                  key={value}
                  variant={createBadgeType === value ? "default" : "secondary"}
                  size="sm"
                  onClick={() => setCreateBadgeType(value)}
                >
                  {label}
                </Button>
              ))}
            </div>
          </div>

          {/* Name */}
          <div>
            <label className="mb-1 block text-xs font-medium text-(--color-fg-muted)">
              {t("issue-badges-badge-name")}
            </label>
            <Input
              value={createName}
              onChange={(e) => setCreateName(e.target.value)}
              placeholder={t("issue-badges-badge-name-placeholder")}
              maxLength={64}
            />
          </div>

          {/* Description */}
          <div>
            <label className="mb-1 block text-xs font-medium text-(--color-fg-muted)">
              {t("issue-badges-description-optional")}
            </label>
            <Input
              value={createDescription}
              onChange={(e) => setCreateDescription(e.target.value)}
              placeholder={t("issue-badges-description-placeholder")}
              maxLength={256}
            />
          </div>

          {/* Image */}
          <div>
            <label className="mb-2 block text-xs font-medium text-(--color-fg-muted)">
              {t("issue-badges-image-optional")}
            </label>
            <div className="flex items-center gap-3">
              {createImageUri ? (
                <div className="flex items-center gap-2">
                  <img
                    src={createImageUri}
                    alt=""
                    className="h-12 w-12 rounded"
                  />
                  <button
                    type="button"
                    onClick={() => {
                      setCreateImageUri(null);
                      setCreateImageBlob(null);
                    }}
                    className="rounded p-1 hover:bg-(--color-bg)"
                  >
                    <X size={16} className="text-(--color-fg-muted)" />
                  </button>
                </div>
              ) : (
                <Button variant="secondary" size="sm" onClick={pickImage}>
                  <ImagePlus size={14} className="mr-1" />
                  {t("issue-badges-choose-image")}
                </Button>
              )}
            </div>
          </div>

          <Button
            onClick={handleCreateDef}
            disabled={!createName.trim() || working}
            className="w-full"
          >
            {working ? "Creating…" : t("issue-badges-create-definition")}
          </Button>
        </div>
      </div>
    );
  }

  // Main view
  return (
    <div className="space-y-6">
      <p className="text-sm text-(--color-fg-muted)">
        {t("issue-badges-manage-description")}
      </p>

      {/* Create new definition */}
      <button
        type="button"
        onClick={() => setView("create")}
        className="flex w-full items-center gap-3 rounded-lg border border-(--color-border) bg-(--color-bg-elevated) px-3 py-3 transition-colors hover:bg-(--color-bg)"
      >
        <div className="flex h-8 w-8 items-center justify-center rounded-full bg-(--color-accent)/10">
          <Plus size={16} className="text-(--color-accent)" />
        </div>
        <div className="text-left">
          <div className="text-sm font-medium">
            {t("issue-badges-create-definition")}
          </div>
          <div className="text-xs text-(--color-fg-muted)">
            {t("issue-badges-create-definition-subtitle")}
          </div>
        </div>
      </button>

      {/* Last result */}
      {lastResult && (
        <div className="flex items-center gap-2 rounded-lg border border-green-500/20 bg-green-500/10 px-3 py-2">
          <Check size={14} className="shrink-0 text-green-500" />
          <div className="min-w-0">
            <div className="text-sm font-medium text-green-500">
              {lastResult.label}
            </div>
            <div className="truncate font-mono text-xs text-(--color-fg-muted)">
              {lastResult.uri}
            </div>
          </div>
        </div>
      )}

      {/* Badge definitions list */}
      {loadingDefs ? (
        <div className="text-sm text-(--color-fg-muted)">Loading…</div>
      ) : defs.length > 0 ? (
        <section>
          <h2 className="font-display mb-1 text-sm font-semibold">
            {t("issue-badges-your-definitions")}
          </h2>
          <p className="mb-3 text-xs text-(--color-fg-muted)">
            {t("issue-badges-tap-to-issue")}
          </p>
          <div className="divide-y divide-(--color-border) rounded-lg border border-(--color-border) bg-(--color-bg-elevated)">
            {defs.map((def) => (
              <button
                key={def.uri}
                type="button"
                onClick={() => {
                  setSelectedDef(def);
                  setView("issue");
                }}
                className="flex w-full items-center gap-3 px-3 py-2.5 text-left transition-colors hover:bg-(--color-bg)"
              >
                {def.value.image ? (
                  <img
                    src={`https://cdn.bsky.app/img/feed_fullsize/plain/${getDidFromAtUri(def.uri)}/${def.value.image.ref.toString()}`}
                    alt=""
                    className="h-7 w-7 rounded"
                  />
                ) : (
                  <div className="flex h-7 w-7 items-center justify-center rounded bg-(--color-bg) text-xs">
                    🎭
                  </div>
                )}
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium">{def.value.name}</div>
                  <div className="text-xs text-(--color-fg-muted)">
                    {def.value.badgeType.split("#")[1]}
                  </div>
                </div>
              </button>
            ))}
          </div>
        </section>
      ) : null}
    </div>
  );
}

function BadgeDefRow({ def }: { def: BadgeDefItem }) {
  return (
    <div className="flex items-center gap-3 rounded-lg border border-(--color-border) bg-(--color-bg-elevated) px-3 py-2.5">
      {def.value.image ? (
        <img
          src={`https://cdn.bsky.app/img/feed_fullsize/plain/${getDidFromAtUri(def.uri)}/${def.value.image.ref.toString()}`}
          alt=""
          className="h-7 w-7 rounded"
        />
      ) : (
        <div className="flex h-7 w-7 items-center justify-center rounded bg-(--color-bg) text-xs">
          🎭
        </div>
      )}
      <div>
        <div className="text-sm font-medium">{def.value.name}</div>
        <div className="text-xs text-(--color-fg-muted)">
          {def.value.badgeType.split("#")[1]}
        </div>
      </div>
    </div>
  );
}
