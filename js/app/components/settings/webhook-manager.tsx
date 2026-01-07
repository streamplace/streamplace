import {
  Button,
  Dialog,
  DialogFooter,
  Input,
  MenuContainer,
  MenuGroup,
  MenuSeparator,
  ResponsiveDialog,
  Text,
  zero,
} from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import AQLink from "components/aqlink";
import Loading from "components/loading/loading";
import { Edit2, Plus, RefreshCw, Trash2, X } from "lucide-react-native";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Alert,
  Pressable,
  ScrollView,
  Switch,
  View,
  VirtualizedList,
} from "react-native";
import { SettingsRowItem } from "./components/settings-navigation-item";

const {
  atoms,
  bg,
  text,
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

const EVENT_OPTIONS = [
  { value: "livestream", labelKey: "events-livestream" },
  { value: "chat", labelKey: "events-chat" },
];

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
  const { theme, zero: z } = zero.useTheme();
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
            <View style={[z.bg.muted, zero.px[2], zero.r.full, zero.ml[2]]}>
              <Text size="sm" muted>
                {t("inactive")}
              </Text>
            </View>
          )}
        </View>

        {/* Description */}
        {webhook.description && (
          <Text size="sm" color="muted">
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
          <Text muted size="sm">
            URL:
          </Text>
          <Text numberOfLines={1} ellipsizeMode="middle">
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
          <Text size="sm" color="muted">
            {t("activates-on")}
          </Text>
          {webhook.events.map((event) => (
            <View
              key={event}
              style={[z.bg.muted, zero.px[2], zero.py[1], zero.r.full]}
            >
              <Text size="sm">{t(`events-${event}`)}</Text>
            </View>
          ))}
        </View>
      </View>

      {/* Actions */}
      <View style={{ flexDirection: "row", gap: 8, marginLeft: 12 }}>
        <Pressable
          onPress={() => onEdit(webhook)}
          style={({ pressed }) => [
            {
              padding: 8,
              borderRadius: 6,
              backgroundColor: pressed ? "#ffffff08" : "transparent",
            },
          ]}
        >
          <Edit2 size={18} color={theme.colors.textMuted} />
        </Pressable>
        <Pressable
          onPress={() => onDelete(webhook.id)}
          disabled={isDeleting}
          style={({ pressed }) => [
            {
              padding: 8,
              borderRadius: 6,
              backgroundColor: pressed ? "#ffffff08" : "transparent",
              opacity: isDeleting ? 0.5 : 1,
            },
          ]}
        >
          <Trash2 size={18} color={theme.colors.destructive} />
        </Pressable>
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
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
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
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
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
            <Text style={[text.red[600], mt[1], { fontSize: 12 }]}>
              {errors.url}
            </Text>
          )}
        </View>

        {/* Description */}
        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
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
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Events *
          </Text>
          {EVENT_OPTIONS.map((option) => (
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
                  borders.color.gray[300],
                  r[1],
                  mr[3],
                  layout.flex.center,
                  formData.events.includes(option.value) && bg.blue[500],
                ]}
              >
                {formData.events.includes(option.value) && (
                  <Text style={[text.white, { fontSize: 12 }]}>✓</Text>
                )}
              </View>
              <Text style={[text.gray[300], { fontSize: 14 }]}>
                {t(option.labelKey)}
              </Text>
            </Pressable>
          ))}
          {errors.events && (
            <Text style={[text.red[600], mt[1], { fontSize: 12 }]}>
              {errors.events}
            </Text>
          )}
        </View>

        {/* Prefix & Suffix */}
        <View style={[layout.flex.row, gap.all[3], mb[4]]}>
          <View style={[flex.values[1]]}>
            <Text
              style={[
                text.gray[400],
                mb[2],
                { fontSize: 14, fontWeight: "500" },
              ]}
            >
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
            <Text
              style={[
                text.gray[400],
                mb[2],
                { fontSize: 14, fontWeight: "500" },
              ]}
            >
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
            <Text style={[text.gray[300], { fontSize: 14, fontWeight: "500" }]}>
              Text Replacements
            </Text>
            <Button width="min" size="pill" onPress={addReplacement}>
              <Text style={[text.white, { fontSize: 12 }]}>+ Add</Text>
            </Button>
          </View>
          <Text style={[text.gray[300], mb[3], { fontSize: 12 }]}>
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
              <Text style={[text.gray[400], px[1]]}>→</Text>
              <View style={[flex.values[2]]}>
                <Input
                  value={replacement.to}
                  onChangeText={(text) => updateReplacement(index, "to", text)}
                  placeholder="output text"
                />
              </View>
              {formData.rewrite.length > 1 && (
                <Button
                  style={[m[0], p[0]]}
                  variant="destructive"
                  onPress={() => removeReplacement(index)}
                >
                  <X size={20} />
                </Button>
              )}
            </View>
          ))}
        </View>

        {/* Mute Words */}
        <View style={[mb[4]]}>
          <Text
            style={[text.gray[400], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Mute Words (Chat Only)
          </Text>
          <Text style={[text.gray[400], mb[3], { fontSize: 12 }]}>
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

        {/* Example message text */}
        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Example
          </Text>
          <View
            style={[
              bg.neutral[800],
              p[3],
              r.md,
              borders.width.thin,
              borders.color.gray[200],
            ]}
          >
            <Text style={[text.gray[400], { fontSize: 14 }]}>
              {formData.prefix}
              <Text style={[text.blue[400]]}>{"{username}"}</Text>
              {formData.suffix}
            </Text>
          </View>
        </View>
        {/* Active toggle */}
        <View
          style={[
            layout.flex.row,
            layout.flex.alignCenter,
            layout.flex.spaceBetween,
            mb[6],
          ]}
        >
          <Text style={[text.gray[300], { fontSize: 14, fontWeight: "500" }]}>
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
        <Button variant="secondary" onPress={onClose} disabled={isLoading}>
          <Text>{t("cancel")}</Text>
        </Button>
        <Button onPress={handleSubmit} disabled={isLoading}>
          <Text>
            {isLoading ? t("saving") : webhook ? t("update") : t("create")}
          </Text>
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
      const response = await agent.place.stream.server.listWebhooks({
        limit: 50,
      });
      // if not type "livestream" | "chat" | "follow" | "mention"[] just return
      // todo: find a better way to check this
      if (response.data.webhooks) {
        for (const webhook of response.data.webhooks) {
          webhook.events = (webhook.events as string[]).filter((event) =>
            ["livestream", "chat", "follow", "mention"].includes(event),
          ) as ("livestream" | "chat" | "follow" | "mention")[];
        }
      }
      setWebhooks((response.data.webhooks as any) || []);
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

      await agent.place.stream.server.createWebhook({
        name: data.name || undefined,
        url: data.url,
        events: data.events as ("livestream" | "chat" | "follow" | "mention")[],
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

      await agent.place.stream.server.updateWebhook({
        id: editingWebhook.id,
        name: data.name || undefined,
        url: data.url,
        events: data.events as ("livestream" | "chat" | "follow" | "mention")[],
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
      await agent.place.stream.server.deleteWebhook({ id });
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

  return (
    <>
      <ScrollView>
        <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
          <View style={{ maxWidth: 800, width: "100%" }}>
            {/* Header */}
            <MenuContainer>
              <View>
                <Text size="xl">{t("webhook-integrations")}</Text>
                <Text size="lg" style={[text.gray[400], { marginTop: 4 }]}>
                  {t("webhook-integrations-description")}
                </Text>

                <View
                  style={[
                    layout.flex.row,
                    layout.flex.justify.start,
                    gap.all[3],
                    w.percent[100],
                    mt[2],
                  ]}
                >
                  <Button
                    onPress={handleCreate}
                    size="pill"
                    width="min"
                    leftIcon={<Plus color={theme.colors.text} />}
                  >
                    <Text>{t("create-webhook")}</Text>
                  </Button>

                  <Button
                    onPress={loadWebhooks}
                    disabled={loading}
                    leftIcon={<RefreshCw color={theme.colors.text} />}
                    size="pill"
                    width="min"
                    variant="secondary"
                  >
                    <Text>{t("refresh")}</Text>
                  </Button>
                </View>
              </View>
            </MenuContainer>

            {/* Content */}
            {loading ? (
              <Loading />
            ) : webhooks === null ? (
              <View style={[layout.flex.center, mt[8]]}>
                <Text style={[text.gray[600]]}>
                  {t("failed-load-webhooks")}
                </Text>
              </View>
            ) : webhooks.length === 0 ? (
              <View style={[layout.flex.center, mt[8]]}>
                <Text style={[text.gray[600], mb[4], { fontSize: 16 }]}>
                  {t("no-webhooks-yet")}
                </Text>
                <Text
                  style={[
                    text.gray[500],
                    mb[6],
                    { fontSize: 14, textAlign: "center" },
                  ]}
                >
                  {t("create-first-webhook-description")}
                </Text>
                <AQLink to={{ screen: "LiveDashboard" }}>
                  <Text style={[text.blue[600], { fontSize: 14 }]}>
                    {t("need-setup-live-dashboard")}
                  </Text>
                </AQLink>
              </View>
            ) : (
              <MenuContainer>
                <MenuGroup>
                  <VirtualizedList
                    data={webhooks}
                    getItemCount={(data) => data.length}
                    getItem={(data, index) => data[index]}
                    keyExtractor={(item) => item.id}
                    ItemSeparatorComponent={MenuSeparator}
                    renderItem={(ri) => {
                      let webhook = ri.item;
                      return (
                        <WebhookRow
                          webhook={webhook}
                          onEdit={handleEdit}
                          onDelete={deleteWebhook}
                          isDeleting={deletingWebhooks.has(webhook.id)}
                        />
                      );
                    }}
                  />
                </MenuGroup>
              </MenuContainer>
            )}
          </View>
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
        <View style={[w.percent[100], mb[8], mt[2]]}>
          <Text style={[{ fontSize: 24 }]}>
            {t("confirm-delete", {
              name: deleteDialog.webhook?.name || t("untitled-webhook"),
            })}
          </Text>
          <Text
            style={[text.gray[400], mt[4], { fontSize: 18, fontWeight: "700" }]}
          >
            {t("action-cannot-be-undone")}
          </Text>
          <Text style={[text.gray[400], { fontSize: 18, fontWeight: "700" }]}>
            {t("webhook-will-no-longer-receive-events")}
          </Text>
        </View>

        <View style={[layout.flex.row, layout.flex.justify.end, gap.all[3]]}>
          <Button
            variant="secondary"
            width="full"
            onPress={() => setDeleteDialog({ isVisible: false, webhook: null })}
            disabled={
              deleteDialog.webhook
                ? deletingWebhooks.has(deleteDialog.webhook.id)
                : false
            }
          >
            <Text>{t("cancel")}</Text>
          </Button>
          <Button
            variant="destructive"
            width="full"
            onPress={confirmDelete}
            disabled={
              deleteDialog.webhook
                ? deletingWebhooks.has(deleteDialog.webhook.id)
                : false
            }
          >
            <Text style={[text.white, { fontSize: 14, fontWeight: "500" }]}>
              {deleteDialog.webhook &&
              deletingWebhooks.has(deleteDialog.webhook.id)
                ? t("deleting")
                : t("delete")}
            </Text>
          </Button>
        </View>
      </Dialog>
    </>
  );
}
