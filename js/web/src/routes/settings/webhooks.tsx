import { BackLink } from "@/components/settings/back-link";
import { Button } from "@/components/ui/button";
import { CardMenuSection } from "@/components/ui/card";
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
import { Edit2, Plus, RefreshCw, Trash2, X } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { usePDSAgent } from "../../lib/store/hooks";

export const Route = createFileRoute("/settings/webhooks")({
  component: WebhookManager,
});

interface Webhook {
  id: string;
  name?: string;
  url: string;
  events: string[];
  active: boolean;
  prefix?: string;
  suffix?: string;
  rewrite?: Array<{ from: string; to: string }>;
  muteWords?: string[];
  description?: string;
  createdAt: string;
}

interface WebhookFormData {
  name: string;
  url: string;
  events: string[];
  active: boolean;
  prefix: string;
  suffix: string;
  rewrite: Array<{ from: string; to: string }>;
  muteWords: string[];
  description: string;
}

const EVENT_OPTIONS = [
  { value: "livestream", label: "Livestream" },
  { value: "chat", label: "Chat" },
];

const emptyForm: WebhookFormData = {
  name: "",
  url: "",
  events: ["livestream"],
  active: true,
  prefix: "",
  suffix: "",
  rewrite: [{ from: "", to: "" }],
  muteWords: [],
  description: "",
};

function WebhookManager() {
  const { t } = useTranslation("settings");
  const agent = usePDSAgent();

  const [webhooks, setWebhooks] = useState<Webhook[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [deletingIds, setDeletingIds] = useState<Set<string>>(new Set());

  const [showForm, setShowForm] = useState(false);
  const [editingWebhook, setEditingWebhook] = useState<Webhook | undefined>();
  const [formLoading, setFormLoading] = useState(false);

  const [deleteTarget, setDeleteTarget] = useState<Webhook | null>(null);

  const loadWebhooks = async () => {
    if (!agent) return;
    try {
      setLoading(true);
      const response = await agent.place.stream.server.listWebhooks({
        limit: 50,
      });
      const hooks = (response.data.webhooks as Webhook[] | undefined) ?? [];
      setWebhooks(hooks);
    } catch (error: any) {
      console.error("Failed to load webhooks:", error);
      toast.error(error.message || "Failed to load webhooks");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (agent) loadWebhooks();
  }, [agent]);

  const handleCreate = () => {
    setEditingWebhook(undefined);
    setShowForm(true);
  };

  const handleEdit = (webhook: Webhook) => {
    setEditingWebhook(webhook);
    setShowForm(true);
  };

  const handleSubmit = async (data: WebhookFormData) => {
    if (!agent) return;
    try {
      setFormLoading(true);
      const rewriteRules = data.rewrite.filter(
        (r) => r.from.trim() && r.to.trim(),
      );
      const payload = {
        name: data.name || undefined,
        url: data.url,
        events: data.events as ("livestream" | "chat" | "follow" | "mention")[],
        active: data.active,
        prefix: data.prefix || undefined,
        suffix: data.suffix || undefined,
        rewrite: rewriteRules.length > 0 ? rewriteRules : undefined,
        muteWords: data.muteWords.length > 0 ? data.muteWords : undefined,
        description: data.description || undefined,
      };

      if (editingWebhook) {
        await agent.place.stream.server.updateWebhook({
          id: editingWebhook.id,
          ...payload,
        });
      } else {
        await agent.place.stream.server.createWebhook(payload);
      }

      setShowForm(false);
      setEditingWebhook(undefined);
      await loadWebhooks();
    } catch (error: any) {
      console.error("Failed to save webhook:", error);
      toast.error(error.message || "Failed to save webhook");
    } finally {
      setFormLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!agent || !deleteTarget) return;
    try {
      setDeletingIds((prev) => new Set(prev).add(deleteTarget.id));
      await agent.place.stream.server.deleteWebhook({ id: deleteTarget.id });
      setDeleteTarget(null);
      await loadWebhooks();
    } catch (error: any) {
      console.error("Failed to delete webhook:", error);
      toast.error(error.message || "Failed to delete webhook");
    } finally {
      setDeletingIds((prev) => {
        const next = new Set(prev);
        next.delete(deleteTarget.id);
        return next;
      });
    }
  };

  return (
    <div className="space-y-6">
      <BackLink to="/settings/streaming" label={t("streaming")} />

      <div>
        <h1 className="text-xl font-semibold">{t("webhook-integrations")}</h1>
        <p className="text-sm text-[var(--color-fg-muted)] mt-1">
          {t("webhook-integrations-description")}
        </p>
      </div>

      <div className="flex gap-2">
        <Button onClick={handleCreate} size="sm">
          <Plus size={16} className="mr-1" />
          {t("create-webhook")}
        </Button>
        <Button
          onClick={() => loadWebhooks()}
          disabled={loading}
          variant="secondary"
          size="sm"
        >
          <RefreshCw size={16} className="mr-1" />
          {t("refresh")}
        </Button>
      </div>

      {/* Webhook list */}
      {loading ? (
        <div className="text-sm text-[var(--color-fg-muted)]">Loading…</div>
      ) : webhooks === null ? (
        <div className="text-sm text-[var(--color-fg-muted)]">
          {t("failed-load-webhooks")}
        </div>
      ) : webhooks.length === 0 ? (
        <div className="text-center py-8 text-sm text-[var(--color-fg-muted)]">
          {t("no-webhooks-yet")}
        </div>
      ) : (
        <CardMenuSection>
          {webhooks.map((webhook) => (
            <div
              key={webhook.id}
              className="px-3 py-3 flex items-start justify-between gap-3"
            >
              <div className="min-w-0 flex-1 space-y-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">
                    {webhook.name || t("untitled-webhook")}
                  </span>
                  {!webhook.active && (
                    <span className="text-xs px-1.5 py-0.5 rounded bg-[var(--color-bg)] text-[var(--color-fg-muted)]">
                      {t("inactive")}
                    </span>
                  )}
                </div>
                {webhook.description && (
                  <div className="text-xs text-[var(--color-fg-muted)]">
                    {webhook.description}
                  </div>
                )}
                <div className="text-xs text-[var(--color-fg-muted)] font-mono truncate">
                  {webhook.url}
                </div>
                <div className="flex gap-1 flex-wrap">
                  {webhook.events.map((event) => (
                    <span
                      key={event}
                      className="text-xs px-1.5 py-0.5 rounded bg-[var(--color-bg)]"
                    >
                      {event}
                    </span>
                  ))}
                </div>
              </div>
              <div className="flex gap-1 shrink-0">
                <button
                  type="button"
                  onClick={() => handleEdit(webhook)}
                  className="p-1.5 rounded hover:bg-[var(--color-bg)] transition-colors"
                >
                  <Edit2 size={16} className="text-[var(--color-fg-muted)]" />
                </button>
                <button
                  type="button"
                  onClick={() => setDeleteTarget(webhook)}
                  disabled={deletingIds.has(webhook.id)}
                  className="p-1.5 rounded hover:bg-[var(--color-bg)] transition-colors disabled:opacity-50"
                >
                  <Trash2 size={16} className="text-destructive" />
                </button>
              </div>
            </div>
          ))}
        </CardMenuSection>
      )}

      {/* Create/Edit dialog */}
      <WebhookFormDialog
        open={showForm}
        onClose={() => {
          setShowForm(false);
          setEditingWebhook(undefined);
        }}
        onSubmit={handleSubmit}
        webhook={editingWebhook}
        loading={formLoading}
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
            <DialogTitle>{t("delete-webhook")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm">
            {t("confirm-delete", {
              name: deleteTarget?.name || t("untitled-webhook"),
            })}
          </p>
          <p className="text-xs text-destructive mt-2">
            {t("action-cannot-be-undone")}
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

function WebhookFormDialog({
  open,
  onClose,
  onSubmit,
  webhook,
  loading,
}: {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: WebhookFormData) => void;
  webhook?: Webhook;
  loading: boolean;
}) {
  const { t } = useTranslation("settings");
  const [form, setForm] = useState<WebhookFormData>(emptyForm);
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (webhook) {
      setForm({
        name: webhook.name || "",
        url: webhook.url,
        events: webhook.events || ["livestream"],
        active: webhook.active ?? true,
        prefix: webhook.prefix || "",
        suffix: webhook.suffix || "",
        rewrite: webhook.rewrite?.length
          ? webhook.rewrite
          : [{ from: "", to: "" }],
        muteWords: webhook.muteWords || [],
        description: webhook.description || "",
      });
    } else {
      setForm(emptyForm);
    }
    setErrors({});
  }, [webhook, open]);

  const validate = () => {
    const newErrors: Record<string, string> = {};
    if (!form.url.trim()) {
      newErrors.url = "URL is required";
    } else if (!form.url.match(/^https?:\/\/.+/)) {
      newErrors.url = "URL must start with http:// or https://";
    }
    if (form.events.length === 0) {
      newErrors.events = "At least one event type must be selected";
    }
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = () => {
    if (validate()) onSubmit(form);
  };

  const toggleEvent = (value: string) => {
    setForm((prev) => ({
      ...prev,
      events: prev.events.includes(value)
        ? prev.events.filter((e) => e !== value)
        : [...prev.events, value],
    }));
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-lg max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {webhook ? t("edit-webhook") : t("create-webhook")}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Name */}
          <div>
            <label className="text-xs font-medium text-[var(--color-fg-muted)] mb-1 block">
              {t("name-optional")}
            </label>
            <Input
              value={form.name}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, name: e.target.value }))
              }
              placeholder={t("example-captain-hook")}
            />
          </div>

          {/* URL */}
          <div>
            <label className="text-xs font-medium text-[var(--color-fg-muted)] mb-1 block">
              Webhook URL *
            </label>
            <Input
              value={form.url}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, url: e.target.value }))
              }
              placeholder="https://discord.com/api/webhooks/..."
            />
            {errors.url && (
              <p className="text-xs text-destructive mt-1">{errors.url}</p>
            )}
          </div>

          {/* Description */}
          <div>
            <label className="text-xs font-medium text-[var(--color-fg-muted)] mb-1 block">
              Description (optional)
            </label>
            <Input
              value={form.description}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, description: e.target.value }))
              }
              placeholder="A Streamplace webhook"
            />
          </div>

          {/* Events */}
          <div>
            <label className="text-xs font-medium text-[var(--color-fg-muted)] mb-2 block">
              Events *
            </label>
            {EVENT_OPTIONS.map((opt) => (
              <label
                key={opt.value}
                className="flex items-center gap-2 mb-1.5 cursor-pointer"
              >
                <input
                  type="checkbox"
                  checked={form.events.includes(opt.value)}
                  onChange={() => toggleEvent(opt.value)}
                  className="size-4 rounded border-[var(--color-border)]"
                />
                <span className="text-sm">{opt.label}</span>
              </label>
            ))}
            {errors.events && (
              <p className="text-xs text-destructive mt-1">{errors.events}</p>
            )}
          </div>

          {/* Prefix & Suffix */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs font-medium text-[var(--color-fg-muted)] mb-1 block">
                Prefix
              </label>
              <Input
                value={form.prefix}
                onChange={(e) =>
                  setForm((prev) => ({ ...prev, prefix: e.target.value }))
                }
                placeholder="Ahoy!"
              />
            </div>
            <div>
              <label className="text-xs font-medium text-[var(--color-fg-muted)] mb-1 block">
                Suffix
              </label>
              <Input
                value={form.suffix}
                onChange={(e) =>
                  setForm((prev) => ({ ...prev, suffix: e.target.value }))
                }
                placeholder=" is now live!"
              />
            </div>
          </div>

          {/* Text Replacements */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="text-xs font-medium text-[var(--color-fg-muted)]">
                Text Replacements
              </label>
              <Button
                variant="secondary"
                size="xs"
                onClick={() =>
                  setForm((prev) => ({
                    ...prev,
                    rewrite: [...prev.rewrite, { from: "", to: "" }],
                  }))
                }
              >
                + Add
              </Button>
            </div>
            {form.rewrite.map((rule, i) => (
              <div key={i} className="flex gap-2 mb-2 items-center">
                <Input
                  value={rule.from}
                  onChange={(e) =>
                    setForm((prev) => ({
                      ...prev,
                      rewrite: prev.rewrite.map((r, j) =>
                        j === i ? { ...r, from: e.target.value } : r,
                      ),
                    }))
                  }
                  placeholder="input text"
                  className="flex-1"
                />
                <span className="text-[var(--color-fg-muted)] text-xs">→</span>
                <Input
                  value={rule.to}
                  onChange={(e) =>
                    setForm((prev) => ({
                      ...prev,
                      rewrite: prev.rewrite.map((r, j) =>
                        j === i ? { ...r, to: e.target.value } : r,
                      ),
                    }))
                  }
                  placeholder="output text"
                  className="flex-[2]"
                />
                {form.rewrite.length > 1 && (
                  <button
                    type="button"
                    onClick={() =>
                      setForm((prev) => ({
                        ...prev,
                        rewrite: prev.rewrite.filter((_, j) => j !== i),
                      }))
                    }
                    className="p-1 rounded hover:bg-[var(--color-bg)]"
                  >
                    <X size={14} className="text-destructive" />
                  </button>
                )}
              </div>
            ))}
          </div>

          {/* Mute Words */}
          <div>
            <label className="text-xs font-medium text-[var(--color-fg-muted)] mb-1 block">
              Mute Words (Chat Only)
            </label>
            <Input
              value={form.muteWords.join(", ")}
              onChange={(e) =>
                setForm((prev) => ({
                  ...prev,
                  muteWords: e.target.value
                    .split(",")
                    .map((w) => w.trim())
                    .filter(Boolean),
                }))
              }
              placeholder="word1, word2, word3"
            />
          </div>

          {/* Active toggle */}
          <div className="flex items-center justify-between">
            <span className="text-sm">Active</span>
            <Switch
              checked={form.active}
              onCheckedChange={(checked) =>
                setForm((prev) => ({ ...prev, active: checked }))
              }
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="secondary" onClick={onClose} disabled={loading}>
            {t("cancel")}
          </Button>
          <Button onClick={handleSubmit} disabled={loading}>
            {loading ? t("saving") : webhook ? t("update") : t("create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
