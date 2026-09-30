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
import { useMemo, useState } from "react";

export function TeleportDialog({
  open,
  onOpenChange,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (handle: string, countdownSeconds: number) => Promise<void>;
}) {
  const [query, setQuery] = useState("");
  const [countdown, setCountdown] = useState("10");
  const [selected, setSelected] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const { data: liveUsers = [], isLoading } = useLiveUsers();
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
    if (!stream?.author?.handle) return;
    if (!Number.isInteger(seconds) || seconds < 5 || seconds > 300) {
      setError("Countdown must be between 5 and 300 seconds.");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit(stream.author.handle, seconds);
      close();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Teleport failed.");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && close()}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Teleport to another live streamer</DialogTitle>
          <DialogDescription>
            Select a streamer to teleport your viewers to their stream.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <label className="block">
            <span className="sr-only">Search live streams</span>
            <input
              autoComplete="off"
              className="h-10 w-full rounded-md border border-(--color-border) bg-(--color-bg) px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-(--color-focus) focus-visible:ring-offset-2"
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search by handle or title"
              value={query}
            />
          </label>

          {isLoading && liveUsers.length === 0 ? (
            <p className="py-8 text-center text-sm text-(--color-fg-muted)">
              Loading live streamers…
            </p>
          ) : filteredUsers.length === 0 ? (
            <p className="py-8 text-center text-sm text-(--color-fg-muted)">
              {liveUsers.length === 0
                ? "No live streamers found."
                : "No matching live streamers found."}
            </p>
          ) : (
            <div className="grid max-h-80 grid-cols-1 gap-2 overflow-y-auto sm:grid-cols-2">
              {filteredUsers.map((stream) => {
                const did = stream.author?.did;
                const handle = stream.author?.handle || did || "Unknown";
                const profile = did ? profiles[did] : undefined;
                const isSelected = stream.uri === selected;
                return (
                  <button
                    key={stream.uri}
                    type="button"
                    aria-pressed={isSelected}
                    onClick={() => setSelected(stream.uri)}
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
                        {stream.viewerCount.count} viewers
                      </span>
                    )}
                  </button>
                );
              })}
            </div>
          )}

          <label className="flex items-center gap-3 text-sm">
            <span className="text-(--color-fg-muted)">Countdown</span>
            <input
              type="number"
              min={5}
              max={300}
              value={countdown}
              onChange={(event) => setCountdown(event.target.value)}
              className="h-10 w-28 rounded-md border border-(--color-border) bg-(--color-bg) px-3 outline-none focus-visible:ring-2 focus-visible:ring-(--color-focus) focus-visible:ring-offset-2"
            />
            <span className="text-(--color-fg-muted)">seconds (5–300)</span>
          </label>
          {error && (
            <p role="alert" className="text-sm text-(--color-danger)">
              {error}
            </p>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={close} disabled={submitting}>
            Cancel
          </Button>
          <Button onClick={submit} disabled={!selected || submitting}>
            {submitting ? "Starting…" : "Teleport"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
