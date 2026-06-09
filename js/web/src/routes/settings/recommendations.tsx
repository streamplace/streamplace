import { BackLink } from "@/components/settings/back-link";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { createFileRoute } from "@tanstack/react-router";
import {
  ArrowDown,
  ArrowUp,
  Check,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  X,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { usePDSAgent } from "../../lib/store/hooks";

export const Route = createFileRoute("/settings/recommendations")({
  component: RecommendationsManager,
});

interface ActorSearchResult {
  did: string;
  handle: string;
}

function RecommendationsManager() {
  const { t } = useTranslation("settings");
  const agent = usePDSAgent();

  const [streamers, setStreamers] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  // Search state
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<ActorSearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [searchTimeout, setSearchTimeout] = useState<ReturnType<
    typeof setTimeout
  > | null>(null);

  // Inline edit state
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [editValue, setEditValue] = useState("");

  // Delete dialog
  const [deleteIndex, setDeleteIndex] = useState<number | null>(null);

  const loadRecommendations = async () => {
    if (!agent) return;
    try {
      setLoading(true);
      const userDID = agent.did;
      if (!userDID) {
        setStreamers([]);
        return;
      }

      const response = await agent.com.atproto.repo.getRecord({
        repo: userDID,
        collection: "place.stream.live.recommendations",
        rkey: "self",
      });

      const record = response.data.value as { streamers?: string[] };
      setStreamers(record.streamers || []);
    } catch (error: any) {
      if (error.status !== 404) {
        console.error("Failed to load recommendations:", error);
        toast.error("Failed to load recommendations");
      }
      setStreamers([]);
    } finally {
      setLoading(false);
    }
  };

  const saveRecommendations = async (newStreamers: string[]) => {
    if (!agent || saving) return;
    try {
      if (!agent.did) throw new Error("User DID not found");
      setSaving(true);

      await agent.com.atproto.repo.putRecord({
        repo: agent.did,
        collection: "place.stream.live.recommendations",
        rkey: "self",
        record: {
          streamers: newStreamers,
          createdAt: new Date().toISOString(),
        },
      });

      setStreamers(newStreamers);
    } catch (error: any) {
      console.error("Failed to save recommendations:", error);
      toast.error(error.message || "Failed to save recommendations");
      await loadRecommendations();
    } finally {
      setSaving(false);
    }
  };

  const searchActors = useCallback(
    async (query: string) => {
      if (!agent || !query.trim()) {
        setSearchResults([]);
        return;
      }
      try {
        setSearching(true);
        const response = await agent.place.stream.live.searchActorsTypeahead({
          q: query,
          limit: 10,
        });
        setSearchResults(
          response.data.actors.map((a: any) => ({
            did: a.did,
            handle: a.handle,
          })),
        );
      } catch (error) {
        console.error("Failed to search actors:", error);
        setSearchResults([]);
      } finally {
        setSearching(false);
      }
    },
    [agent],
  );

  const handleSearchChange = (query: string) => {
    setSearchQuery(query);
    if (searchTimeout) clearTimeout(searchTimeout);
    if (query.trim()) {
      const timeout = setTimeout(() => searchActors(query), 300);
      setSearchTimeout(timeout);
    } else {
      setSearchResults([]);
    }
  };

  const handleSelectActor = async (actor: ActorSearchResult) => {
    if (streamers.length >= 8) {
      toast.error("You can only add up to 8 recommendations.");
      return;
    }
    if (streamers.includes(actor.did)) {
      toast.error("This streamer is already in your recommendations.");
      return;
    }
    await saveRecommendations([...streamers, actor.did]);
    setSearchQuery("");
    setSearchResults([]);
  };

  const handleAddManual = () => {
    if (streamers.length >= 8) {
      toast.error("You can only add up to 8 recommendations.");
      return;
    }
    const newIndex = streamers.length;
    setStreamers([...streamers, ""]);
    setEditingIndex(newIndex);
    setEditValue("");
  };

  const handleSaveEdit = async () => {
    if (editingIndex === null) return;
    const trimmed = editValue.trim();
    if (!trimmed) return;
    if (!trimmed.startsWith("did:")) {
      toast.error("DID must start with 'did:'");
      return;
    }
    const newStreamers = [...streamers];
    newStreamers[editingIndex] = trimmed;
    await saveRecommendations(newStreamers);
    setEditingIndex(null);
    setEditValue("");
  };

  const handleDelete = async () => {
    if (deleteIndex === null) return;
    const newStreamers = streamers.filter((_, i) => i !== deleteIndex);
    await saveRecommendations(newStreamers);
    setDeleteIndex(null);
  };

  const handleMove = async (from: number, to: number) => {
    if (to < 0 || to >= streamers.length) return;
    const newStreamers = [...streamers];
    const [item] = newStreamers.splice(from, 1);
    newStreamers.splice(to, 0, item);
    await saveRecommendations(newStreamers);
  };

  useEffect(() => {
    if (agent) loadRecommendations();
  }, [agent]);

  useEffect(() => {
    return () => {
      if (searchTimeout) clearTimeout(searchTimeout);
    };
  }, [searchTimeout]);

  if (!agent) {
    return (
      <div className="text-sm text-[var(--color-fg-muted)]">
        Please log in to manage recommendations.
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <BackLink to="/settings/streaming" label={t("streaming")} />

      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">
            {t("recommendations-to-others")}
          </h1>
          <p className="text-sm text-[var(--color-fg-muted)] mt-1">
            {t("recommendations-description")}
          </p>
        </div>
        <Button
          onClick={loadRecommendations}
          disabled={loading || saving}
          variant="secondary"
          size="sm"
        >
          <RefreshCw size={16} className="mr-1" />
          {t("refresh")}
        </Button>
      </div>

      {/* Search bar */}
      {streamers.length < 8 && (
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-3 space-y-2">
          <div className="flex items-center gap-2">
            <Search
              size={16}
              className="text-[var(--color-fg-muted)] shrink-0"
            />
            <Input
              value={searchQuery}
              onChange={(e) => handleSearchChange(e.target.value)}
              placeholder="Search for streamers..."
            />
          </div>

          {searching && (
            <div className="text-xs text-[var(--color-fg-muted)] py-1">
              Searching…
            </div>
          )}

          {!searching && searchResults.length > 0 && (
            <div className="divide-y divide-[var(--color-border)]">
              {searchResults.map((actor) => {
                const alreadyAdded = streamers.includes(actor.did);
                return (
                  <button
                    key={actor.did}
                    type="button"
                    onClick={() => !alreadyAdded && handleSelectActor(actor)}
                    disabled={alreadyAdded}
                    className="flex items-center justify-between w-full px-2 py-1.5 text-left hover:bg-[var(--color-bg)] transition-colors disabled:opacity-50 rounded"
                  >
                    <span className="text-sm">@{actor.handle}</span>
                    {alreadyAdded && (
                      <span className="text-xs text-[var(--color-fg-muted)]">
                        Added
                      </span>
                    )}
                  </button>
                );
              })}
            </div>
          )}

          {!searching && searchQuery.trim() && searchResults.length === 0 && (
            <div className="text-xs text-[var(--color-fg-muted)] py-1">
              No results found
            </div>
          )}
        </div>
      )}

      {/* Streamer list */}
      {loading ? (
        <div className="text-sm text-[var(--color-fg-muted)]">Loading…</div>
      ) : (
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] divide-y divide-[var(--color-border)]">
          {streamers.length === 0 ? (
            <div className="px-3 py-6 text-center text-sm text-[var(--color-fg-muted)]">
              {t("no-recommendations-yet")}
            </div>
          ) : (
            streamers.map((streamer, index) => (
              <div
                key={`${streamer}-${index}`}
                className="flex items-center gap-2 px-3 py-2"
              >
                {/* Reorder buttons */}
                <div className="flex flex-col shrink-0">
                  <button
                    type="button"
                    onClick={() => handleMove(index, index - 1)}
                    disabled={index === 0 || saving}
                    className="p-0.5 rounded hover:bg-[var(--color-bg)] disabled:opacity-30 transition-colors"
                  >
                    <ArrowUp
                      size={12}
                      className="text-[var(--color-fg-muted)]"
                    />
                  </button>
                  <button
                    type="button"
                    onClick={() => handleMove(index, index + 1)}
                    disabled={index === streamers.length - 1 || saving}
                    className="p-0.5 rounded hover:bg-[var(--color-bg)] disabled:opacity-30 transition-colors"
                  >
                    <ArrowDown
                      size={12}
                      className="text-[var(--color-fg-muted)]"
                    />
                  </button>
                </div>

                {/* Content */}
                {editingIndex === index ? (
                  <>
                    <Input
                      value={editValue}
                      onChange={(e) => setEditValue(e.target.value)}
                      placeholder="did:plc:..."
                      autoFocus
                      className="flex-1"
                    />
                    <button
                      type="button"
                      onClick={handleSaveEdit}
                      className="p-1.5 rounded hover:bg-[var(--color-bg)]"
                    >
                      <Check size={16} className="text-[var(--color-accent)]" />
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        if (!streamer) {
                          setStreamers((prev) =>
                            prev.filter((_, i) => i !== index),
                          );
                        }
                        setEditingIndex(null);
                      }}
                      className="p-1.5 rounded hover:bg-[var(--color-bg)]"
                    >
                      <X size={16} className="text-[var(--color-fg-muted)]" />
                    </button>
                  </>
                ) : (
                  <>
                    <span className="flex-1 text-sm truncate font-mono">
                      {streamer || "(empty)"}
                    </span>
                    <button
                      type="button"
                      onClick={() => {
                        setEditingIndex(index);
                        setEditValue(streamer);
                      }}
                      className="p-1.5 rounded hover:bg-[var(--color-bg)]"
                    >
                      <Pencil
                        size={16}
                        className="text-[var(--color-fg-muted)]"
                      />
                    </button>
                    <button
                      type="button"
                      onClick={() => setDeleteIndex(index)}
                      className="p-1.5 rounded hover:bg-[var(--color-bg)]"
                    >
                      <X size={16} className="text-destructive" />
                    </button>
                  </>
                )}
              </div>
            ))
          )}

          {streamers.length < 8 && (
            <button
              type="button"
              onClick={handleAddManual}
              className="flex items-center gap-2 px-3 py-2.5 w-full hover:bg-[var(--color-bg)] transition-colors"
            >
              <Plus size={16} className="text-[var(--color-fg-muted)]" />
              <span className="text-sm">Add DID manually</span>
            </button>
          )}
        </div>
      )}

      {saving && (
        <div className="text-xs text-[var(--color-fg-muted)]">
          {t("saving")}
        </div>
      )}

      {/* Delete confirmation */}
      <Dialog
        open={deleteIndex !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteIndex(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("delete")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm">{t("confirm-delete")}</p>
          <p className="text-xs text-[var(--color-fg-muted)] mt-1">
            {t("action-cannot-be-undone")}
          </p>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setDeleteIndex(null)}>
              {t("cancel")}
            </Button>
            <Button variant="destructive" onClick={handleDelete}>
              {t("delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
