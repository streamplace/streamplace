import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { createFileRoute } from "@tanstack/react-router";
import { Edit2, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { place } from "streamplace";
import { usePDSAgent } from "../../../lib/store/hooks";

export const Route = createFileRoute("/dashboard/multistream/")({
  component: MultistreamManager,
});

interface MultistreamTarget {
  uri: string;
  record: {
    [x: string]: unknown;
    name?: string;
    url: string;
    active: boolean;
    createdAt: string;
  };
  latestEvent?: {
    status: string;
    createdAt: string;
  };
}

interface TargetFormData {
  name: string;
  url: string;
  active: boolean;
}

const emptyTargetForm: TargetFormData = {
  name: "",
  url: "",
  active: true,
};

function MultistreamManager() {
  const { t } = useTranslation("settings");
  const agent = usePDSAgent();

  const [targets, setTargets] = useState<MultistreamTarget[] | null>(null);
  const [loading, setLoading] = useState(true);

  const [showForm, setShowForm] = useState(false);
  const [editingTarget, setEditingTarget] = useState<
    MultistreamTarget | undefined
  >();
  const [formLoading, setFormLoading] = useState(false);
  const [formError, setFormError] = useState("");

  const [deleteTarget, setDeleteTarget] = useState<MultistreamTarget | null>(
    null,
  );
  const [deletingUris, setDeletingUris] = useState<Set<string>>(new Set());
  const [togglingUris, setTogglingUris] = useState<Set<string>>(new Set());

  const loadTargets = async () => {
    if (!agent) return;
    try {
      setLoading(true);
      const res = await agent.client.call(
        place.stream.multistream.listTargets,
        {
          limit: 50,
        },
      );
      setTargets(res.targets as unknown as MultistreamTarget[]);
    } catch (error: any) {
      console.error("Failed to load multistream targets:", error);
      toast.error(error.message || "Failed to load targets");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (agent) loadTargets();
  }, [agent]);

  const handleCreate = () => {
    setEditingTarget(undefined);
    setFormError("");
    setShowForm(true);
  };

  const handleEdit = (target: MultistreamTarget) => {
    setEditingTarget(target);
    setFormError("");
    setShowForm(true);
  };

  const handleSubmit = async (data: TargetFormData) => {
    if (!agent) return;
    try {
      setFormLoading(true);
      setFormError("");

      if (editingTarget) {
        await agent.client.call(place.stream.multistream.putTarget, {
          multistreamTarget: {
            $type: "place.stream.multistream.target" as const,
            name: data.name || undefined,
            url: data.url as any,
            active: data.active,
            createdAt: editingTarget.record.createdAt as any,
          },
          rkey: editingTarget.uri.split("/").pop() || "",
        });
      } else {
        await agent.client.call(place.stream.multistream.createTarget, {
          multistreamTarget: {
            $type: "place.stream.multistream.target" as const,
            name: data.name || undefined,
            url: data.url as any,
            active: data.active,
            createdAt: new Date().toISOString() as any,
          },
        });
      }

      setShowForm(false);
      setEditingTarget(undefined);
      await loadTargets();
    } catch (error: any) {
      setFormError(error.message || "Failed to save target");
    } finally {
      setFormLoading(false);
    }
  };

  const handleToggle = async (
    target: MultistreamTarget,
    newActive: boolean,
  ) => {
    if (!agent) return;
    try {
      setTogglingUris((prev) => new Set(prev).add(target.uri));
      await agent.client.call(place.stream.multistream.putTarget, {
        multistreamTarget: {
          ...(target.record as any),
          $type: "place.stream.multistream.target" as const,
          active: newActive,
        },
        rkey: target.uri.split("/").pop() || "",
      });
      await loadTargets();
    } catch (error: any) {
      toast.error(error.message || "Failed to toggle target");
    } finally {
      setTogglingUris((prev) => {
        const next = new Set(prev);
        next.delete(target.uri);
        return next;
      });
    }
  };

  const handleDelete = async () => {
    if (!agent || !deleteTarget) return;
    try {
      setDeletingUris((prev) => new Set(prev).add(deleteTarget.uri));
      await agent.client.call(place.stream.multistream.deleteTarget, {
        rkey: deleteTarget.uri.split("/").pop() || "",
      });
      setDeleteTarget(null);
      await loadTargets();
    } catch (error: any) {
      toast.error(error.message || "Failed to delete target");
    } finally {
      setDeletingUris((prev) => {
        const next = new Set(prev);
        next.delete(deleteTarget.uri);
        return next;
      });
    }
  };

  const redactUrl = (url: string) => {
    try {
      const u = new URL(url);
      return `${u.protocol}//${u.host}/redacted`;
    } catch {
      return "parsing failed";
    }
  };

  const targetTitle = (target: MultistreamTarget) => {
    if (target.record.name) return target.record.name;
    try {
      const u = new URL(target.record.url);
      return u.host;
    } catch {
      return t("untitled-multistream-target");
    }
  };

  return (
    <div className="mx-auto w-full max-w-2xl space-y-6 py-4">
      <div>
        <h1 className="font-display text-xl font-semibold">
          {t("multistream-targets")}
        </h1>
        <p className="mt-1 text-sm text-(--color-fg-muted)">
          {t("multistream-description")}
        </p>
      </div>

      <div className="flex gap-2">
        <Button onClick={handleCreate} size="sm">
          <Plus size={16} className="mr-1" />
          {t("create-multistream-target")}
        </Button>
        <Button
          onClick={() => loadTargets()}
          disabled={loading}
          variant="secondary"
          size="sm"
        >
          <RefreshCw size={16} className="mr-1" />
          {t("refresh")}
        </Button>
      </div>

      {/* Target list */}
      {loading ? (
        <div className="text-sm text-(--color-fg-muted)">Loading…</div>
      ) : targets === null ? (
        <div className="text-sm text-(--color-fg-muted)">
          {t("failed-load-multistream-targets")}
        </div>
      ) : targets.length === 0 ? (
        <div className="py-8 text-center text-sm text-(--color-fg-muted)">
          {t("no-multistream-targets-yet")}
        </div>
      ) : (
        <div className="divide-y divide-(--color-border) rounded-lg border border-(--color-border) bg-(--color-bg-elevated)">
          {targets.map((target) => {
            const isDeleting = deletingUris.has(target.uri);
            const isToggling = togglingUris.has(target.uri);
            return (
              <div
                key={target.uri}
                className="flex items-start justify-between gap-3 px-3 py-3"
                style={{ opacity: isDeleting ? 0.5 : 1 }}
              >
                <div className="min-w-0 flex-1 space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">
                      {targetTitle(target)}
                    </span>
                    {!target.record.active && (
                      <span className="rounded bg-(--color-bg) px-1.5 py-0.5 text-xs text-(--color-fg-muted)">
                        inactive
                      </span>
                    )}
                  </div>
                  <div className="truncate font-mono text-xs text-(--color-fg-muted)">
                    {redactUrl(target.record.url)}
                  </div>
                  <div className="text-xs text-(--color-fg-muted)">
                    {t("created")} {timeAgo(new Date(target.record.createdAt))}
                  </div>
                  {target.latestEvent && (
                    <div className="text-xs text-(--color-fg-muted)">
                      {t("status")}: {target.latestEvent.status} ·{" "}
                      {timeAgo(new Date(target.latestEvent.createdAt))}
                    </div>
                  )}
                </div>
                <div className="flex shrink-0 items-center gap-1">
                  {/* Active toggle */}
                  <Switch
                    size="sm"
                    checked={target.record.active}
                    disabled={isToggling}
                    onCheckedChange={(checked) => handleToggle(target, checked)}
                  />
                  <button
                    type="button"
                    onClick={() => handleEdit(target)}
                    className="rounded p-1.5 transition-colors hover:bg-(--color-bg)"
                  >
                    <Edit2 size={16} className="text-(--color-fg-muted)" />
                  </button>
                  <button
                    type="button"
                    onClick={() => setDeleteTarget(target)}
                    disabled={isDeleting}
                    className="rounded p-1.5 transition-colors hover:bg-(--color-bg) disabled:opacity-50"
                  >
                    <Trash2 size={16} className="text-destructive" />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Create/Edit dialog */}
      <MultistreamFormDialog
        open={showForm}
        onClose={() => {
          setShowForm(false);
          setEditingTarget(undefined);
        }}
        onSubmit={handleSubmit}
        target={editingTarget}
        loading={formLoading}
        error={formError}
      />

      {/* Delete confirmation */}
      <Dialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("delete")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm">
            {t("multistream-delete-target-confirmation", {
              target: deleteTarget ? targetTitle(deleteTarget) : "",
            })}
          </p>
          <p className="text-destructive mt-2 text-xs font-semibold">
            {t("this-action-cannot-be-undone")}
          </p>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setDeleteTarget(null)}>
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

function MultistreamFormDialog({
  open,
  onClose,
  onSubmit,
  target,
  loading,
  error,
}: {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: TargetFormData) => void;
  target?: MultistreamTarget;
  loading: boolean;
  error: string;
}) {
  const { t } = useTranslation("settings");
  const [form, setForm] = useState<TargetFormData>(emptyTargetForm);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [changedUrl, setChangedUrl] = useState(false);

  useEffect(() => {
    if (target) {
      setForm({
        name: target.record.name || "",
        url: "",
        active: target.record.active ?? true,
      });
      setChangedUrl(false);
    } else {
      setForm(emptyTargetForm);
      setChangedUrl(true);
    }
    setErrors({});
  }, [target, open]);

  const validate = () => {
    const newErrors: Record<string, string> = {};
    if (!form.url.trim() && !target) {
      newErrors.url = "URL is required";
    } else if (form.url.trim() && !form.url.match(/^rtmps?:\/\/.+/)) {
      newErrors.url = "URL must start with rtmp:// or rtmps://";
    }
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = () => {
    if (validate()) {
      onSubmit({
        ...form,
        url: form.url.trim() || (target ? target.record.url : ""),
      });
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {target
              ? t("multistream-edit-target")
              : t("multistream-create-target")}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Name */}
          <div>
            <label className="mb-1 block text-xs font-medium text-(--color-fg-muted)">
              {t("rtmp-target-name")} ({t("optional")})
            </label>
            <Input
              value={form.name}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, name: e.target.value }))
              }
              placeholder={t("rtmp-target-name-placeholder")}
            />
          </div>

          {/* URL */}
          <div>
            <label className="mb-1 block text-xs font-medium text-(--color-fg-muted)">
              {t("rtmp-target-url")} *
            </label>
            <Input
              value={changedUrl ? form.url : ""}
              onChange={(e) => {
                setChangedUrl(true);
                setForm((prev) => ({
                  ...prev,
                  url: e.target.value.trim().replaceAll(/\n/g, ""),
                }));
              }}
              placeholder={
                target
                  ? redactUrl(target.record.url)
                  : "rtmps://example.com:443/live/foo"
              }
            />
            {errors.url && (
              <p className="text-destructive mt-1 text-xs">{errors.url}</p>
            )}
          </div>

          {/* Active toggle */}
          <div className="flex items-center justify-between">
            <span className="text-sm">{t("active")}</span>
            <Switch
              checked={form.active}
              onCheckedChange={(checked) =>
                setForm((prev) => ({ ...prev, active: checked }))
              }
            />
          </div>

          {error && <p className="text-destructive text-xs">{error}</p>}
        </div>

        <DialogFooter>
          <Button variant="secondary" onClick={onClose} disabled={loading}>
            {t("cancel")}
          </Button>
          <Button onClick={handleSubmit} disabled={loading}>
            {loading ? t("saving") : target ? t("update") : t("create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function redactUrl(url: string) {
  try {
    const u = new URL(url);
    return `${u.protocol}//${u.host}/redacted`;
  } catch {
    return "parsing failed";
  }
}

function timeAgo(date: Date) {
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}
