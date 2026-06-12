// Reusable upload form: file picker, metadata, thumbnail, progress, actions.
// Used by both /dashboard/upload (new upload) and /dashboard/videos (edit).
import { Button } from "@/components/ui/button";
import { humanBytes, type useUpload } from "@/hooks/use-upload";
import { LICENSE_OPTIONS } from "@/lib/content-licenses";
import { CONTENT_WARNINGS } from "@/lib/content-warnings";
import { Link } from "@tanstack/react-router";
import {
  AlertCircle,
  ArrowUp,
  CheckCircle2,
  ImagePlus,
  LoaderCircle,
  Video,
  X,
} from "lucide-react";
import { useCallback, useRef } from "react";

type UploadHook = ReturnType<typeof useUpload>;

interface UploadFormProps {
  /** The upload hook instance. The parent owns this so it can pre-fill state for edits. */
  upload: UploadHook;
  /** When set, the form is in "edit" mode: hides file picker, shows update/delete actions. */
  editingVideoUri?: string;
  /** Called when the user wants to cancel editing. */
  onCancelEdit?: () => void;
  /** Called when the user clicks "Update Video". Receives the hook instance so the parent can drive the API call. */
  onUpdate?: (upload: UploadHook) => Promise<void>;
  /** Called when the user clicks "Delete Video". */
  onDelete?: () => Promise<void>;
  /** Whether an update or delete is in progress (disables buttons). */
  isUpdating?: boolean;
  isDeleting?: boolean;
}

export function UploadForm({
  upload,
  editingVideoUri,
  onCancelEdit,
  onUpdate,
  onDelete,
  isUpdating,
  isDeleting,
}: UploadFormProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const thumbnailInputRef = useRef<HTMLInputElement | null>(null);

  const handleFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const f = e.target.files?.[0] ?? null;
      upload.selectFile(f);
      e.target.value = "";
    },
    [upload],
  );

  const handleThumbnailChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const f = e.target.files?.[0] ?? null;
      upload.selectThumbnail(f);
      e.target.value = "";
    },
    [upload],
  );

  const isEdit = !!editingVideoUri;

  return (
    <div className="flex w-full flex-col items-start gap-6 md:flex-row">
      {/* ── left column: metadata ────────────────────────────────────── */}
      <div className="min-w-0 flex-1 space-y-4">
        {/* title + description */}
        <div className="space-y-1">
          <h3 className="text-sm font-medium text-(--color-fg-muted)">
            Details
          </h3>
          <div className="rounded-lg border border-(--color-border) bg-(--color-bg)">
            <div className="space-y-1 p-3">
              <label className="text-xs text-(--color-fg-muted)">Title</label>
              <input
                type="text"
                value={upload.title}
                onChange={(e) => upload.setTitle(e.target.value)}
                maxLength={140}
                placeholder="Give your video a title"
                className="w-full rounded-md border border-(--color-border) bg-(--color-bg) px-3 py-2 text-sm text-(--color-fg) placeholder:text-(--color-fg-muted)"
              />
            </div>
            <div className="border-t border-(--color-border)" />
            <div className="space-y-1 p-3">
              <label className="text-xs text-(--color-fg-muted)">
                Description
              </label>
              <textarea
                value={upload.description}
                onChange={(e) => upload.setDescription(e.target.value)}
                maxLength={5000}
                rows={4}
                placeholder="Describe your video..."
                className="w-full resize-none rounded-md border border-(--color-border) bg-(--color-bg) px-3 py-2 text-sm text-(--color-fg) placeholder:text-(--color-fg-muted)"
              />
            </div>
          </div>
        </div>

        {/* tags */}
        <div className="space-y-1">
          <h3 className="text-sm font-medium text-(--color-fg-muted)">Tags</h3>
          <div className="rounded-lg border border-(--color-border) bg-(--color-bg) p-3">
            <div className="flex flex-wrap gap-2">
              {upload.tags.map((tag) => (
                <span
                  key={tag}
                  className="inline-flex items-center gap-1 rounded-full bg-(--color-accent-subtle) px-2.5 py-0.5 text-xs font-medium text-(--color-accent)"
                >
                  {tag}
                  <button
                    type="button"
                    onClick={() => upload.removeTag(tag)}
                    className="ml-0.5 hover:text-(--color-fg)"
                  >
                    &times;
                  </button>
                </span>
              ))}
              {upload.tags.length < 10 && (
                <input
                  type="text"
                  value={upload.tagInput}
                  onChange={(e) =>
                    upload.setTagInput(
                      e.target.value.replace(/[^a-zA-Z0-9:]/g, ""),
                    )
                  }
                  placeholder="Add tag, press Enter"
                  className="w-40 bg-transparent text-sm text-(--color-fg) placeholder:text-(--color-fg-muted) focus:outline-none"
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      upload.addTag(upload.tagInput);
                    }
                  }}
                />
              )}
            </div>
          </div>
        </div>

        {/* content warnings */}
        <div className="space-y-1">
          <h3 className="text-sm font-medium text-(--color-fg-muted)">
            Content Warnings
          </h3>
          <div className="rounded-lg border border-(--color-border) bg-(--color-bg) p-3">
            <div className="space-y-2">
              {CONTENT_WARNINGS.map((cw) => (
                <label
                  key={cw.value}
                  className="flex cursor-pointer items-center gap-2 text-sm"
                  title={cw.description}
                >
                  <input
                    type="checkbox"
                    checked={upload.warnings.has(cw.value)}
                    onChange={() => upload.toggleWarning(cw.value)}
                    className="rounded border-(--color-border)"
                  />
                  {cw.label}
                </label>
              ))}
            </div>
          </div>
        </div>

        {/* license */}
        <div className="space-y-1">
          <h3 className="text-sm font-medium text-(--color-fg-muted)">
            License
          </h3>
          <div className="rounded-lg border border-(--color-border) bg-(--color-bg) p-3">
            <select
              value={upload.license}
              onChange={(e) => upload.setLicense(e.target.value)}
              className="w-full rounded-md border border-(--color-border) bg-(--color-bg) px-3 py-2 text-sm text-(--color-fg)"
            >
              {LICENSE_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </div>
        </div>
      </div>

      {/* ── right column: file + status + actions ────────────────────── */}
      <div className="w-full space-y-4 md:w-2/5">
        <div className="space-y-4">
          {/* file picker (upload mode only) */}
          {!isEdit && (
            <div className="space-y-1">
              <h3 className="text-sm font-medium text-(--color-fg-muted)">
                File
              </h3>
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                disabled={upload.isUploading}
                className="flex h-32 w-full flex-col items-center justify-center gap-1 rounded-lg border-2 border-dashed border-(--color-border) bg-(--color-bg) text-(--color-fg-muted) transition-colors hover:border-(--color-accent) hover:text-(--color-fg) disabled:opacity-50"
              >
                <Video className="size-8" />
                <span className="text-sm font-medium">
                  {upload.file ? upload.file.name : "Choose a video file"}
                </span>
                {upload.file && (
                  <span className="text-xs text-(--color-fg-muted)">
                    {upload.file.type || "unknown"} —{" "}
                    {humanBytes(upload.file.size)}
                  </span>
                )}
              </button>
              <input
                type="file"
                accept="video/*"
                ref={fileInputRef}
                onChange={handleFileChange}
                className="hidden"
              />
            </div>
          )}

          {/* thumbnail (upload mode only) */}
          {!isEdit && (
            <div className="space-y-1">
              <h3 className="text-sm font-medium text-(--color-fg-muted)">
                Thumbnail
              </h3>
              {upload.thumbnailUrl ? (
                <div className="relative">
                  <img
                    src={upload.thumbnailUrl}
                    alt="Thumbnail preview"
                    className="h-40 w-full rounded-lg object-cover"
                  />
                  <button
                    type="button"
                    onClick={upload.removeThumbnail}
                    className="absolute top-2 right-2 rounded-full bg-black/70 p-1 text-white transition-colors hover:bg-black/90"
                  >
                    <X className="size-4" />
                  </button>
                </div>
              ) : (
                <button
                  type="button"
                  onClick={() => thumbnailInputRef.current?.click()}
                  className="flex h-32 w-full flex-col items-center justify-center gap-1 rounded-lg border-2 border-dashed border-(--color-border) bg-(--color-bg) text-(--color-fg-muted) transition-colors hover:border-(--color-accent) hover:text-(--color-fg)"
                >
                  <ImagePlus className="size-8" />
                  <span className="text-xs">Add thumbnail image</span>
                  <span className="text-[10px] text-(--color-fg-muted)">
                    Optional &middot; JPG, PNG up to 975KB
                  </span>
                </button>
              )}
              <input
                type="file"
                accept="image/*"
                ref={thumbnailInputRef}
                onChange={handleThumbnailChange}
                className="hidden"
              />
            </div>
          )}

          {/* status */}
          {upload.phase.kind !== "idle" && (
            <div className="space-y-1">
              <h3 className="text-sm font-medium text-(--color-fg-muted)">
                Status
              </h3>
              <div className="rounded-lg border border-(--color-border) bg-(--color-bg) p-3">
                <div className="space-y-3">
                  {upload.phase.kind === "creating" && (
                    <div className="space-y-2">
                      <div className="flex items-center gap-2 text-sm text-(--color-fg-muted)">
                        <LoaderCircle className="size-4 animate-spin" />
                        Preparing upload...
                      </div>
                      <div className="h-1.5 overflow-hidden rounded-full bg-(--color-muted)">
                        <div className="h-full w-full animate-pulse rounded-full bg-(--color-accent) opacity-40" />
                      </div>
                    </div>
                  )}
                  {upload.phase.kind === "uploading" && (
                    <div className="space-y-2">
                      <div className="flex items-center gap-2 text-sm">
                        <ArrowUp className="size-4 text-(--color-accent)" />
                        {upload.phase.pct.toFixed(1)}% &mdash;{" "}
                        {humanBytes(upload.phase.bytesSent)} /{" "}
                        {humanBytes(upload.phase.bytesTotal)}
                      </div>
                      <div className="h-1.5 overflow-hidden rounded-full bg-(--color-muted)">
                        <div
                          className="h-full rounded-full bg-(--color-accent) transition-all"
                          style={{ width: `${upload.phase.pct}%` }}
                        />
                      </div>
                    </div>
                  )}
                  {(upload.phase.kind === "processing" ||
                    upload.phase.kind === "publishing") && (
                    <div className="space-y-2">
                      <div className="flex items-center gap-2 text-sm text-(--color-fg-muted)">
                        <LoaderCircle className="size-4 animate-spin" />
                        {upload.phase.kind === "processing"
                          ? upload.phase.serverStatus === "processing"
                            ? `Processing video${upload.phase.progress != null ? ` (${upload.phase.progress}%)` : "..."}`
                            : "Waiting to process..."
                          : "Publishing..."}
                      </div>
                      {upload.phase.kind === "processing" &&
                      upload.phase.progress != null ? (
                        <div className="h-1.5 overflow-hidden rounded-full bg-(--color-muted)">
                          <div
                            className="h-full rounded-full bg-(--color-accent) transition-all"
                            style={{ width: `${upload.phase.progress}%` }}
                          />
                        </div>
                      ) : (
                        <div className="h-1.5 overflow-hidden rounded-full bg-(--color-muted)">
                          <div className="h-full w-full animate-pulse rounded-full bg-(--color-accent) opacity-40" />
                        </div>
                      )}
                    </div>
                  )}
                  {upload.phase.kind === "ready" && (
                    <div className="flex items-center gap-2 text-sm text-emerald-500">
                      <CheckCircle2 className="size-4" />
                      Ready to publish
                    </div>
                  )}
                  {upload.phase.kind === "done" && (
                    <div className="space-y-2">
                      <div className="flex items-center gap-2 text-sm text-emerald-500">
                        <CheckCircle2 className="size-4" />
                        Published
                      </div>
                      <Link
                        to={upload.phase.videoUri}
                        className="text-xs text-(--color-accent) hover:underline"
                      >
                        View video &rarr;
                      </Link>
                    </div>
                  )}
                  {upload.phase.kind === "error" && (
                    <div className="flex items-center gap-2 text-sm text-red-500">
                      <AlertCircle className="size-4" />
                      {upload.phase.message}
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* actions */}
          <div className="space-y-2">
            {isEdit ? (
              <>
                <Button
                  onClick={() => onUpdate?.(upload)}
                  disabled={!upload.title.trim() || isUpdating}
                  className="w-full"
                >
                  {isUpdating ? "Updating..." : "Update Video"}
                </Button>
                <Button
                  variant="destructive"
                  onClick={onDelete}
                  disabled={isDeleting}
                  className="w-full"
                >
                  {isDeleting ? "Deleting..." : "Delete"}
                </Button>
                {onCancelEdit && (
                  <Button
                    variant="secondary"
                    onClick={onCancelEdit}
                    className="w-full"
                  >
                    Cancel
                  </Button>
                )}
              </>
            ) : (
              <>
                {upload.canUpload && (
                  <Button onClick={upload.startUpload} className="w-full">
                    Upload
                  </Button>
                )}
                {!upload.file && upload.phase.kind === "idle" && (
                  <Button
                    variant="secondary"
                    onClick={() => fileInputRef.current?.click()}
                    className="w-full"
                  >
                    Choose file
                  </Button>
                )}
                {upload.isUploading && upload.phase.kind !== "processing" && (
                  <Button
                    variant="destructive"
                    onClick={upload.cancelUpload}
                    className="w-full"
                  >
                    Cancel
                  </Button>
                )}
                {(upload.phase.kind === "ready" || upload.isPublishing) && (
                  <Button
                    onClick={upload.publish}
                    disabled={!upload.canPublish || upload.isPublishing}
                    className="w-full"
                  >
                    {upload.isPublishing ? "Publishing..." : "Publish"}
                  </Button>
                )}
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
