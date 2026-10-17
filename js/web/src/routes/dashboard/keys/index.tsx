import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { createFileRoute, Link } from "@tanstack/react-router";
import { Trash2, X } from "lucide-react";
import { useEffect, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { toast } from "sonner";
import { useStore } from "../../../lib/store";
import { useKeyRecords } from "../../../lib/store/hooks";

export const Route = createFileRoute("/dashboard/keys/")({
  component: KeyManager,
});

function timeAgo(date: Date): string {
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function KeyManager() {
  const { t } = useTranslation("settings");

  const deleteStreamKeyRecord = useStore((s) => s.deleteStreamKeyRecord);
  const getStreamKeyRecords = useStore((s) => s.getStreamKeyRecords);
  const keyObj = useKeyRecords();
  const keyRecords = keyObj?.records ?? null;

  const [deletingKeys, setDeletingKeys] = useState<Set<string>>(new Set());
  const [showDeleteAllDialog, setShowDeleteAllDialog] = useState(false);

  const handleDelete = async (rkey: string) => {
    if (deletingKeys.has(rkey)) return;
    setDeletingKeys((prev) => new Set(prev).add(rkey));
    try {
      await deleteStreamKeyRecord(rkey);
    } catch (error: any) {
      toast.error(error.message || "Failed to delete key");
    } finally {
      setDeletingKeys((prev) => {
        const next = new Set(prev);
        next.delete(rkey);
        return next;
      });
    }
  };

  const handleDeleteAll = async () => {
    setShowDeleteAllDialog(false);
    const rkeys =
      keyRecords?.records.map((r) => r.uri.split("/").pop() as string) ?? [];
    if (rkeys.length === 0) return;
    setDeletingKeys(new Set(rkeys));
    let failed = 0;
    for (const rkey of rkeys) {
      try {
        await deleteStreamKeyRecord(rkey);
      } catch {
        failed++;
      }
    }
    if (failed > 0) {
      toast.error(`Failed to delete ${failed} key${failed > 1 ? "s" : ""}`);
    }
  };

  useEffect(() => {
    const timeout = setTimeout(() => {
      getStreamKeyRecords();
    }, 500);
    return () => clearTimeout(timeout);
  }, []);

  return (
    <div className="mx-auto w-full max-w-2xl space-y-6 py-4">
      <h1 className="font-display text-xl font-semibold">{t("key-manager")}</h1>

      {keyRecords === null || keyObj === null ? (
        <div className="text-sm text-(--color-fg-muted)">Loading…</div>
      ) : keyRecords.records.length === 0 ? (
        <div className="space-y-3">
          <p className="text-sm text-(--color-fg-muted)">{t("no-keys")}</p>
          <p>
            <Trans key="create-stream-key-in-stream-settings">
              Create a stream key in{" "}
              <Link
                to="/dashboard/stream"
                className="font-semibold hover:underline"
              >
                Stream Settings
              </Link>
            </Trans>
          </p>
          <button
            type="button"
            onClick={() => getStreamKeyRecords()}
            className="h-8 rounded-md border border-(--color-border) px-3 text-sm hover:border-(--color-border-strong)"
          >
            {t("refresh")}
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">{t("your-stream-pubkeys")}</p>
              <p className="mt-0.5 text-xs text-(--color-fg-muted)">
                {t("pubkey-description")}
              </p>
            </div>
            <button
              type="button"
              onClick={() => setShowDeleteAllDialog(true)}
              disabled={deletingKeys.size > 0}
              className="flex h-8 shrink-0 items-center gap-1.5 rounded-md border border-(--color-border) px-3 text-sm text-(--color-fg-muted) transition-colors hover:border-red-500/50 hover:text-red-500 disabled:opacity-50"
            >
              <Trash2 size={14} />
              {t("delete-all-keys", { defaultValue: "Delete All Keys" })}
            </button>
          </div>

          <div className="divide-y divide-(--color-border) rounded-lg border border-(--color-border) bg-(--color-bg-elevated)">
            {keyRecords.records.map((keyRecord) => {
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
                        made {timeAgo(new Date(value.createdAt))}
                        {value.createdBy && ` by ${value.createdBy}`}
                      </div>
                    )}
                  </div>
                  <button
                    type="button"
                    onClick={() => handleDelete(rkey)}
                    disabled={isDeleting}
                    className="flex h-6 w-6 shrink-0 items-center justify-center rounded bg-(--color-bg) transition-colors hover:bg-(--color-border) disabled:opacity-50"
                  >
                    <X size={14} />
                  </button>
                </div>
              );
            })}
          </div>

          <p className="text-xs text-(--color-fg-muted)">
            {t("keys-count", { count: keyRecords.records.length })}
          </p>
        </div>
      )}

      <Dialog open={showDeleteAllDialog} onOpenChange={setShowDeleteAllDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete all stream keys?</DialogTitle>
            <DialogDescription>
              This will permanently remove all {keyRecords?.records.length ?? 0}{" "}
              stream keys. Any streaming software configured with these keys
              will stop working.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <DialogClose className="inline-flex h-9 items-center justify-center rounded-md border border-(--color-border) px-4 text-sm font-medium transition-colors hover:bg-(--color-bg-elevated)">
              Cancel
            </DialogClose>
            <button
              type="button"
              onClick={handleDeleteAll}
              className="inline-flex h-9 items-center justify-center rounded-md bg-red-500 px-4 text-sm font-medium text-white transition-colors hover:bg-red-600"
            >
              Delete All
            </button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
