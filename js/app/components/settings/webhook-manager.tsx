import {
  Button,
  Dialog,
  DialogFooter,
  IconButton,
  Input,
  MenuContainer,
  MenuGroup,
  MenuSeparator,
  ResponsiveDialog,
  Switch,
  Text,
  zero,
} from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { EmptyState, EmptyStateTile } from "components/empty-state";
import Loading from "components/loading/loading";
import {
  Edit2,
  Plus,
  RefreshCw,
  Trash2,
  Webhook as WebhookIcon,
  X,
} from "lucide-react-native";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Alert, Pressable, ScrollView, View } from "react-native";
import { place } from "streamplace";
import { SettingsRowItem } from "./components/settings-navigation-item";
import { SettingsViewHeader } from "./components/settings-view-header";

const {
  m,
  mt,
  mr,
  mb,
  ml,
  mx,
  my,
  p,
  pt,
  pr,
  pb,
  pl,
  px,
  py,
  w,
  h,
  r,
  layout,
  borders,
  flex,
  gap,
} = zero;

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
  updatedAt?: string;
  lastTriggered?: string;
  errorCount?: number;
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

type WebhookEvent =
  | "livestream"
  | "chat"
  | "follow"
  | "mention"
  | "stream.received";

const VALID_WEBHOOK_EVENTS: WebhookEvent[] = [
  "livestream",
  "chat",
  "follow",
  "mention",
  "stream.received",
];

const EVENT_OPTIONS = [
  { value: "livestream", labelKey: "events-livestream" },
  { value: "chat", labelKey: "events-chat" },
  { value: "stream.received", labelKey: "events-stream-received" },
];

const EVENT_LABEL_KEYS = new Map(
  EVENT_OPTIONS.map((option) => [option.value, option.labelKey]),
);

function getEventLabel(t: (key: string) => string, event: string) {
  const labelKey = EVENT_LABEL_KEYS.get(event);
  return labelKey ? t(labelKey) : event;
}

function WebhookRow({
  webhook,
  onEdit,
  onDelete,
  isDeleting,
}: {
  webhook: Webhook;
  onEdit: (webhook: Webhook) => void;
  onDelete: (id: string) => void;
  isDeleting: boolean;
}) {
  const { theme } = zero.useTheme();
  const { t } = useTranslation("settings");
  const isDiscord = webhook.url
    .toLowerCase()
    .startsWith("https://discord.com/api/webhooks");

  return (
    <SettingsRowItem>
      <View style={[zero.gap.row[1], zero.flex.values[1]]}>
        {/* Name and Active Status */}
        <View style={[zero.layout.flex.row, zero.layout.flex.alignCenter]}>
          <Text size="lg">{webhook.name || t("untitled-webhook")}</Text>
          {!webhook.active && (
            <View
              style={[
                { backgroundColor: theme.colors.surface2 },
                zero.px[2],
                zero.py[1],
                zero.r.full,
                zero.ml[2],
              ]}
            >
              <Text size="sm" style={{ color: theme.colors.text3 }}>
                {t("inactive")}
              </Text>
            </View>
          )}
        </View>

        {/* Description */}
        {webhook.description && (
          <Text size="sm" style={{ color: theme.colors.text3 }}>
            {webhook.description}
          </Text>
        )}

        {/* URL */}
        <View
          style={[
            zero.layout.flex.row,
            zero.layout.flex.alignCenter,
            zero.gap.column[1],
          ]}
        >
          <Text size="sm" style={{ color: theme.colors.text3 }}>
            URL:
          </Text>
          <Text
            size="sm"
            numberOfLines={1}
            ellipsizeMode="middle"
            style={{ color: theme.colors.text2 }}
          >
            {webhook.url}
          </Text>
        </View>

        {/* Events */}
        <View
          style={{
            flexDirection: "row",
            gap: 8,
            flexWrap: "wrap",
            alignItems: "center",
          }}
        >
          <Text size="sm" style={{ color: theme.colors.text3 }}>
            {t("activates-on")}
          </Text>
          {webhook.events.map((event) => (
            <View
              key={event}
              style={[
                { backgroundColor: theme.colors.surface2 },
                zero.px[2],
                zero.py[1],
                zero.r.full,
              ]}
            >
              <Text size="sm" style={{ color: theme.colors.text2 }}>
                {getEventLabel(t, event)}
              </Text>
            </View>
          ))}
        </View>
      </View>

      {/* Actions */}
      <View style={{ flexDirection: "row", gap: 4, marginLeft: 12 }}>
        <IconButton
          size="sm"
          variant="ghost"
          accessibilityLabel="Edit webhook"
          onPress={() => onEdit(webhook)}
        >
          <Edit2 size={18} color={theme.colors.text3} />
        </IconButton>
        <IconButton
          size="sm"
          variant="ghost"
          accessibilityLabel="Delete webhook"
          disabled={isDeleting}
          onPress={() => onDelete(webhook.id)}
        >
          <Trash2 size={18} color={theme.colors.danger} />
        </IconButton>
      </View>
    </SettingsRowItem>
  );
}

function WebhookForm({
  webhook,
  isVisible,
  onClose,
  onSubmit,
  isLoading,
}: {
  webhook?: Webhook;
  isVisible: boolean;
  onClose: () => void;
  onSubmit: (data: WebhookFormData) => void;
  isLoading: boolean;
}) {
  const [formData, setFormData] = useState<WebhookFormData>({
    name: webhook?.name || "",
    url: webhook?.url || "",
    events: webhook?.events || ["livestream"],
    active: webhook?.active ?? true,
    prefix: webhook?.prefix || "",
    suffix: webhook?.suffix || "",
    rewrite: webhook?.rewrite || [{ from: "", to: "" }],
    muteWords: webhook?.muteWords || [],
    description: webhook?.description || "",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const { t } = useTranslation("settings");
  const { theme } = zero.useTheme();

  const labelStyle = {
    color: theme.colors.text2,
    marginBottom: 8,
  } as const;

  // Update form data when webhook prop changes (for editing)
  useEffect(() => {
    if (webhook) {
      setFormData({
        name: webhook.name || "",
        url: webhook.url || "",
        events: webhook.events || ["livestream"],
        active: webhook.active ?? true,
        prefix: webhook.prefix || "",
        suffix: webhook.suffix || "",
        rewrite: webhook.rewrite || [{ from: "", to: "" }],
        muteWords: webhook.muteWords || [],
        description: webhook.description || "",
      });
    } else {
      // Reset form for new webhook
      setFormData({
        name: "",
        url: "",
        events: ["livestream"],
        active: true,
        prefix: "",
        suffix: "",
        rewrite: [{ from: "", to: "" }],
        muteWords: [],
        description: "",
      });
    }
  }, [webhook]);

  const validateForm = () => {
    const newErrors: Record<string, string> = {};

    if (!formData.url.trim()) {
      newErrors.url = "URL is required";
    } else if (!formData.url.match(/^https?:\/\/.+/)) {
      newErrors.url = "URL must start with http:// or https://";
    }

    if (formData.events.length === 0) {
      newErrors.events = "At least one event type must be selected";
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = () => {
    if (validateForm()) {
      onSubmit(formData);
    }
  };

  const toggleEvent = (eventValue: string) => {
    setFormData((prev) => ({
      ...prev,
      events: prev.events.includes(eventValue)
        ? prev.events.filter((e) => e !== eventValue)
        : [...prev.events, eventValue],
    }));
  };

  const addReplacement = () => {
    setFormData((prev) => ({
      ...prev,
      rewrite: [...prev.rewrite, { from: "", to: "" }],
    }));
  };

  const removeReplacement = (index: number) => {
    setFormData((prev) => ({
      ...prev,
      rewrite: prev.rewrite.filter((_, i) => i !== index),
    }));
  };

  const updateReplacement = (
    index: number,
    field: "from" | "to",
    value: string,
  ) => {
    setFormData((prev) => ({
      ...prev,
      rewrite: prev.rewrite.map((item, i) =>
        i === index ? { ...item, [field]: value } : item,
      ),
    }));
  };

  return (
    <ResponsiveDialog
      open={isVisible}
      onOpenChange={(open) => !open && onClose()}
      title={webhook ? t("edit-webhook") : t("create-webhook")}
      size="lg"
      dismissible={false}
    >
      <View style={[w.percent[100]]}>
        {/* Name */}
        <View style={[mb[4]]}>
          <Text size="sm" weight="medium" style={labelStyle}>
            {t("name-optional")}
          </Text>
          <Input
            value={formData.name}
            onChangeText={(text) =>
              setFormData((prev) => ({ ...prev, name: text }))
            }
            placeholder={t("example-captain-hook")}
          />
        </View>

        {/* URL */}
        <View style={[mb[4]]}>
          <Text size="sm" weight="medium" style={labelStyle}>
            Webhook URL *
          </Text>
          <Input
            value={formData.url}
            onChangeText={(text) =>
              setFormData((prev) => ({ ...prev, url: text }))
            }
            placeholder="https://discord.com/api/webhooks/..."
            multiline
          />
          {errors.url && (
            <Text
              size="sm"
              style={{ color: theme.colors.danger, marginTop: 4 }}
            >
              {errors.url}
            </Text>
          )}
        </View>

        {/* Description */}
        <View style={[mb[4]]}>
          <Text size="sm" weight="medium" style={labelStyle}>
            Description (optional)
          </Text>
          <Input
            value={formData.description}
            onChangeText={(text) =>
              setFormData((prev) => ({ ...prev, description: text }))
            }
            placeholder="A Streamplace webhook"
            multiline
          />
        </View>

        {/* Events */}
        <View style={[mb[4]]}>
          <Text size="sm" weight="medium" style={labelStyle}>
            Events *
          </Text>
          {EVENT_OPTIONS.map((option) => {
            const checked = formData.events.includes(option.value);
            return (
              <Pressable
                key={option.value}
                style={[layout.flex.row, layout.flex.alignCenter, mb[2]]}
                onPress={() => toggleEvent(option.value)}
              >
                <View
                  style={[
                    w[5],
                    h[5],
                    borders.width.thin,
                    {
                      borderColor: checked
                        ? theme.colors.primary
                        : theme.colors.borderStrong,
                    },
                    r.sm,
                    mr[3],
                    layout.flex.center,
                    checked && { backgroundColor: theme.colors.primary },
                  ]}
                >
                  {checked && (
                    <Text
                      style={{
                        color: theme.colors.primaryForeground,
                        fontSize: 12,
                      }}
                    >
                      ✓
                    </Text>
                  )}
                </View>
                <Text style={{ color: theme.colors.text2 }}>
                  {t(option.labelKey)}
                </Text>
              </Pressable>
            );
          })}
          {errors.events && (
            <Text
              size="sm"
              style={{ color: theme.colors.danger, marginTop: 4 }}
            >
              {errors.events}
            </Text>
          )}
        </View>

        {/* Prefix & Suffix */}
        <View style={[layout.flex.row, gap.all[3], mb[4]]}>
          <View style={[flex.values[1]]}>
            <Text size="sm" weight="medium" style={labelStyle}>
              Prefix
            </Text>
            <Input
              value={formData.prefix}
              onChangeText={(text) =>
                setFormData((prev) => ({ ...prev, prefix: text }))
              }
              placeholder="Ahoy!"
            />
          </View>
          <View style={[flex.values[1]]}>
            <Text size="sm" weight="medium" style={labelStyle}>
              Suffix
            </Text>
            <Input
              value={formData.suffix}
              onChangeText={(text) =>
                setFormData((prev) => ({ ...prev, suffix: text }))
              }
              placeholder=" is now live!"
            />
          </View>
        </View>

        {/* Example message preview */}
        <View style={[mb[4]]}>
          <Text size="sm" weight="medium" style={labelStyle}>
            Example
          </Text>
          <View
            style={[
              { backgroundColor: theme.colors.surface0 },
              p[3],
              r.md,
              borders.width.thin,
              { borderColor: theme.colors.borderSubtle },
            ]}
          >
            <Text style={{ color: theme.colors.text3 }}>
              {formData.prefix}
              <Text style={{ color: theme.colors.primary }}>
                {"{username}"}
              </Text>
              {formData.suffix}
            </Text>
          </View>
        </View>

        {/* Replacements */}
        <View style={[mb[4]]}>
          <View
            style={[
              layout.flex.row,
              layout.flex.alignCenter,
              layout.flex.spaceBetween,
              mb[2],
            ]}
          >
            <Text
              size="sm"
              weight="medium"
              style={{ color: theme.colors.text2 }}
            >
              Text Replacements
            </Text>
            <Button
              variant="secondary"
              size="sm"
              width="min"
              leftIcon={<Plus size={14} />}
              onPress={addReplacement}
            >
              Add
            </Button>
          </View>
          <Text
            size="sm"
            style={{ color: theme.colors.text3, marginBottom: 12 }}
          >
            Replace text in messages. Example: "#gaming" →
            "&lt;@1384516462017777734&gt;"
          </Text>

          {formData.rewrite.map((replacement, index) => (
            <View
              key={index}
              style={[
                layout.flex.row,
                gap.all[2],
                mb[2],
                layout.flex.alignCenter,
              ]}
            >
              <View style={[flex.values[1]]}>
                <Input
                  value={replacement.from}
                  onChangeText={(text) =>
                    updateReplacement(index, "from", text)
                  }
                  placeholder="input text"
                />
              </View>
              <Text style={[{ color: theme.colors.text3 }, px[1]]}>→</Text>
              <View style={[flex.values[2]]}>
                <Input
                  value={replacement.to}
                  onChangeText={(text) => updateReplacement(index, "to", text)}
                  placeholder="output text"
                />
              </View>
              {formData.rewrite.length > 1 && (
                <IconButton
                  size="sm"
                  variant="ghost"
                  accessibilityLabel="Remove replacement"
                  onPress={() => removeReplacement(index)}
                >
                  <X size={18} color={theme.colors.danger} />
                </IconButton>
              )}
            </View>
          ))}
        </View>

        {/* Mute Words */}
        <View style={[mb[4]]}>
          <Text size="sm" weight="medium" style={labelStyle}>
            Mute Words (Chat Only)
          </Text>
          <Text
            size="sm"
            style={{ color: theme.colors.text3, marginBottom: 12 }}
          >
            Chat messages containing any of these words will not be forwarded.
            Useful for avoiding reforwarding of forwarded messages.
          </Text>
          <Input
            value={formData.muteWords.join(", ")}
            onChangeText={(text) =>
              setFormData((prev) => ({
                ...prev,
                muteWords: text
                  .split(",")
                  .map((w) => w.trim())
                  .filter((w) => w),
              }))
            }
            placeholder="word1, word2, word3"
            multiline
          />
        </View>

        {/* Active toggle */}
        <View
          style={[
            layout.flex.row,
            layout.flex.alignCenter,
            layout.flex.spaceBetween,
            p[3],
            r.md,
            borders.width.thin,
            {
              backgroundColor: theme.colors.surface1,
              borderColor: theme.colors.borderSubtle,
            },
            mb[6],
          ]}
        >
          <Text weight="medium" style={{ color: theme.colors.text2 }}>
            Active
          </Text>
          <Switch
            value={formData.active}
            onValueChange={(active) =>
              setFormData((prev) => ({ ...prev, active }))
            }
          />
        </View>
      </View>

      <DialogFooter>
        <Button
          width="min"
          variant="secondary"
          onPress={onClose}
          disabled={isLoading}
        >
          {t("cancel")}
        </Button>
        <Button width="min" onPress={handleSubmit} disabled={isLoading}>
          {isLoading ? t("saving") : webhook ? t("update") : t("create")}
        </Button>
      </DialogFooter>
    </ResponsiveDialog>
  );
}

export default function WebhookManager() {
  const agent = usePDSAgent();
  const { theme } = zero.useTheme();
  const [webhooks, setWebhooks] = useState<Webhook[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [deletingWebhooks, setDeletingWebhooks] = useState<Set<string>>(
    new Set(),
  );
  const [editingWebhook, setEditingWebhook] = useState<Webhook | undefined>();
  const [showForm, setShowForm] = useState(false);
  const [formLoading, setFormLoading] = useState(false);
  const [deleteDialog, setDeleteDialog] = useState<{
    isVisible: boolean;
    webhook: Webhook | null;
  }>({ isVisible: false, webhook: null });

  const { t } = useTranslation("settings");

  const loadWebhooks = async () => {
    if (!agent) return;

    try {
      setLoading(true);
      const response = await agent.client.call(
        place.stream.server.listWebhooks,
        {
          limit: 50,
        },
      );
      // Filter out unknown event types returned by the server.
      // todo: find a better way to check this
      if (response.webhooks) {
        for (const webhook of response.webhooks) {
          webhook.events = (webhook.events as string[]).filter((event) =>
            VALID_WEBHOOK_EVENTS.includes(event as WebhookEvent),
          ) as WebhookEvent[];
        }
      }
      setWebhooks((response.webhooks as any) || []);
    } catch (error) {
      console.error("Failed to load webhooks:", error);
      Alert.alert("Error", "Failed to load webhooks. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  const createWebhook = async (data: WebhookFormData) => {
    if (!agent) return;

    try {
      setFormLoading(true);

      // Filter out empty rewrite rules
      const rewriteRules = data.rewrite.filter(
        (r) => r.from.trim() && r.to.trim(),
      );

      await agent.client.call(place.stream.server.createWebhook, {
        name: data.name || undefined,
        url: data.url as any,
        events: data.events as WebhookEvent[],
        active: data.active,
        prefix: data.prefix || undefined,
        suffix: data.suffix || undefined,
        rewrite: rewriteRules.length > 0 ? rewriteRules : undefined,
        muteWords: data.muteWords.length > 0 ? data.muteWords : undefined,
        description: data.description || undefined,
      });
      setShowForm(false);
      setEditingWebhook(undefined);
      await loadWebhooks();
    } catch (error: any) {
      console.error("Failed to create webhook:", error);
      Alert.alert(
        "Error",
        error.message || "Failed to create webhook. Please try again.",
      );
    } finally {
      setFormLoading(false);
    }
  };

  const updateWebhook = async (data: WebhookFormData) => {
    if (!agent || !editingWebhook) return;

    try {
      setFormLoading(true);

      // Filter out empty rewrite rules
      const rewriteRules = data.rewrite.filter(
        (r) => r.from.trim() && r.to.trim(),
      );

      await agent.client.call(place.stream.server.updateWebhook, {
        id: editingWebhook.id,
        name: data.name || undefined,
        url: data.url as any,
        events: data.events as WebhookEvent[],
        active: data.active,
        prefix: data.prefix || undefined,
        suffix: data.suffix || undefined,
        rewrite: rewriteRules.length > 0 ? rewriteRules : undefined,
        muteWords: data.muteWords.length > 0 ? data.muteWords : undefined,
        description: data.description || undefined,
      });
      setShowForm(false);
      setEditingWebhook(undefined);
      await loadWebhooks();
    } catch (error: any) {
      console.error("Failed to update webhook:", error);
      Alert.alert(
        "Error",
        error.message || "Failed to update webhook. Please try again.",
      );
    } finally {
      setFormLoading(false);
    }
  };

  const deleteWebhook = async (id: string) => {
    const webhook = webhooks?.find((w) => w.id === id);
    if (!webhook) return;

    setDeleteDialog({ isVisible: true, webhook });
  };

  const confirmDelete = async () => {
    if (!agent || !deleteDialog.webhook) return;

    const id = deleteDialog.webhook.id;

    try {
      setDeletingWebhooks((prev) => new Set(prev).add(id));
      await agent.client.call(place.stream.server.deleteWebhook, { id });
      await loadWebhooks();
      setDeleteDialog({ isVisible: false, webhook: null });
    } catch (error: any) {
      console.error("Failed to delete webhook:", error);
      Alert.alert(
        "Error",
        error.message || "Failed to delete webhook. Please try again.",
      );
    } finally {
      setDeletingWebhooks((prev) => {
        const newSet = new Set(prev);
        newSet.delete(id);
        return newSet;
      });
    }
  };

  const handleEdit = (webhook: Webhook) => {
    setEditingWebhook(webhook);
    setShowForm(true);
  };

  const handleCreate = () => {
    setEditingWebhook(undefined);
    setShowForm(true);
  };

  const handleSubmit = (data: WebhookFormData) => {
    if (editingWebhook) {
      updateWebhook(data);
    } else {
      createWebhook(data);
    }
  };

  useEffect(() => {
    if (!agent) return;
    loadWebhooks();
  }, [agent]);

  if (!agent) {
    return <Loading />;
  }

  const deleteDisabled = deleteDialog.webhook
    ? deletingWebhooks.has(deleteDialog.webhook.id)
    : false;

  return (
    <>
      <ScrollView contentContainerStyle={{ flexGrow: 1, alignItems: "center" }}>
        <View
          style={{
            width: "100%",
            maxWidth: 800,
            flex: 1,
            paddingHorizontal: 32,
            paddingVertical: 24,
          }}
        >
          {/* Header */}
          <SettingsViewHeader
            title={t("webhook-integrations")}
            description={t("webhook-integrations-description")}
            action={
              <View style={{ flexDirection: "row", gap: 8 }}>
                <Button
                  size="sm"
                  width="min"
                  variant="secondary"
                  leftIcon={<RefreshCw size={16} />}
                  disabled={loading}
                  onPress={loadWebhooks}
                >
                  {t("refresh")}
                </Button>
                <Button
                  size="sm"
                  width="min"
                  variant="primary"
                  leftIcon={<Plus size={16} />}
                  onPress={handleCreate}
                >
                  {t("create")}
                </Button>
              </View>
            }
          />

          {/* Content */}
          {loading ? (
            <View
              style={{
                flex: 1,
                minHeight: 320,
                justifyContent: "center",
                alignItems: "center",
              }}
            >
              <Loading />
            </View>
          ) : webhooks === null ? (
            <View style={[layout.flex.center, mt[8]]}>
              <Text style={{ color: theme.colors.danger }}>
                {t("failed-load-webhooks")}
              </Text>
            </View>
          ) : webhooks.length === 0 ? (
            <EmptyState
              illustration={<EmptyStateTile icon={WebhookIcon} />}
              title={t("no-webhooks-yet")}
              subtitle={t("create-first-webhook-description")}
              action={
                <Button
                  size="sm"
                  leftIcon={<Plus size={16} />}
                  onPress={handleCreate}
                >
                  {t("create-webhook")}
                </Button>
              }
            />
          ) : (
            <MenuContainer>
              <MenuGroup>
                {webhooks.map((webhook, index) => (
                  <View key={webhook.id}>
                    {index > 0 && <MenuSeparator />}
                    <WebhookRow
                      webhook={webhook}
                      onEdit={handleEdit}
                      onDelete={deleteWebhook}
                      isDeleting={deletingWebhooks.has(webhook.id)}
                    />
                  </View>
                ))}
              </MenuGroup>
            </MenuContainer>
          )}
        </View>
      </ScrollView>

      <WebhookForm
        webhook={editingWebhook}
        isVisible={showForm}
        onClose={() => {
          setShowForm(false);
          setEditingWebhook(undefined);
        }}
        onSubmit={handleSubmit}
        isLoading={formLoading}
      />

      <Dialog
        open={deleteDialog.isVisible}
        onOpenChange={(open) =>
          !open && setDeleteDialog({ isVisible: false, webhook: null })
        }
        title={t("delete-webhook")}
        dismissible={false}
      >
        <View style={{ width: "100%", marginTop: 8, marginBottom: 16, gap: 8 }}>
          <Text size="lg">
            {t("confirm-delete", {
              name: deleteDialog.webhook?.name || t("untitled-webhook"),
            })}
          </Text>
          <Text size="sm" style={{ color: theme.colors.text3 }}>
            {t("action-cannot-be-undone")}
          </Text>
          <Text size="sm" style={{ color: theme.colors.text3 }}>
            {t("webhook-will-no-longer-receive-events")}
          </Text>
        </View>

        <DialogFooter>
          <Button
            variant="secondary"
            width="min"
            onPress={() => setDeleteDialog({ isVisible: false, webhook: null })}
            disabled={deleteDisabled}
          >
            {t("cancel")}
          </Button>
          <Button
            variant="danger"
            width="min"
            onPress={confirmDelete}
            disabled={deleteDisabled}
          >
            {deleteDialog.webhook &&
            deletingWebhooks.has(deleteDialog.webhook.id)
              ? t("deleting")
              : t("delete")}
          </Button>
        </DialogFooter>
      </Dialog>
    </>
  );
}
