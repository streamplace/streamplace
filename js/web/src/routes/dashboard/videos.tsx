import { UploadForm } from "@/components/dashboard/upload-form";
import { useUpload } from "@/hooks/use-upload";
import { useSession } from "@/lib/session";
import { useIsReady, usePDSAgent, useUserProfile } from "@/lib/store/hooks";
import { getTidFromAtUri } from "@/lib/video";
import { createFileRoute } from "@tanstack/react-router";
import {
  Clipboard,
  ExternalLink,
  LoaderCircle,
  Pencil,
  Video,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";

export const Route = createFileRoute("/dashboard/videos")({
  component: DashboardVideosPage,
});

function DashboardVideosPage() {
  const agent = usePDSAgent();
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const { state: session, did } = useSession();
  const upload = useUpload();

  const [userVideos, setUserVideos] = useState<any[]>([]);
  const [videosLoading, setVideosLoading] = useState(false);
  const [editingVideoUri, setEditingVideoUri] = useState<string | undefined>(
    undefined,
  );
  const [updating, setUpdating] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [existingThumb, setExistingThumb] = useState<any>(null);

  const fetchVideos = useCallback(async () => {
    if (!agent?.did) return;
    setVideosLoading(true);
    try {
      const res = await agent.place.stream.media.getVideoList({
        repo: agent.did,
      });
      setUserVideos(res.data.videos || []);
    } catch (err) {
      console.error("Failed to fetch videos", err);
    } finally {
      setVideosLoading(false);
    }
  }, [agent]);

  useEffect(() => {
    fetchVideos();
  }, [fetchVideos]);

  const handleSelectVideo = useCallback(
    (video: any) => {
      const rec = video.record?.value || video.record || {};
      setEditingVideoUri(video.uri);
      upload.setTitle(rec.title || "");
      upload.setDescription(rec.description || "");
      upload.setTagsDirectly(rec.tags || []);
      setExistingThumb(rec.thumb || null);
      const cw = rec.contentWarnings?.warnings || [];
      upload.setWarningsDirectly(new Set(cw));
      const rights = rec.contentRights || {};
      upload.setLicense(
        rights.license?.$type ||
          "place.stream.metadata.contentRights#all-rights-reserved",
      );
    },
    [upload],
  );

  const handleUpdate = useCallback(
    async (u: ReturnType<typeof useUpload>) => {
      if (!agent?.did || !editingVideoUri) return;
      setUpdating(true);
      try {
        const rkey = editingVideoUri.split("/").pop()!;
        const existing = await agent.com.atproto.repo.getRecord({
          repo: agent.did,
          collection: "place.stream.video",
          rkey,
        });
        const existingRec = existing.data.value as any;

        const record: Record<string, any> = {
          $type: "place.stream.video",
          title: u.title.trim(),
          source: existingRec.source,
          durationMs: existingRec.durationMs,
          createdAt: existingRec.createdAt || new Date().toISOString(),
        };
        if (u.description.trim()) record.description = u.description.trim();
        if (u.tags.length > 0) record.tags = u.tags;
        if (u.warnings.size > 0) {
          const cw: Record<string, boolean> = {};
          for (const w of u.warnings) {
            const key = w.split("#")[1];
            if (key) cw[key] = true;
          }
          record.contentWarnings = {
            $type: "place.stream.metadata.contentWarnings",
            ...cw,
          };
        }
        if (
          u.license &&
          u.license !==
            "place.stream.metadata.contentRights#all-rights-reserved"
        ) {
          const licenseKey = u.license.split("#")[1];
          record.contentRights = {
            $type: "place.stream.metadata.contentRights",
            license: { $type: u.license },
            [licenseKey]: { $type: u.license },
          };
        }
        if (u.thumbnail) {
          try {
            const blobRes = await agent.uploadBlob(u.thumbnail, {
              encoding: u.thumbnail.type || "image/jpeg",
            });
            if (blobRes.success) record.thumb = blobRes.data.blob;
          } catch {
            // non-fatal
          }
        } else if (existingThumb) {
          // Preserve the existing thumbnail when no new one was selected
          record.thumb = existingThumb;
        }
        await agent.com.atproto.repo.putRecord({
          repo: agent.did,
          collection: "place.stream.video",
          rkey,
          record: record as any,
        });
        setEditingVideoUri(undefined);
        setExistingThumb(null);
        u.reset();
        fetchVideos();
      } catch (err) {
        console.error("Failed to update video", err);
      } finally {
        setUpdating(false);
      }
    },
    [agent, editingVideoUri, fetchVideos, existingThumb],
  );

  const handleDelete = useCallback(async () => {
    if (!agent?.did || !editingVideoUri) return;
    setDeleting(true);
    try {
      await agent.com.atproto.repo.deleteRecord({
        repo: agent.did,
        collection: "place.stream.video",
        rkey: editingVideoUri.split("/").pop()!,
      });
      setEditingVideoUri(undefined);
      setExistingThumb(null);
      upload.reset();
      fetchVideos();
    } catch (err) {
      console.error("Failed to delete video", err);
    } finally {
      setDeleting(false);
    }
  }, [agent, editingVideoUri, upload, fetchVideos]);

  const handleCancelEdit = useCallback(() => {
    setEditingVideoUri(undefined);
    setExistingThumb(null);
    upload.reset();
  }, [upload]);

  if (!isReady) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <LoaderCircle className="animate-spin text-(--color-fg-muted)" />
      </div>
    );
  }
  if (!userProfile) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center px-6">
        <p className="text-sm text-(--color-fg-muted)">
          Please log in to manage your videos.
        </p>
      </div>
    );
  }

  // Edit mode: show the upload form pre-filled with the video's metadata
  if (editingVideoUri) {
    return (
      <div className="mx-auto w-full max-w-240 px-4 py-4">
        <div className="mb-3 flex items-center justify-between">
          <span className="text-sm text-(--color-fg-muted)">Editing video</span>
        </div>
        <UploadForm
          upload={upload}
          editingVideoUri={editingVideoUri}
          onCancelEdit={handleCancelEdit}
          onUpdate={handleUpdate}
          onDelete={handleDelete}
          isUpdating={updating}
          isDeleting={deleting}
        />
      </div>
    );
  }

  // List mode
  return (
    <div className="mx-auto w-full max-w-240 px-4 py-4">
      <div className="space-y-3">
        {videosLoading && (
          <div className="flex justify-center py-8">
            <LoaderCircle className="animate-spin text-(--color-fg-muted)" />
          </div>
        )}
        {!videosLoading && userVideos.length === 0 && (
          <div className="flex flex-col items-center py-12 text-(--color-fg-muted)">
            <Video className="mb-3 size-12" />
            <p className="text-sm">No videos yet.</p>
          </div>
        )}
        {userVideos.map((video: any) => {
          const rec = video.record?.value || video.record || {};
          const thumb = rec.thumb;
          const thumbUrl = thumb
            ? `https://cdn.stream.place/thumb/${thumb.ref?.$link || thumb.cid || ""}`
            : undefined;
          const tid = getTidFromAtUri(video.uri);
          const videoUser = did || "";
          const videoPath = `/${videoUser}/video/${tid}`;
          const videoUrl = `${window.location.origin}${videoPath}`;

          return (
            <div
              key={video.uri}
              className="flex items-center gap-3 rounded-lg border border-(--color-border) bg-(--color-bg) p-3 transition-colors hover:bg-(--color-muted)"
            >
              {thumbUrl ? (
                <img
                  src={thumbUrl}
                  alt=""
                  className="h-11.25 w-20 rounded object-cover"
                />
              ) : (
                <div className="flex h-11.25 w-20 items-center justify-center rounded bg-(--color-muted)">
                  <Video className="size-5 text-(--color-fg-muted)" />
                </div>
              )}
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-semibold">
                  {rec.title || "Untitled"}
                </p>
                <p className="text-xs text-(--color-fg-muted)">
                  {video.viewCounts?.count != null &&
                    `${video.viewCounts.count} views · `}
                  {rec.durationMs
                    ? `${Math.round(rec.durationMs / 1000)}s`
                    : ""}
                </p>
              </div>
              <div className="flex items-center gap-1">
                <button
                  type="button"
                  onClick={() => handleSelectVideo(video)}
                  className="rounded p-1.5 text-(--color-fg-muted) transition-colors hover:bg-(--color-accent-subtle) hover:text-(--color-accent)"
                  title="Edit"
                >
                  <Pencil className="size-4" />
                </button>
                <button
                  type="button"
                  onClick={() => navigator.clipboard.writeText(videoUrl)}
                  className="rounded p-1.5 text-(--color-fg-muted) transition-colors hover:bg-(--color-accent-subtle) hover:text-(--color-accent)"
                  title="Copy link"
                >
                  <Clipboard className="size-4" />
                </button>
                <a
                  href={videoPath}
                  className="rounded p-1.5 text-(--color-fg-muted) transition-colors hover:bg-(--color-accent-subtle) hover:text-(--color-accent)"
                  title="Go to video"
                >
                  <ExternalLink className="size-4" />
                </a>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
