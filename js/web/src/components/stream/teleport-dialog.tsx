import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import useAvatars from "@/hooks/use-avatars";
import { useLiveUsers } from "@/hooks/use-live-users";
import type { TeleportErrorData } from "@streamplace/core";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

function translateTeleportError(
  error: TeleportErrorData,
  t: ReturnType<typeof useTranslation>["t"],
) {
  switch (error.code) {
    case "streamer-only":
      return t("teleport-error-streamer-only");
    case "handle-format":
      return t("teleport-error-handle-format");
    case "countdown-number":
      return t("teleport-error-countdown-number");
    case "countdown-range":
      return t("teleport-countdown-error");
    case "self":
      return t("teleport-error-self");
    case "resolve-handle":
      return t("teleport-error-resolve-handle", {
        handle: error.params?.handle,
      });
    case "create":
      return t("teleport-error-create");
  }
}

export function TeleportDialog({
  open,
  onOpenChange,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (
    handle: string,
    countdownSeconds: number,
  ) => Promise<TeleportErrorData | undefined>;
}) {
  const { t } = useTranslation("common");
  const [query, setQuery] = useState("");
  const [countdown, setCountdown] = useState("10");
  const [selected, setSelected] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const { data: liveUsers = [], isLoading } = useLiveUsers();
  const selectedStream = useMemo(
    () => liveUsers.find((stream) => stream.uri === selected),
    [liveUsers, selected],
  );
  useEffect(() => {
    if (!selected || selectedStream) return;
    setSelected(null);
    setError(t("teleport-selection-expired"));
  }, [selected, selectedStream, t]);
  const authorDids = useMemo(
    () =>
      liveUsers.map((stream) => stream.author?.did).filter(Boolean) as string[],
    [liveUsers],
  );
  const profiles = useAvatars(authorDids);
  const filteredUsers = useMemo(() => {
    const filter = query.trim().toLowerCase();
    if (!filter) return liveUsers;
    return liveUsers.filter(
      (stream) =>
        stream.author?.handle?.toLowerCase().includes(filter) ||
        String(stream.record.title || "")
          .toLowerCase()
          .includes(filter),
    );
  }, [liveUsers, query]);

  const close = () => {
    setQuery("");
    setCountdown("10");
    setSelected(null);
    setError(null);
    onOpenChange(false);
  };

  const submit = async () => {
    const stream = liveUsers.find((candidate) => candidate.uri === selected);
    const seconds = Number.parseInt(countdown, 10);
    if (!stream?.author?.handle) {
      setSelected(null);
      setError(t("teleport-selection-expired"));
      return;
    }
    if (!Number.isInteger(seconds) || seconds < 5 || seconds > 300) {
      setError(t("teleport-countdown-error"));
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const teleportError = await onSubmit(stream.author.handle, seconds);
      if (teleportError) {
        setError(translateTeleportError(teleportError, t));
        return;
      }
      close();
    } catch {
      setError(t("teleport-error-create"));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && close()}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("teleport-dialog-title")}</DialogTitle>
          <DialogDescription>
            {t("teleport-dialog-description")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <label className="block">
            <span className="sr-only">{t("teleport-search-label")}</span>
            <input
              autoComplete="off"
              className="h-10 w-full rounded-md border border-(--color-border) bg-(--color-bg) px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-(--color-focus) focus-visible:ring-offset-2"
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t("teleport-search-placeholder")}
              value={query}
            />
          </label>

          {isLoading && liveUsers.length === 0 ? (
            <p className="py-8 text-center text-sm text-(--color-fg-muted)">
              {t("teleport-loading-streamers")}
            </p>
          ) : filteredUsers.length === 0 ? (
            <p className="py-8 text-center text-sm text-(--color-fg-muted)">
              {liveUsers.length === 0
                ? t("teleport-empty-streamers")
                : t("teleport-no-matching-streamers")}
            </p>
          ) : (
            <div className="grid max-h-80 grid-cols-1 gap-2 overflow-y-auto sm:grid-cols-2">
              {filteredUsers.map((stream) => {
                const did = stream.author?.did;
                const handle =
                  stream.author?.handle ||
                  did ||
                  t("teleport-unknown-streamer");
                const profile = did ? profiles[did] : undefined;
                const isSelected = stream.uri === selected;
                return (
                  <button
                    key={stream.uri}
                    type="button"
                    aria-pressed={isSelected}
                    onClick={() => {
                      setSelected(stream.uri);
                      setError(null);
                    }}
                    className={`flex min-w-0 items-center gap-3 rounded-md border p-3 text-left transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--color-focus) ${isSelected ? "border-(--color-accent) bg-(--color-bg-overlay)" : "border-(--color-border) hover:bg-(--color-bg-overlay)"}`}
                  >
                    {profile?.avatar ? (
                      <img
                        src={profile.avatar}
                        alt=""
                        className="size-10 shrink-0 rounded-full object-cover"
                      />
                    ) : (
                      <span className="size-10 shrink-0 rounded-full bg-(--color-bg-elevated)" />
                    )}
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium">
                        @{handle}
                      </span>
                      {stream.record.title && (
                        <span className="block truncate text-xs text-(--color-fg-muted)">
                          {String(stream.record.title)}
                        </span>
                      )}
                    </span>
                    {stream.viewerCount && (
                      <span className="shrink-0 text-xs text-(--color-fg-muted)">
                        {t("teleport-viewer-count", {
                          count: stream.viewerCount.count,
                        })}
                      </span>
                    )}
                  </button>
                );
              })}
            </div>
          )}

          <label className="flex items-center gap-3 text-sm">
            <span className="text-(--color-fg-muted)">
              {t("teleport-countdown")}
            </span>
            <input
              type="number"
              min={5}
              max={300}
              value={countdown}
              onChange={(event) => setCountdown(event.target.value)}
              className="h-10 w-28 rounded-md border border-(--color-border) bg-(--color-bg) px-3 outline-none focus-visible:ring-2 focus-visible:ring-(--color-focus) focus-visible:ring-offset-2"
            />
            <span className="text-(--color-fg-muted)">
              {t("teleport-seconds-range")}
            </span>
          </label>
          {error && (
            <p role="alert" className="text-sm text-(--color-danger)">
              {error}
            </p>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={close} disabled={submitting}>
            {t("cancel")}
          </Button>
          <Button
            onClick={submit}
            disabled={!selectedStream?.author?.handle || submitting}
          >
            {submitting ? t("teleport-starting") : t("teleport-start")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
