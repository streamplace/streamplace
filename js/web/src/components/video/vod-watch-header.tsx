import { Button, buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import useAvatars from "@/hooks/use-avatars";
import { useToast } from "@/hooks/use-toast";
import type { VideoView } from "@/hooks/use-video-record";
import { useVodLike } from "@/hooks/use-vod-like";
import { cn } from "@/lib/utils";
import { Link } from "@tanstack/react-router";
import {
  Check,
  Code2,
  Download,
  Ellipsis,
  ExternalLink,
  Heart,
  Link2,
  Send,
  Share2,
} from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { place } from "streamplace";
import {
  buildVodShareLinks,
  getAuthorHandle,
  shouldCollapseDescription,
} from "./vod-watch";

type VodWatchHeaderProps = {
  video: VideoView;
  routeUser: string;
  tid: string;
  downloading: boolean;
  onDownload: () => void;
};

export function VodWatchHeader({
  video,
  routeUser,
  tid,
  downloading,
  onDownload,
}: VodWatchHeaderProps) {
  const { t, i18n } = useTranslation("common");
  const { show: showToast } = useToast();
  const [descriptionExpanded, setDescriptionExpanded] = useState(false);
  const [copied, setCopied] = useState<string | null>(null);
  const [failedAvatar, setFailedAvatar] = useState<string | null>(null);
  const record = video.record as place.stream.video.Main;
  const dids = useMemo(() => [video.author.did], [video.author.did]);
  const profiles = useAvatars(dids);
  const profile = profiles[video.author.did];
  const authorHandle = getAuthorHandle(video.author, routeUser);
  const authorRoute = video.author.handle || video.author.did;
  const avatar = profile?.avatar ?? video.author.avatar;
  const showAvatar = avatar && failedAvatar !== avatar;
  const description = record.description?.trim() ?? "";
  const collapsible = shouldCollapseDescription(description);
  const createdAt = record.createdAt
    ? new Intl.DateTimeFormat(i18n.language, { dateStyle: "long" }).format(
        new Date(record.createdAt),
      )
    : null;
  const viewCount = video.viewCounts?.count ?? 0;
  const title = record.title || t("untitled");
  const nativeNavigator = (
    typeof navigator === "undefined" ? {} : navigator
  ) as Navigator & {
    share?: (data?: ShareData) => Promise<void>;
  };
  const canNativeShare = typeof nativeNavigator.share === "function";
  const shareLinks = buildVodShareLinks(
    typeof window === "undefined"
      ? "https://stream.place"
      : window.location.origin,
    routeUser,
    tid,
  );
  const like = useVodLike({
    subjectUri: video.uri,
    initialCount: video.likeCount ?? 0,
  });

  const copy = async (value: string, key: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(key);
      showToast(t("share-copied"), undefined, { variant: "success" });
      window.setTimeout(() => setCopied(null), 1600);
    } catch {
      showToast(t("share-copy-failed"), undefined, { variant: "error" });
    }
  };

  const shareToBluesky = () => {
    const message = `${title} ${shareLinks.pageUrl}`;
    window.open(
      `https://bsky.app/intent/compose?text=${encodeURIComponent(message)}`,
      "_blank",
      "noopener,noreferrer",
    );
  };

  const nativeShare = async () => {
    if (!nativeNavigator.share) return;
    try {
      await nativeNavigator.share({
        title,
        text: t("share-video-message", { title }),
        url: shareLinks.pageUrl,
      });
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      showToast(t("share-failed"), undefined, { variant: "error" });
    }
  };

  const shareItems = (
    <>
      {canNativeShare && (
        <ShareAction icon={Share2} onClick={nativeShare}>
          {t("share-more-options")}
        </ShareAction>
      )}
      <ShareAction icon={Send} onClick={shareToBluesky}>
        {t("share-to-bluesky")}
      </ShareAction>
      <ShareAction
        icon={copied === "link" ? Check : Link2}
        onClick={() => copy(shareLinks.pageUrl, "link")}
      >
        {copied === "link" ? t("share-copied") : t("share-copy-link")}
      </ShareAction>
    </>
  );

  return (
    <section className="mx-auto mt-5 max-w-350 px-4 pb-8 sm:px-6">
      <div className="max-w-4xl">
        <h2 className="font-display text-xl leading-tight font-semibold text-(--color-fg) sm:text-2xl">
          {title}
        </h2>
        <p className="mt-1.5 text-sm text-(--color-fg-muted)">
          {t("views-count", { count: viewCount })}
          {createdAt && (
            <>
              <span aria-hidden="true"> · </span>
              {createdAt}
            </>
          )}
        </p>
      </div>

      <div className="mt-4 flex flex-wrap items-center gap-3">
        <Link
          to="/$user"
          params={{ user: authorRoute }}
          className="group flex min-w-56 flex-1 items-center gap-3 rounded-lg py-1 outline-none focus-visible:ring-3 focus-visible:ring-(--color-accent)/40"
          aria-label={t("view-profile", {
            handle: authorHandle,
          })}
        >
          <div className="flex size-11 shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--color-border) bg-(--color-bg-elevated) text-sm font-semibold text-(--color-fg-muted)">
            {showAvatar ? (
              <img
                src={avatar}
                alt=""
                className="size-full object-cover"
                onError={() => setFailedAvatar(avatar)}
              />
            ) : (
              (authorHandle[0]?.toUpperCase() ?? "?")
            )}
          </div>
          <span className="min-w-0 truncate text-sm font-semibold text-(--color-fg) group-hover:text-(--color-accent)">
            {authorHandle}
          </span>
        </Link>

        <div className="flex min-w-72 flex-1 items-center justify-end gap-2 sm:flex-none">
          <Button
            type="button"
            variant={like.liked ? "secondary" : "outline"}
            size="lg"
            className={cn(
              "h-11 min-w-16 flex-1 rounded-full px-3 sm:flex-none",
              like.liked &&
                "border-(--color-accent)/35 bg-(--color-accent)/15 text-(--color-accent) hover:bg-(--color-accent)/20",
            )}
            aria-pressed={like.liked}
            aria-label={t(like.liked ? "unlike-video" : "like-video", {
              count: like.count,
            })}
            disabled={like.loading}
            onClick={like.toggle}
          >
            <Heart className={cn(like.liked && "fill-current")} />
            <span className="tabular-nums">{like.count}</span>
          </Button>

          <div className="hidden sm:block">
            <DropdownMenu>
              <DropdownMenuTrigger
                className={buttonVariants({
                  variant: "outline",
                  size: "icon-touch",
                  className: "rounded-full",
                })}
                aria-label={t("share-video")}
              >
                <Share2 />
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="min-w-52">
                <DropdownMenuGroup>
                  <DropdownMenuLabel>{t("share-video")}</DropdownMenuLabel>
                  {canNativeShare && (
                    <DropdownMenuItem onClick={nativeShare}>
                      <Share2 /> {t("share-more-options")}
                    </DropdownMenuItem>
                  )}
                  <DropdownMenuItem onClick={shareToBluesky}>
                    <Send /> {t("share-to-bluesky")}
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={() => copy(shareLinks.pageUrl, "link")}
                  >
                    {copied === "link" ? <Check /> : <Link2 />}
                    {copied === "link"
                      ? t("share-copied")
                      : t("share-copy-link")}
                  </DropdownMenuItem>
                </DropdownMenuGroup>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>

          <div className="sm:hidden">
            <Sheet>
              <SheetTrigger
                render={
                  <Button
                    type="button"
                    variant="outline"
                    size="icon-touch"
                    className="rounded-full"
                    aria-label={t("share-video")}
                  >
                    <Share2 />
                  </Button>
                }
              />
              <SheetContent
                side="bottom"
                className="rounded-t-2xl pb-[max(1rem,env(safe-area-inset-bottom))]"
              >
                <SheetHeader>
                  <SheetTitle>{t("share-video")}</SheetTitle>
                  <SheetDescription className="line-clamp-1">
                    {title}
                  </SheetDescription>
                </SheetHeader>
                <div className="grid gap-2 px-4 pb-2">{shareItems}</div>
              </SheetContent>
            </Sheet>
          </div>

          <DropdownMenu>
            <DropdownMenuTrigger
              className={buttonVariants({
                variant: "ghost",
                size: "icon-touch",
                className: "rounded-full",
              })}
              aria-label={t("more-actions")}
            >
              <Ellipsis />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="min-w-52">
              <DropdownMenuGroup>
                <DropdownMenuLabel>{t("video-tools")}</DropdownMenuLabel>
                <DropdownMenuItem disabled={downloading} onClick={onDownload}>
                  <Download /> {t("download-video")}
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuLabel>{t("video-embed")}</DropdownMenuLabel>
                <DropdownMenuItem
                  onClick={() => copy(shareLinks.embedUrl, "embed-url")}
                >
                  {copied === "embed-url" ? <Check /> : <ExternalLink />}
                  {copied === "embed-url"
                    ? t("share-copied")
                    : t("share-copy-embed-url")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => copy(shareLinks.embedCode, "embed-code")}
                >
                  {copied === "embed-code" ? <Check /> : <Code2 />}
                  {copied === "embed-code"
                    ? t("share-copied")
                    : t("share-copy-embed-code")}
                </DropdownMenuItem>
              </DropdownMenuGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {description && (
        <div className="mt-5 max-w-3xl border-t border-(--color-border) pt-4">
          <p
            className={cn(
              "text-[0.9375rem] leading-relaxed whitespace-pre-wrap text-(--color-fg)",
              collapsible && !descriptionExpanded && "line-clamp-3",
            )}
          >
            {description}
          </p>
          {collapsible && (
            <Button
              type="button"
              variant="link"
              className="mt-1 h-auto p-0 text-sm text-(--color-fg-muted)"
              onClick={() => setDescriptionExpanded((expanded) => !expanded)}
            >
              {t(descriptionExpanded ? "less-details" : "more-details")}
            </Button>
          )}
        </div>
      )}
    </section>
  );
}

function ShareAction({
  icon: Icon,
  onClick,
  children,
}: {
  icon: typeof Share2;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <SheetClose
      render={
        <Button
          type="button"
          variant="ghost"
          className="h-11 w-full justify-start gap-3 px-3"
          onClick={onClick}
        >
          <Icon className="size-5" />
          {children}
        </Button>
      }
    />
  );
}
