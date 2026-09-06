import { useToast } from "@/hooks/use-toast";
import { useStore } from "@/lib/store";
import { useStreamplaceUrl } from "@/lib/store/hooks";
import { cn } from "@/lib/utils";
import type { LivestreamStore } from "@streamplace/core";
import {
  Check,
  ChevronRight,
  ClipboardCopy,
  Pencil,
  Plus,
  Share2,
} from "lucide-react";
import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import { place } from "streamplace";
import { useStore as useLivestreamStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import { useCanModerate } from "../../hooks/use-can-moderate";
import type { Liveness } from "../../hooks/use-liveness-state";
import { useModerationActions } from "../../hooks/use-moderation-actions";
import { useSession } from "../../lib/session";
import { Button, buttonVariants } from "../ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";
import { Textarea } from "../ui/textarea";
import { StreamAvatar } from "./stream-avatar";

const ACTIVITY_I18N_KEYS: Record<string, string> = {
  events: "activity-events",
  just_chatting: "activity-just-chatting",
  music: "activity-music",
  art: "activity-art",
  software_dev: "activity-software-dev",
  cooking: "activity-cooking",
  miniatures: "activity-miniatures",
  makers_crafting: "activity-makers-crafting",
  fitness: "activity-fitness",
  sports: "activity-sports",
};

export function activityLabel(
  activity:
    | place.stream.defs.ActivityGame
    | place.stream.defs.ActivityLabel
    | { $type: string }
    | undefined,
  t: (key: string) => string,
): string | null {
  if (!activity) return null;
  if ("name" in activity && activity.name) return activity.name;
  if ("label" in activity)
    return t(ACTIVITY_I18N_KEYS[activity.label] ?? activity.label);
  return null;
}

export function StreamInfo({
  store,
  user,
  liveness,
  chatOpen,
  onToggleChat,
  avatar,
}: {
  store: LivestreamStore;
  user: string;
  liveness: Liveness;
  chatOpen: boolean;
  onToggleChat: () => void;
  avatar?: string;
}) {
  const { t } = useTranslation("common");
  const state = useLivestreamStore(
    store,
    useShallow((s) => ({
      livestream: s.livestream,
      viewers: s.viewers,
    })),
  );

  const { state: sessionState } = useSession();
  const followUser = useStore((s) => s.followUser);
  const [following, setFollowing] = useState(false);
  const record = state.livestream?.record;
  const author = state.livestream?.author;
  const title = record?.title || user;
  const activity = activityLabel(record?.activity, t);
  const tags = record?.tags;
  const isLive = liveness === "live";
  const moderation = useCanModerate(store);
  const [showEditTitle, setShowEditTitle] = useState(false);

  const node = useStreamplaceUrl();

  const handleFollow = async () => {
    if (!author?.did) return;
    setFollowing(true);
    try {
      await followUser(author.did);
    } catch (e) {
      console.error("Failed to follow:", e);
    } finally {
      setFollowing(false);
    }
  };

  return (
    <div className="mx-4 mt-4 mb-5 border-b border-(--color-border) pb-5">
      <div className="flex items-start gap-3">
        <StreamAvatar
          avatar={avatar ?? author?.avatar}
          label={author?.displayName || author?.handle || user}
          className="h-11 w-11"
        />

        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
            <span className="truncate font-medium">
              {author?.displayName || author?.handle || user}
            </span>
            {author?.handle && author.displayName && (
              <span className="truncate text-sm text-(--color-fg-muted)">
                @{author.handle}
              </span>
            )}
            {isLive && state.viewers != null && (
              <span className="flex items-center gap-1 text-xs text-(--color-accent)">
                <span className="h-1.5 w-1.5 rounded-full bg-(--color-accent)" />
                {t("watching-count", { count: state.viewers })}
              </span>
            )}
          </div>

          <div className="mt-1 flex items-start gap-1">
            <h2 className="font-display line-clamp-2 text-lg leading-tight font-semibold text-(--color-fg)">
              {title}
            </h2>
            {moderation.canManageLivestream && record && (
              <button
                type="button"
                onClick={() => setShowEditTitle(true)}
                className="mt-0.5 shrink-0 rounded p-1 text-(--color-fg-muted) transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
                aria-label={t("edit-title")}
                title={t("edit-title")}
              >
                <Pencil className="size-3.5" />
              </button>
            )}
          </div>

          <div className="mt-2 flex flex-wrap items-center gap-1.5">
            {activity && (
              <span className="rounded-full border border-(--color-border) bg-(--color-bg-elevated) px-2 py-0.5 text-xs font-medium text-(--color-fg-muted)">
                {activity}
              </span>
            )}
            {tags?.map((tag) => (
              <span
                key={tag}
                className="rounded-full border border-(--color-border) bg-(--color-bg-elevated) px-2 py-0.5 text-xs text-(--color-fg-subtle)"
              >
                {tag.startsWith("lang:") ? tag.slice(5).toUpperCase() : tag}
              </span>
            ))}
          </div>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          {sessionState.status === "authenticated" &&
            sessionState.session.did !== author?.did && (
              <Button
                type="button"
                size="sm"
                onClick={handleFollow}
                disabled={following}
              >
                <Plus className="size-4" /> {t("follow")}
              </Button>
            )}
          <CopyButton type="live" nodeBaseURL={node} />
          <Button
            type="button"
            variant="outline"
            size="icon-lg"
            onClick={onToggleChat}
            aria-label={chatOpen ? t("chat-close") : t("chat-open")}
            title={chatOpen ? t("chat-close") : t("chat-open")}
          >
            <ChevronRight
              className={cn(
                "size-6 transition-transform duration-250 ease-in-out",
                !chatOpen && "-scale-100",
              )}
            />
          </Button>
        </div>
      </div>
      {showEditTitle && (
        <EditTitleDialog
          store={store}
          onClose={() => setShowEditTitle(false)}
        />
      )}
    </div>
  );
}

function assembleShareLink(
  nodeBaseUrl: string,
  type: "vod" | "live",
  embed?: boolean,
  rkey?: string,
): string {
  const url = new URL(nodeBaseUrl);
  if (type === "vod" && rkey) {
    url.pathname = `/${type}/vod/${rkey}`;
  } else {
    url.pathname = `/${type}`;
  }
  url.pathname += embed ? "/embed" : "";
  return url.toString();
}

// copy button that has a dropdown for multiple platforms
export function CopyButton({
  nodeBaseURL,
  type,
  rkey,
  className,
  variant = "outline",
  size = "icon-lg",
}: {
  nodeBaseURL?: string;
  type: "vod" | "live";
  rkey?: string;
  className?: string;
  variant?: "outline" | "ghost";
  size?: "icon-lg" | "icon-touch";
}) {
  const { t } = useTranslation("common");
  const [copied, setCopied] = useState(false);
  const share = nodeBaseURL
    ? assembleShareLink(nodeBaseURL, type, false, rkey)
    : undefined;

  const embedShare = nodeBaseURL
    ? assembleShareLink(nodeBaseURL, type, true, rkey)
    : undefined;

  const handleCopy = (link: string) => {
    navigator.clipboard
      .writeText(link)
      .then(() => {
        setCopied(true);
        window.setTimeout(() => setCopied(false), 1600);
      })
      .catch(() => setCopied(false));
  };
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          className={cn(
            buttonVariants({ variant, size }),
            "relative overflow-hidden",
            className,
          )}
          aria-label={copied ? t("share-copied") : t("share-copy")}
          title={copied ? t("share-copied") : t("share-copy")}
        >
          {copied ? (
            <Check className="size-4 text-(--color-accent)" />
          ) : (
            <Share2 className="size-4" />
          )}
        </DropdownMenuTrigger>
        <DropdownMenuContent className="min-w-40">
          <DropdownMenuGroup>
            <DropdownMenuLabel>{t("share-copy")}</DropdownMenuLabel>
            <DropdownMenuItem
              onClick={() => {
                if (share) {
                  handleCopy(share);
                }
              }}
            >
              <ClipboardCopy /> {t("share-copy-link")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => {
                if (embedShare) {
                  handleCopy(
                    `<iframe src="${embedShare}" width="560" height="315" frameborder="0" allowfullscreen></iframe>`,
                  );
                }
              }}
            >
              <ClipboardCopy /> {t("share-copy-embed-code")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => {
                if (embedShare) {
                  handleCopy(embedShare);
                }
              }}
            >
              <ClipboardCopy /> {t("share-copy-embed-url")}
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  );
}

function EditTitleDialog({
  store,
  onClose,
}: {
  store: LivestreamStore;
  onClose: () => void;
}) {
  const { t } = useTranslation("common");
  const toast = useToast();
  const { updateStreamTitle } = useModerationActions();
  const livestream = useLivestreamStore(store, (s) => s.livestream);
  const [title, setTitle] = useState(livestream?.record.title ?? "");
  const [isSaving, setIsSaving] = useState(false);

  const handleSave = useCallback(async () => {
    if (!livestream || isSaving) return;
    const next = title.trim();
    if (!next) return;
    setIsSaving(true);
    try {
      await updateStreamTitle(livestream.uri, next, livestream.author.did);
      toast.show(t("edit-title-saved"), "", { duration: 3000 });
      onClose();
    } catch (e) {
      console.error(e);
      toast.show(
        t("edit-title-failed"),
        e instanceof Error ? e.message : String(e),
        { duration: 5000 },
      );
    } finally {
      setIsSaving(false);
    }
  }, [isSaving, livestream, onClose, t, title, toast, updateStreamTitle]);

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("edit-title-dialog-title")}</DialogTitle>
        </DialogHeader>
        <Textarea
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          maxLength={140}
          rows={2}
          aria-label={t("edit-title-label")}
        />
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isSaving}>
            {t("cancel")}
          </Button>
          <Button onClick={handleSave} disabled={isSaving || !title.trim()}>
            {isSaving ? t("edit-title-saving") : t("save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
