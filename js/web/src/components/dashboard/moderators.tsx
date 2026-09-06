import useAvatars from "@/hooks/use-avatars";
import { useToast } from "@/hooks/use-toast";
import { useSession } from "@/lib/session";
import { Plus, Shield, ShieldOff, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { place } from "streamplace";
import { Button } from "../ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../ui/dialog";
import { Input } from "../ui/input";
import { Switch } from "../ui/switch";

const PERMISSION_RECORD = "place.stream.moderation.permission";

interface ModeratorRecord {
  uri: string;
  rkey: string;
  value: place.stream.moderation.permission.Main;
}

const PERMISSION_OPTIONS = [
  "ban",
  "hide",
  "livestream.manage",
  "message.pin",
] as const;

/**
 * Manage delegation of stream moderation: lists the
 * place.stream.moderation.permission records in the logged-in user's repo
 * and adds/removes them. Operates on the session's own repo, so the
 * dashboard's stream store is not needed.
 */
export function ModeratorsWidget() {
  const { t } = useTranslation("common");
  const { pdsAgent, did } = useSession();
  const toast = useToast();

  const [moderators, setModerators] = useState<ModeratorRecord[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showAdd, setShowAdd] = useState(false);
  const [removing, setRemoving] = useState<ModeratorRecord | null>(null);

  const moderatorDids = useMemo(
    () => moderators.map((m) => m.value.moderator),
    [moderators],
  );
  const profiles = useAvatars(moderatorDids);

  const refresh = useCallback(async () => {
    if (!pdsAgent?.did) {
      setModerators([]);
      setError(null);
      return;
    }
    setIsLoading(true);
    try {
      const result = await pdsAgent.com.atproto.repo.listRecords({
        repo: pdsAgent.did,
        collection: PERMISSION_RECORD,
        limit: 100,
      });
      const records = (result.data.records ?? [])
        .filter((r) => r.value?.$type === PERMISSION_RECORD)
        .map((r) => ({
          uri: r.uri,
          rkey: r.uri.split("/").pop() ?? "",
          value: r.value as place.stream.moderation.permission.Main,
        }));
      setModerators(records);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setModerators([]);
    } finally {
      setIsLoading(false);
    }
  }, [pdsAgent]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const handleRemove = useCallback(async () => {
    if (!pdsAgent?.did || !removing) return;
    try {
      await pdsAgent.com.atproto.repo.deleteRecord({
        repo: pdsAgent.did,
        collection: PERMISSION_RECORD,
        rkey: removing.rkey,
      });
      toast.show(
        t("moderators-removed-toast", {
          handle: handleFor(removing.value.moderator, profiles),
        }),
        "",
        { duration: 3000 },
      );
      setRemoving(null);
      refresh();
    } catch (e) {
      toast.show(
        t("moderators-remove-failed"),
        e instanceof Error ? e.message : String(e),
        { duration: 5000 },
      );
    }
  }, [pdsAgent, refresh, removing, t, toast, profiles]);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex items-center justify-between gap-2 border-b border-(--color-border) p-4">
        <div className="flex items-center gap-2">
          <Shield className="size-4" />
          <span className="font-semibold">{t("moderators-title")}</span>
        </div>
        <Button size="sm" onClick={() => setShowAdd(true)}>
          <Plus /> {t("moderators-add")}
        </Button>
      </div>

      <div className="min-h-0 flex-1 space-y-2 overflow-y-auto p-4">
        {error && (
          <p className="border-destructive/40 bg-destructive/10 text-destructive rounded-md border p-2 text-sm">
            {error}
          </p>
        )}
        {isLoading && moderators.length === 0 && (
          <p className="py-4 text-center text-sm text-(--color-fg-muted)">
            {t("moderators-loading")}
          </p>
        )}
        {!isLoading && moderators.length === 0 && !error && (
          <div className="flex flex-col items-center gap-1 py-8 text-center">
            <ShieldOff className="mb-2 size-8 text-(--color-fg-muted)" />
            <p className="text-sm font-medium">{t("moderators-empty")}</p>
            <p className="text-xs text-(--color-fg-muted)">
              {t("moderators-empty-hint")}
            </p>
          </div>
        )}
        {moderators.map((mod) => (
          <ModeratorRow
            key={mod.rkey}
            moderator={mod}
            profiles={profiles}
            onRemove={() => setRemoving(mod)}
          />
        ))}
      </div>

      <AddModeratorDialog
        open={showAdd}
        onClose={() => setShowAdd(false)}
        onAdded={() => {
          setShowAdd(false);
          refresh();
        }}
      />

      <Dialog
        open={removing !== null}
        onOpenChange={(open) => {
          if (!open) setRemoving(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("moderators-remove-title")}</DialogTitle>
            <DialogDescription>
              {t("moderators-remove-description", {
                handle: removing
                  ? handleFor(removing.value.moderator, profiles)
                  : "",
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoving(null)}>
              {t("moderators-cancel")}
            </Button>
            <Button variant="destructive" onClick={handleRemove}>
              {t("moderators-remove")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function handleFor(
  did: string,
  profiles: ReturnType<typeof useAvatars>,
): string {
  return profiles[did]?.handle ?? did;
}

function ModeratorRow({
  moderator,
  profiles,
  onRemove,
}: {
  moderator: ModeratorRecord;
  profiles: ReturnType<typeof useAvatars>;
  onRemove: () => void;
}) {
  const { t } = useTranslation("common");
  const handle = handleFor(moderator.value.moderator, profiles);
  const avatar = profiles[moderator.value.moderator]?.avatar;
  const isExpired =
    !!moderator.value.expirationTime &&
    new Date(moderator.value.expirationTime) <= new Date();

  return (
    <div className="flex items-center gap-3 rounded-md border border-(--color-border) bg-(--color-bg-elevated) p-3">
      {avatar && (
        <img
          src={avatar}
          alt=""
          className="size-8 shrink-0 rounded-full bg-(--color-bg-overlay)"
        />
      )}
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">@{handle}</p>
        <div className="mt-1 flex flex-wrap gap-1">
          {moderator.value.permissions?.map((perm) => (
            <span
              key={perm}
              className="rounded-sm border border-(--color-border) px-1.5 py-0.5 text-[11px] text-(--color-fg-muted)"
            >
              {perm}
            </span>
          ))}
          {isExpired && (
            <span className="border-destructive/40 bg-destructive/10 text-destructive rounded-sm border px-1.5 py-0.5 text-[11px]">
              {t("moderators-expired")}
            </span>
          )}
          {moderator.value.expirationTime && !isExpired && (
            <span className="px-1.5 py-0.5 text-[11px] text-(--color-fg-muted)">
              {t("moderators-expires", {
                date: new Date(
                  moderator.value.expirationTime,
                ).toLocaleDateString(),
              })}
            </span>
          )}
        </div>
      </div>
      <Button
        variant="ghost"
        size="icon"
        onClick={onRemove}
        aria-label={t("moderators-remove")}
      >
        <Trash2 />
      </Button>
    </div>
  );
}

function AddModeratorDialog({
  open,
  onClose,
  onAdded,
}: {
  open: boolean;
  onClose: () => void;
  onAdded: () => void;
}) {
  const { t } = useTranslation("common");
  const { pdsAgent } = useSession();
  const toast = useToast();
  const [handleOrDid, setHandleOrDid] = useState("");
  const [permissions, setPermissions] = useState<Set<string>>(new Set());
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Reset the form whenever the dialog closes so a reopened dialog
  // never shows the previous submission.
  useEffect(() => {
    if (!open) {
      setHandleOrDid("");
      setPermissions(new Set());
      setError(null);
    }
  }, [open]);

  const togglePermission = useCallback((perm: string) => {
    setPermissions((prev) => {
      const next = new Set(prev);
      if (next.has(perm)) {
        next.delete(perm);
      } else {
        next.add(perm);
      }
      return next;
    });
  }, []);

  const handleAdd = useCallback(async () => {
    if (!pdsAgent?.did) return;
    const input = handleOrDid.trim().replace(/^@/, "");
    if (!input) {
      setError(t("moderators-error-required"));
      return;
    }
    if (permissions.size === 0) {
      setError(t("moderators-error-permissions"));
      return;
    }
    setIsSaving(true);
    setError(null);
    try {
      let moderatorDid = input;
      if (!moderatorDid.startsWith("did:")) {
        const resolved = await pdsAgent.com.atproto.identity.resolveHandle({
          handle: moderatorDid,
        });
        moderatorDid = resolved.data.did;
      }
      await pdsAgent.com.atproto.repo.createRecord({
        repo: pdsAgent.did,
        collection: PERMISSION_RECORD,
        record: {
          $type: PERMISSION_RECORD,
          moderator: moderatorDid as any,
          permissions: [...permissions],
          createdAt: new Date().toISOString(),
        },
      });
      toast.show(t("moderators-added-toast"), "", { duration: 3000 });
      onAdded();
    } catch (e) {
      setError(e instanceof Error ? e.message : t("moderators-add-failed"));
    } finally {
      setIsSaving(false);
    }
  }, [handleOrDid, onAdded, pdsAgent, permissions, t, toast]);

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("moderators-add-title")}</DialogTitle>
          <DialogDescription>
            {t("moderators-add-description")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-2">
            <label
              htmlFor="moderator-handle"
              className="text-sm text-(--color-fg-muted)"
            >
              {t("moderators-handle-label")}
            </label>
            <Input
              id="moderator-handle"
              value={handleOrDid}
              onChange={(e) => {
                setHandleOrDid(e.target.value);
                if (error) setError(null);
              }}
              placeholder="handle.bsky.social or did:plc:…"
              onKeyDown={(e) => {
                if (e.key === "Enter") handleAdd();
              }}
            />
          </div>

          <div className="space-y-2">
            <p className="text-sm text-(--color-fg-muted)">
              {t("moderators-permissions-label")}
            </p>
            {PERMISSION_OPTIONS.map((perm) => (
              <div
                key={perm}
                className="flex items-center justify-between gap-3 rounded-md border border-(--color-border) bg-(--color-bg-elevated) p-3"
              >
                <div>
                  <p className="text-sm font-medium">
                    {t(`moderators-permission-${perm}`)}
                  </p>
                  <p className="text-xs text-(--color-fg-muted)">
                    {t(`moderators-permission-${perm}-desc`)}
                  </p>
                </div>
                <Switch
                  checked={permissions.has(perm)}
                  onCheckedChange={() => togglePermission(perm)}
                  aria-label={t(`moderators-permission-${perm}`)}
                />
              </div>
            ))}
          </div>

          {error && (
            <p className="border-destructive/40 bg-destructive/10 text-destructive rounded-md border p-2 text-sm">
              {error}
            </p>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isSaving}>
            {t("moderators-cancel")}
          </Button>
          <Button onClick={handleAdd} disabled={isSaving}>
            {isSaving ? t("moderators-adding") : t("moderators-add")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
