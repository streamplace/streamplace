import { useStreamplaceUrl } from "@/lib/store/hooks";
import { cn } from "@/lib/utils";
import type { LivestreamStore } from "@streamplace/core";
import { ChevronRight, ClipboardCopy, Plus, Share2 } from "lucide-react";
import type { PlaceStreamDefs } from "streamplace";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import type { Liveness } from "../../hooks/use-liveness-state";
import { formatViewers } from "../../lib/format";
import { useSession } from "../../lib/session";
import { Button, buttonVariants } from "../ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";

const ACTIVITY_LABELS: Record<string, string> = {
  events: "Events",
  just_chatting: "Just Chatting",
  music: "Music",
  art: "Art",
  software_dev: "Software Dev",
  cooking: "Cooking",
  miniatures: "Miniatures",
  makers_crafting: "Makers & Crafting",
  fitness: "Fitness",
  sports: "Sports",
};

export function activityLabel(
  activity:
    | PlaceStreamDefs.ActivityGame
    | PlaceStreamDefs.ActivityLabel
    | { $type: string }
    | undefined,
): string | null {
  if (!activity) return null;
  if ("name" in activity && activity.name) return activity.name;
  if ("label" in activity)
    return ACTIVITY_LABELS[activity.label] ?? activity.label;
  return null;
}

export function StreamInfo({
  store,
  user,
  liveness,
  chatOpen,
  onToggleChat,
}: {
  store: LivestreamStore;
  user: string;
  liveness: Liveness;
  chatOpen: boolean;
  onToggleChat: () => void;
}) {
  const state = useStore(
    store,
    useShallow((s) => ({
      livestream: s.livestream,
      viewers: s.viewers,
    })),
  );

  const { state: sessionState } = useSession();
  const record = state.livestream?.record;
  const author = state.livestream?.author;
  const title = record?.title || user;
  const activity = activityLabel(record?.activity);
  const tags = record?.tags;
  const viewers = formatViewers(state.viewers);
  const isLive = liveness === "live";

  const node = useStreamplaceUrl();

  return (
    <div className="mt-3 space-y-3 mx-3">
      <div className="flex items-start gap-3">
        <img
          src={author?.avatar ?? undefined}
          alt=""
          className="w-10 h-10 rounded-full bg-[var(--color-bg-elevated)] flex-shrink-0"
          onError={(e) => {
            (e.currentTarget as HTMLImageElement).style.display = "none";
          }}
        />

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="truncate">
              {author?.displayName || author?.handle || user}
            </span>
            {isLive && viewers && (
              <span className="text-xs text-[var(--color-fg-muted)] flex-shrink-0">
                {viewers} watching
              </span>
            )}
          </div>

          <h2 className="font-display font-semibold text-[var(--color-fg)] mt-0.5 line-clamp-2">
            {title}
          </h2>

          <div className="flex items-center gap-1.5 mt-1.5 flex-wrap">
            {activity && (
              <span className="text-xs px-2 py-0.5 rounded-full bg-[var(--color-bg-elevated)] border border-[var(--color-border)] text-[var(--color-fg-muted)]">
                {activity}
              </span>
            )}
            {tags?.map((tag) => (
              <span
                key={tag}
                className="text-xs px-2 py-0.5 rounded-full bg-[var(--color-bg-elevated)] border border-[var(--color-border)] text-[var(--color-fg-subtle)]"
              >
                {tag.startsWith("lang:") ? tag.slice(5).toUpperCase() : tag}
              </span>
            ))}
          </div>
        </div>

        <div className="flex items-center gap-2 flex-shrink-0">
          {sessionState.status === "authenticated" &&
            sessionState.session.did !== author?.did && (
              <Button type="button">
                <Plus className="size-4" /> Follow
              </Button>
            )}
          <CopyButton type="live" nodeBaseURL={node} />
          <Button
            type="button"
            variant="outline"
            size="icon-lg"
            onClick={onToggleChat}
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

// checkmark button save 4 later
//           <Button
//   type="button"
//   variant={copied ? "success" : "outline"}
//   size="icon-lg"
//   disabled={typeof share === "undefined"}
//   className={cn("relative overflow-hidden")}
// >
//   {copied ? "Copied!" : "Share"}
//   <svg
//     className={cn(
//       "size-4 text-green-500 absolute",
//       "transition-all duration-300",
//       copied ? "opacity-100" : "opacity-0 translate-y-6",
//     )}
//     fill="none"
//     viewBox="0 0 24 24"
//     stroke="currentColor"
//     strokeWidth={2}
//   >
//     <path
//       strokeLinecap="round"
//       strokeLinejoin="round"
//       d="M5 13l4 4L19 7"
//     />
//   </svg>
//   <Share2
//     className={cn(
//       "size-4 transition-transform duration-300",
//       copied && "scale-0",
//     )}
//   />
// </Button>

// copy button that has a dropdown for multiple platforms
function CopyButton({
  nodeBaseURL,
  type,
  rkey,
}: {
  nodeBaseURL?: string;
  type: "vod" | "live";
  rkey?: string;
}) {
  const share = nodeBaseURL
    ? assembleShareLink(nodeBaseURL, type, false, rkey)
    : undefined;

  const embedShare = nodeBaseURL
    ? assembleShareLink(nodeBaseURL, type, true, rkey)
    : undefined;

  const handleCopy = (link: string) => {
    console.error("copying link", link);
    navigator.clipboard.writeText(link);
  };
  const handleOpenInNewTab = (link: string) => {
    window.open(link, "_blank");
  };

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          className={cn(
            buttonVariants({ variant: "outline", size: "icon-lg" }),
            "relative overflow-hidden",
          )}
        >
          <Share2 className={cn("size-4 transition-transform duration-300")} />
        </DropdownMenuTrigger>
        <DropdownMenuContent className="min-w-40">
          <DropdownMenuGroup>
            <DropdownMenuLabel>Copy</DropdownMenuLabel>
            <DropdownMenuItem
              onClick={() => {
                if (share) {
                  handleCopy(share);
                }
              }}
            >
              <ClipboardCopy /> Copy Link
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
              <ClipboardCopy /> Copy Embed Code
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => {
                if (embedShare) {
                  handleCopy(embedShare);
                }
              }}
            >
              <ClipboardCopy /> Copy Embed URL
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  );
}
