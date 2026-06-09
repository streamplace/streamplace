import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { X } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { useStore } from "../../lib/store";
import { useKeyRecords } from "../../lib/store/hooks";

export const Route = createFileRoute("/settings/keys")({
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
  const navigate = useNavigate();

  const deleteStreamKeyRecord = useStore((s) => s.deleteStreamKeyRecord);
  const getStreamKeyRecords = useStore((s) => s.getStreamKeyRecords);
  const keyObj = useKeyRecords();
  const keyRecords = keyObj?.records ?? null;

  const [deletingKeys, setDeletingKeys] = useState<Set<string>>(new Set());

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

  useEffect(() => {
    const timeout = setTimeout(() => {
      getStreamKeyRecords();
    }, 500);
    return () => clearTimeout(timeout);
  }, []);

  return (
    <div className="space-y-6">
      <nav>
        <button
          type="button"
          onClick={() => navigate({ to: "/settings/streaming" })}
          className="flex items-center gap-2 text-sm text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            <path
              d="M10 3l-5 5 5 5"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          {t("streaming")}
        </button>
      </nav>

      <h1 className="text-xl font-semibold">{t("key-manager")}</h1>

      {keyRecords === null || keyObj === null ? (
        <div className="text-sm text-[var(--color-fg-muted)]">Loading…</div>
      ) : keyRecords.records.length === 0 ? (
        <div className="space-y-3">
          <p className="text-sm text-[var(--color-fg-muted)]">{t("no-keys")}</p>
          <button
            type="button"
            onClick={() => getStreamKeyRecords()}
            className="h-8 px-3 rounded-md border border-[var(--color-border)] hover:border-[var(--color-border-strong)] text-sm"
          >
            {t("refresh")}
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          <div>
            <p className="text-sm font-medium">{t("your-stream-pubkeys")}</p>
            <p className="text-xs text-[var(--color-fg-muted)] mt-0.5">
              {t("pubkey-description")}
            </p>
          </div>

          <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] divide-y divide-[var(--color-border)]">
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
                      <div className="text-xs font-mono truncate">
                        {value.signingKey}
                      </div>
                    )}
                    {value.createdAt && (
                      <div className="text-xs text-[var(--color-fg-muted)]">
                        made {timeAgo(new Date(value.createdAt))}
                        {value.createdBy && ` by ${value.createdBy}`}
                      </div>
                    )}
                  </div>
                  <button
                    type="button"
                    onClick={() => handleDelete(rkey)}
                    disabled={isDeleting}
                    className="shrink-0 w-6 h-6 rounded flex items-center justify-center bg-[var(--color-bg)] hover:bg-[var(--color-border)] transition-colors disabled:opacity-50"
                  >
                    <X size={14} />
                  </button>
                </div>
              );
            })}
          </div>

          <p className="text-xs text-[var(--color-fg-muted)]">
            {t("keys-count", { count: keyRecords.records.length })}
          </p>
        </div>
      )}
    </div>
  );
}
