import { useModerationActions } from "@/hooks/use-moderation-actions";
import { useToast } from "@/hooks/use-toast";
import { useSession } from "@/lib/session";
import {
  ComAtprotoModerationCreateReport,
  ComAtprotoModerationDefs,
} from "@atproto/api";
import type { LivestreamStore } from "@streamplace/core";
import {
  CheckCircle2,
  Circle,
  EllipsisVertical,
  LoaderCircle,
} from "lucide-react";
import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "zustand";
import { Button, buttonVariants } from "../ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";
import { Textarea } from "../ui/textarea";

export type ReportSubject =
  ComAtprotoModerationCreateReport.InputSchema["subject"];

// AT Protocol moderation reason types with labels (mirrors the mobile app).
const REPORT_REASONS = [
  {
    value: ComAtprotoModerationDefs.REASONSPAM,
    labelKey: "report-reason-spam",
    descKey: "report-reason-spam-desc",
  },
  {
    value: ComAtprotoModerationDefs.REASONVIOLATION,
    labelKey: "report-reason-violation",
    descKey: "report-reason-violation-desc",
  },
  {
    value: ComAtprotoModerationDefs.REASONMISLEADING,
    labelKey: "report-reason-misleading",
    descKey: "report-reason-misleading-desc",
  },
  {
    value: ComAtprotoModerationDefs.REASONSEXUAL,
    labelKey: "report-reason-sexual",
    descKey: "report-reason-sexual-desc",
  },
  {
    value: ComAtprotoModerationDefs.REASONRUDE,
    labelKey: "report-reason-rude",
    descKey: "report-reason-rude-desc",
  },
  {
    value: ComAtprotoModerationDefs.REASONOTHER,
    labelKey: "report-reason-other",
    descKey: "report-reason-other-desc",
  },
] as const;

/**
 * File a moderation report against a subject (chat message strongRef,
 * account repoRef, …) with an AT Protocol reason type and optional context.
 */
export function ReportDialog({
  subject,
  onClose,
}: {
  subject: ReportSubject;
  onClose: () => void;
}) {
  const { t } = useTranslation("common");
  const toast = useToast();
  const { submitReport } = useModerationActions();
  const [selectedReason, setSelectedReason] = useState<string | null>(null);
  const [comment, setComment] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = useCallback(async () => {
    if (!selectedReason || isSubmitting) return;
    setIsSubmitting(true);
    setError(null);
    try {
      await submitReport(subject, selectedReason, comment.trim() || undefined);
      toast.show(t("report-submitted"), "", { duration: 3000 });
      onClose();
    } catch (error) {
      console.error("Failed to submit report:", error);
      setError(t("report-failed"));
    } finally {
      setIsSubmitting(false);
    }
  }, [
    comment,
    isSubmitting,
    onClose,
    selectedReason,
    subject,
    submitReport,
    t,
    toast,
  ]);

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("report-title")}</DialogTitle>
          <DialogDescription>{t("report-description")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-1">
          {REPORT_REASONS.map((reason) => {
            const selected = selectedReason === reason.value;
            return (
              <button
                key={reason.value}
                type="button"
                onClick={() => setSelectedReason(reason.value)}
                className={`flex w-full items-center gap-3 rounded-md p-2.5 text-left transition-colors hover:bg-(--color-bg-overlay) ${
                  selected ? "bg-(--color-bg-overlay)" : ""
                }`}
              >
                {selected ? (
                  <CheckCircle2 className="size-5 shrink-0" />
                ) : (
                  <Circle className="size-5 shrink-0 text-(--color-fg-muted)" />
                )}
                <span className="min-w-0">
                  <span className="block text-sm font-semibold">
                    {t(reason.labelKey)}
                  </span>
                  <span className="block text-sm text-(--color-fg-muted)">
                    {t(reason.descKey)}
                  </span>
                </span>
              </button>
            );
          })}
        </div>

        <div className="space-y-2">
          <label
            htmlFor="report-comment"
            className="text-sm text-(--color-fg-muted)"
          >
            {t("report-comments-label")}
          </label>
          <Textarea
            id="report-comment"
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            maxLength={500}
            rows={3}
            placeholder={t("report-comments-placeholder")}
          />
          {error && <p className="text-destructive text-sm">{error}</p>}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isSubmitting}>
            {t("cancel")}
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={!selectedReason || isSubmitting}
          >
            {isSubmitting ? (
              <>
                <LoaderCircle className="animate-spin" />{" "}
                {t("report-submitting")}
              </>
            ) : (
              t("report-submit")
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * Kebab on the stream header with the stream-level report entry points:
 * the livestream record (strongRef) and the channel account (repoRef).
 * Hidden from the streamer themselves and from logged-out viewers.
 */
export function StreamReportMenu({ store }: { store: LivestreamStore }) {
  const { t } = useTranslation("common");
  const { did } = useSession();
  const livestream = useStore(store, (s) => s.livestream);
  const [subject, setSubject] = useState<ReportSubject | null>(null);

  if (!did || !livestream || did === livestream.author.did) return null;

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          className={buttonVariants({ variant: "outline", size: "icon-lg" })}
          aria-label={t("report-title")}
          title={t("report-title")}
        >
          <EllipsisVertical />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-48">
          <DropdownMenuItem
            onClick={() =>
              setSubject({
                $type: "com.atproto.repo.strongRef",
                uri: livestream.uri,
                cid: livestream.cid,
              })
            }
          >
            {t("report-live-stream")}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() =>
              setSubject({
                $type: "com.atproto.admin.defs#repoRef",
                did: livestream.author.did,
              })
            }
          >
            {t("report-something-else")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      {subject && (
        <ReportDialog subject={subject} onClose={() => setSubject(null)} />
      )}
    </>
  );
}
