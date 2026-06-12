import { CONTENT_WARNINGS } from "@/lib/content-warnings";
import { useStore as useGlobalStore } from "@/lib/store";
import { cn } from "@/lib/utils";
import type { LivestreamStore } from "@streamplace/core";
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ImagePlus,
  Loader2,
  Radio,
  Square,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { place } from "streamplace";
import { useStore } from "zustand";
import {
  usePDSAgent,
  useStreamplaceUrl,
  useUserProfile,
} from "../../lib/store/hooks";
import { Admonition } from "../ui/admonition";
import { Button } from "../ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";
import { Input } from "../ui/input";
import { Textarea } from "../ui/textarea";
import { ActivityPicker } from "./activity-picker";
import { useDashboardStore } from "./dashboard-store-context";

const LANGUAGES = [
  { code: "en", label: "English" },
  { code: "es", label: "Español" },
  { code: "fr", label: "Français" },
  { code: "de", label: "Deutsch" },
  { code: "pt", label: "Português" },
  { code: "ja", label: "日本語" },
  { code: "ko", label: "한국어" },
  { code: "zh", label: "中文" },
  { code: "ar", label: "العربية" },
  { code: "ru", label: "Русский" },
];

const LANG_TAG_PREFIX = "lang:";

/**
 * Stream info and control widget. Title, activity, tags, thumbnail,
 * content-warnings quick-edit, options, and create/update/end
 * livestream. The full metadata editor (with content rights and
 * distribution) lives at /dashboard/stream.
 */
export function StreamInfoWidget({ store }: { store: LivestreamStore }) {
  const { t } = useTranslation("common");
  const agent = usePDSAgent();
  const url = useStreamplaceUrl();
  const profile = useUserProfile();
  const liveStore = useDashboardStore();

  const livestream = useStore(store, (s) => s.livestream);
  const segment = useStore(store, (s) => s.segment);

  const isLive = !!segment;
  const hasLivestream = !!livestream;
  const isEnded = livestream?.record?.endedAt !== undefined;

  const [page, setPage] = useState<"info" | "contentWarnings">("info");

  // Form state
  const [title, setTitle] = useState("");
  const [activity, setActivity] = useState<
    place.stream.livestream.Main["activity"] | undefined
  >(undefined);
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");
  const [thumbnail, setThumbnail] = useState<Blob | undefined>();
  const [thumbnailPreview, setThumbnailPreview] = useState<
    string | undefined
  >();
  const [createPost, setCreatePost] = useState(true);
  const [idleTimeout, setIdleTimeout] = useState(true);
  const [loading, setLoading] = useState(false);
  const [endingLivestream, setEndingLivestream] = useState(false);
  const initializedRef = useRef(false);

  // Initialize form from existing livestream record
  useEffect(() => {
    if (!livestream || initializedRef.current) return;
    initializedRef.current = true;
    if (livestream.record.title) setTitle(livestream.record.title);
    if (livestream.record.activity) {
      setActivity(
        livestream.record.activity as place.stream.livestream.Main["activity"],
      );
    }
    if (livestream.record.tags) setTags(livestream.record.tags as string[]);
    if (typeof livestream.record.idleTimeoutSeconds === "number") {
      setIdleTimeout(livestream.record.idleTimeoutSeconds > 0);
    }
  }, [livestream]);

  // Reset ending state when livestream ends
  useEffect(() => {
    if (livestream && livestream.record.endedAt !== undefined) {
      setEndingLivestream(false);
    }
  }, [livestream]);

  const canSubmit = title.trim().length > 0 && !loading;
  const canEnd = hasLivestream && !isEnded && !endingLivestream;

  const buttonText = useMemo(() => {
    if (loading) return t("loading", { defaultValue: "Loading…" });
    if (!isLive)
      return t("waiting-for-stream", { defaultValue: "Waiting for stream…" });
    if (!hasLivestream || isEnded)
      return t("start-livestream", { defaultValue: "Start Livestream" });
    return t("update-livestream", { defaultValue: "Update Livestream" });
  }, [loading, isLive, hasLivestream, isEnded, t]);

  // Thumbnail handling
  const handleThumbnailSelect = useCallback(() => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "image/*";
    input.onchange = (e) => {
      const file = (e.target as HTMLInputElement).files?.[0];
      if (file) {
        setThumbnail(file);
        setThumbnailPreview(URL.createObjectURL(file));
      }
    };
    input.click();
  }, []);

  const handleThumbnailRemove = useCallback(() => {
    if (thumbnailPreview) URL.revokeObjectURL(thumbnailPreview);
    setThumbnail(undefined);
    setThumbnailPreview(undefined);
  }, [thumbnailPreview]);

  // Tag handling
  const handleAddTag = useCallback(
    (tag: string) => {
      const trimmed = tag.trim();
      if (trimmed && !tags.includes(trimmed) && tags.length < 10) {
        setTags([...tags, trimmed]);
      }
      setTagInput("");
    },
    [tags],
  );

  const handleRemoveTag = useCallback(
    (tag: string) => {
      setTags(tags.filter((t) => t !== tag));
    },
    [tags],
  );

  // Create/Update
  const handleSubmit = useCallback(async () => {
    if (!agent || !agent.did || !canSubmit) return;
    setLoading(true);

    try {
      const record = {
        $type: "place.stream.livestream",
        title: title.trim(),
        url: `${url}/${agent.did}` as any,
        createdAt: new Date().toISOString() as any,
        lastSeenAt: new Date().toISOString() as any,
        idleTimeoutSeconds: idleTimeout ? 300 : 0,
        activity,
        tags: tags.length > 0 ? tags : undefined,
      } as any;

      // Upload thumbnail if provided
      if (thumbnail) {
        try {
          const uploaded = await agent.uploadBlob(thumbnail);
          if (uploaded.success) {
            (record as any).thumb = uploaded.data.blob;
          }
        } catch (e) {
          console.error("Thumbnail upload failed:", e);
        }
      }

      if (!hasLivestream || isEnded) {
        // Create new livestream
        const result = await agent.client.call(place.stream.live.startLivestream, {
          livestream: record,
          streamer: agent.did as any,
          createBlueskyPost: createPost,
        });
        toast.success(
          t("livestream-announced", { defaultValue: "Livestream announced" }),
        );
      } else {
        // Update existing
        const rkey = livestream!.uri.split("/").pop();
        if (!rkey) throw new Error("No rkey");
        // Preserve existing thumb if no new one
        if (!record.thumb && livestream!.record.thumb) {
          record.thumb = livestream!.record.thumb;
        }
        record.post = livestream!.record.post;
        await agent.com.atproto.repo.putRecord({
          repo: agent.did,
          collection: "place.stream.livestream",
          rkey,
          record,
        });
        toast.success(
          t("livestream-updated", { defaultValue: "Livestream updated" }),
        );
      }
    } catch (error) {
      console.error("Error with livestream:", error);
      toast.error(String(error).slice(0, 200));
    } finally {
      setLoading(false);
    }
  }, [
    agent,
    url,
    title,
    activity,
    tags,
    thumbnail,
    createPost,
    idleTimeout,
    canSubmit,
    hasLivestream,
    isEnded,
    livestream,
    t,
  ]);

  const language = useMemo(() => {
    const langTag = tags.find((t) => t.startsWith(LANG_TAG_PREFIX));
    if (langTag) {
      const code = langTag.slice(LANG_TAG_PREFIX.length);
      return LANGUAGES.find((l) => l.code === code)?.label || null;
    }
    return null;
  }, [tags]);

  // End livestream
  const handleEnd = useCallback(async () => {
    if (!agent || !canEnd) return;
    setEndingLivestream(true);
    try {
      await agent.client.call(place.stream.live.stopLivestream, {});
      toast.success(
        t("livestream-ended", { defaultValue: "Livestream ended" }),
      );
    } catch (error) {
      console.error("Error ending livestream:", error);
      toast.error(
        t("failed-to-end", { defaultValue: "Failed to end livestream" }),
      );
      setEndingLivestream(false);
    }
  }, [agent, canEnd, t]);

  if (!agent) {
    return (
      <div className="rounded-b-lg border border-(--color-border) bg-(--color-bg-elevated) p-4 text-sm text-(--color-fg-muted)">
        {t("login-required", {
          defaultValue: "Please log in to manage streams.",
        })}
      </div>
    );
  }

  if (page === "contentWarnings") {
    return (
      <div className="h-full overflow-auto rounded-b-lg border border-(--color-border) bg-(--color-bg-elevated) px-2">
        <Button
          variant="outline"
          size="sm"
          onClick={() => setPage("info")}
          className="mx-2 my-4"
        >
          <ChevronLeft />
          Back{" "}
        </Button>
        <h2 className="px-2 pb-1 text-xl font-semibold">
          {t("content-warnings", { defaultValue: "Content Warnings" })}
        </h2>
        <p className="px-2 pb-4 text-sm">
          {t("content-warnings-info", {
            defaultValue:
              "You're required to flag your stream if it has themes that viewers may want a heads-up about.",
          })}
        </p>
        <ContentWarningsQuickEdit
          liveStore={liveStore}
          livestreamUri={livestream?.uri}
        />
      </div>
    );
  }

  return (
    <div className="h-full overflow-auto rounded-b-lg border border-(--color-border) bg-(--color-bg-elevated)">
      <div className="space-y-4 p-4">
        {/* Title */}
        <div className="space-y-0">
          <label className="mb-1 block text-sm text-(--color-fg-muted)">
            {t("title", { defaultValue: "Title" })}
          </label>
          <Textarea
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder={t("enter-stream-title", {
              defaultValue: "Enter your stream title…",
            })}
            maxLength={140}
            rows={2}
          />
          <div className="mt-1.5 flex justify-end">
            <span
              className={cn(
                "text-xs",
                title.length <= 120
                  ? "text-(--color-fg-muted)"
                  : title.length < 140
                    ? "text-amber-400"
                    : "text-red-400",
              )}
            >
              {title.length}/140
            </span>
          </div>
        </div>

        {/* Activity */}
        <ActivityPicker value={activity} onChange={setActivity} />

        {/* Tags */}
        <div className="space-y-1.5">
          <label className="mb-1 block text-sm text-(--color-fg-muted)">
            {t("tags", { defaultValue: "Tags" })}
          </label>
          <div className="flex flex-wrap items-center gap-1.5">
            {tags
              .filter((t) => !t.startsWith(LANG_TAG_PREFIX))
              .map((tag) => (
                <span
                  key={tag}
                  className="inline-flex items-center gap-1 rounded-full border border-(--color-accent)/20 bg-(--color-accent)/10 px-2 py-0.5 text-sm text-(--color-accent)"
                >
                  {tag}
                  <button
                    type="button"
                    onClick={() => handleRemoveTag(tag)}
                    className="hover:text-(--color-accent-fg)"
                  >
                    <X className="size-3" />
                  </button>
                </span>
              ))}
            {tags.length < 10 && (
              <Input
                value={tagInput}
                onChange={(e) =>
                  setTagInput(e.target.value.replace(/[^a-zA-Z0-9]/g, ""))
                }
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    handleAddTag(tagInput);
                  }
                }}
                placeholder={t("add-tag", { defaultValue: "Add tag…" })}
                className="min-w-20 flex-1 rounded-full px-2 py-0 text-sm"
              />
            )}
            {/* Language picker */}
            <DropdownMenu>
              <DropdownMenuTrigger className="flex items-center justify-center rounded-full border border-(--color-border) bg-(--color-bg) px-2 py-1 text-sm">
                {language
                  ? language
                  : t("language", { defaultValue: "Language" })}
                <ChevronDown className="ml-1 size-3" />
              </DropdownMenuTrigger>
              <DropdownMenuContent>
                {LANGUAGES.map((l) => (
                  <DropdownMenuItem
                    key={l.code}
                    onClick={() => {
                      const withoutLang = tags.filter(
                        (t) => !t.startsWith(LANG_TAG_PREFIX),
                      );
                      setTags(
                        `${LANG_TAG_PREFIX}${l.code}`
                          ? [...withoutLang, `${LANG_TAG_PREFIX}${l.code}`]
                          : withoutLang,
                      );
                    }}
                  >
                    {l.label}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
            {/*<select
              value={
                tags
                  .find((t) => t.startsWith(LANG_TAG_PREFIX))
                  ?.slice(LANG_TAG_PREFIX.length) ?? ""
              }
              onChange={(e) => {
                const code = e.target.value;
                const withoutLang = tags.filter(
                  (t) => !t.startsWith(LANG_TAG_PREFIX),
                );
                setTags(
                  code
                    ? [...withoutLang, `${LANG_TAG_PREFIX}${code}`]
                    : withoutLang,
                );
              }}
              className="px-2 py-1 text-sm rounded-full border border-(--color-border) bg-(--color-bg) text-(--color-fg-muted)"
            >
              <option value="">
                {t("language", { defaultValue: "Language" })}
              </option>
              {LANGUAGES.map((l) => (
                <option key={l.code} value={l.code}>
                  {l.label}
                </option>
              ))}
            </select>*/}
          </div>
        </div>

        {/* Thumbnail */}
        <div className="space-y-1.5">
          <label className="mb-1 block text-sm text-(--color-fg-muted)">
            {t("thumbnail", { defaultValue: "Thumbnail" })}
            <span className="ml-1 text-sm text-(--color-fg-muted)/60">
              ({t("optional", { defaultValue: "optional" })})
            </span>
          </label>
          {thumbnailPreview ? (
            <div className="relative">
              <img
                src={thumbnailPreview}
                alt=""
                className="h-32 w-full rounded-md object-cover"
              />
              <button
                type="button"
                onClick={handleThumbnailRemove}
                className="absolute top-2 right-2 rounded-full bg-black/70 p-1 text-white transition-colors hover:bg-black/90"
              >
                <X className="size-3.5" />
              </button>
            </div>
          ) : (
            <button
              type="button"
              onClick={handleThumbnailSelect}
              className="flex h-24 w-full flex-col items-center justify-center gap-1 rounded-md border-2 border-dashed border-(--color-border) text-(--color-fg-muted) transition-colors hover:border-(--color-border-strong)"
            >
              <ImagePlus className="size-6" />
              <span className="text-sm">
                {t("add-thumbnail", { defaultValue: "Add thumbnail" })}
              </span>
            </button>
          )}
        </div>

        {/* Options */}
        <div className="space-y-2 rounded-md border border-(--color-border) p-3">
          <label className="flex cursor-pointer items-center gap-2">
            <input
              type="checkbox"
              checked={createPost}
              onChange={(e) => setCreatePost(e.target.checked)}
              className="rounded border-(--color-border)"
            />
            <span className="text-sm text-(--color-fg)">
              {t("create-bluesky-post", {
                defaultValue: "Create Bluesky post",
              })}
            </span>
          </label>
          <label className="flex cursor-pointer items-center gap-2">
            <input
              type="checkbox"
              checked={idleTimeout}
              onChange={(e) => setIdleTimeout(e.target.checked)}
              className="rounded border-(--color-border)"
            />
            <span className="text-sm text-(--color-fg)">
              {t("end-automatically", {
                defaultValue: "End livestream automatically",
              })}
            </span>
          </label>
        </div>

        <Button
          variant="outline"
          size="default"
          onClick={() => setPage("contentWarnings")}
          className="w-full"
        >
          <div className="flex-1 text-left text-base">
            {t("add-content-warnings", {
              defaultValue: "Add content warnings",
            })}
          </div>
          <ChevronRight className="size-4" />
        </Button>

        <Admonition type="warning" size="sm">
          <Admonition.Title>
            {t("content-warnings-required", {
              defaultValue: "Content warnings are required.",
            })}
          </Admonition.Title>
          <Admonition.Description>
            {t("content-warnings-required-desc", {
              defaultValue:
                "Content warnings help inform viewers about potentially sensitive themes in your stream.",
            })}
          </Admonition.Description>
        </Admonition>

        {/* Actions */}
        <div className="space-y-2">
          <button
            type="button"
            onClick={handleSubmit}
            disabled={!canSubmit || !isLive}
            className={cn(
              "flex h-9 w-full items-center justify-center gap-2 rounded-md text-sm font-medium transition-colors",
              "bg-(--color-accent) text-(--color-accent-fg) hover:bg-(--color-accent-hover)",
              "disabled:cursor-not-allowed disabled:opacity-50",
            )}
          >
            {loading && <Loader2 className="size-4 animate-spin" />}
            <Radio className="size-4" />
            {buttonText}
          </button>
          <button
            type="button"
            onClick={handleEnd}
            disabled={!canEnd}
            className={cn(
              "flex h-9 w-full items-center justify-center gap-2 rounded-md text-sm font-medium transition-colors",
              canEnd
                ? "border border-red-500/20 bg-red-500/10 text-red-400 hover:bg-red-500/20"
                : "cursor-not-allowed border border-(--color-border) bg-(--color-bg) text-(--color-fg-muted) opacity-50",
            )}
          >
            {endingLivestream ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <Square className="size-4" />
            )}
            {endingLivestream
              ? t("ending", { defaultValue: "Ending…" })
              : t("end-livestream", { defaultValue: "End Livestream" })}
          </button>
        </div>
      </div>
    </div>
  );
}

// ─── Content warnings quick-edit (collapsed by default) ──────────────

function ContentWarningsQuickEdit({
  liveStore,
  livestreamUri,
}: {
  liveStore: LivestreamStore;
  livestreamUri?: string;
}) {
  const { t } = useTranslation("common");
  const getContentMetadata = useGlobalStore((s) => s.getContentMetadata);
  const createContentMetadata = useGlobalStore((s) => s.createContentMetadata);
  const updateContentMetadata = useGlobalStore((s) => s.updateContentMetadata);
  const lastRecord = useGlobalStore((s) => s.lastCreatedRecord) as any;

  const existingWarnings = useStore(
    liveStore,
    (s) =>
      (s.livestream?.record as any)?.contentWarnings?.warnings as
        | string[]
        | undefined,
  );

  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [initialized, setInitialized] = useState(false);
  const [saving, setSaving] = useState(false);

  // Initial load: pull existing metadata from the slice cache
  useEffect(() => {
    if (initialized) return;
    setInitialized(true);
    if (existingWarnings && existingWarnings.length > 0) {
      setSelected(new Set(existingWarnings));
    } else if (lastRecord?.record?.contentWarnings?.warnings) {
      setSelected(new Set(lastRecord.record.contentWarnings.warnings));
    }
    // Trigger a fetch to make sure the cache is populated
    void getContentMetadata({});
  }, [initialized, existingWarnings, lastRecord, getContentMetadata]);

  const toggle = useCallback((value: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(value)) next.delete(value);
      else next.add(value);
      return next;
    });
  }, []);

  const selectedCount = selected.size;

  const handleSave = useCallback(async () => {
    setSaving(true);
    try {
      const rkey = livestreamUri?.split("/").pop();
      const livestreamRef =
        rkey && livestreamUri
          ? {
              uri: livestreamUri,
              cid: (lastRecord?.cid as string) ?? "",
            }
          : undefined;

      if (livestreamRef) {
        await updateContentMetadata({
          rkey,
          livestreamRef,
          contentWarnings: Array.from(selected),
        });
      } else {
        await createContentMetadata({
          contentWarnings: Array.from(selected),
        });
      }
      toast.success(
        t("content-warnings-saved", {
          defaultValue: "Content warnings saved",
        }),
      );
    } catch (error) {
      console.error("Failed to save content warnings:", error);
      toast.error(
        t("content-warnings-save-failed", {
          defaultValue: "Failed to save content warnings",
        }),
      );
    } finally {
      setSaving(false);
    }
  }, [
    livestreamUri,
    lastRecord,
    selected,
    createContentMetadata,
    updateContentMetadata,
    t,
  ]);

  return (
    <div className="space-y-1.5 px-3 pt-1 pb-3">
      {CONTENT_WARNINGS.map((cw) => {
        const checked = selected.has(cw.value);
        return (
          <label
            key={cw.value}
            className="flex cursor-pointer items-start gap-2 text-sm"
          >
            <input
              type="checkbox"
              checked={checked}
              onChange={() => toggle(cw.value)}
              className="focus:ring-ring mt-0.5 size-3.5 rounded border-(--color-border) text-(--color-accent)"
            />
            <div className="space-y-0">
              <div className={"font-medium"}>{cw.label}</div>
              <span className="text-sm leading-px text-(--color-fg-muted)">
                {t(cw.label + "-description", { defaultValue: cw.description })}
              </span>
            </div>
          </label>
        );
      })}
      <Admonition type="info" size="sm">
        <Admonition.Title>
          {t("content-warnings-required", {
            defaultValue: "Check the community guidelines!",
          })}
        </Admonition.Title>
        <Admonition.Description>
          {t("content-warnings-required-desc", {
            defaultValue:
              "Your node may prohibit some of this content. Read the community guidelines to make sure.",
          })}
        </Admonition.Description>
        <Admonition.Link href="https://blog.stream.place/3mcqwibo4ks2w">
          {t("learn-more", {
            defaultValue: "Learn more",
          })}
        </Admonition.Link>
      </Admonition>
      <Button
        type="button"
        onClick={handleSave}
        disabled={saving}
        variant="secondary"
        className="mt-2 w-full"
      >
        {saving && <Loader2 className="size-3 animate-spin" />}
        {t("save", { defaultValue: "Save" })}
      </Button>
    </div>
  );
}
