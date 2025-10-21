import {
  Button,
  Dialog,
  DialogFooter,
  Input,
  Text,
  zero,
} from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import AQLink from "components/aqlink";
import Loading from "components/loading/loading";
import { Plus, RefreshCw, X } from "lucide-react-native";
import { ReactNode, useEffect, useState } from "react";
import { Alert, Pressable, ScrollView, Switch, View } from "react-native";
import { timeAgo } from "utils/timeAgo";
import { SettingsListItem } from "./settings-list-item";

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
  description: string;
}

const EVENT_OPTIONS = [
  { value: "livestream", label: "Livestream Started" },
  { value: "chat", label: "Chat Messages" },
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
  const isDiscord = webhook.url
    .toLowerCase()
    .startsWith("https://discord.com/api/webhooks");

  const badges = isDiscord
    ? [{ text: "Discord", color: "#6366f1", bgColor: "#312e81" }]
    : [];

  const metadata = [
    {
      label: "Events",
      value: webhook.events.map(
        (event) =>
          EVENT_OPTIONS.find((opt) => opt.value === event)?.label || event,
      ),
    },
  ];

  const getFooterRight = () => {
    const items: ReactNode[] = [];
    if (webhook.errorCount !== undefined && webhook.errorCount > 0) {
      items.push(
        <Text key="errors" style={[text.red[600], { fontSize: 11 }]}>
          {webhook.errorCount} errors
        </Text>,
      );
    }
    if (webhook.lastTriggered) {
      items.push(
        <Text key="triggered" style={[text.gray[400], { fontSize: 11 }]}>
          Last triggered {timeAgo(new Date(webhook.lastTriggered))}
        </Text>,
      );
    }
    return items.length > 0 ? <>{items}</> : null;
  };

  return (
    <SettingsListItem
      title={webhook.name || "Untitled Webhook"}
      subtitle={webhook.description}
      url={webhook.url}
      active={webhook.active}
      isDeleting={isDeleting}
      badges={badges}
      metadata={metadata}
      footer={{
        left: `Created ${timeAgo(new Date(webhook.createdAt))}`,
        right: getFooterRight(),
      }}
      onEdit={() => onEdit(webhook)}
      onDelete={() => onDelete(webhook.id)}
    />
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
    description: webhook?.description || "",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

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
    <Dialog
      open={isVisible}
      onOpenChange={(open) => !open && onClose()}
      title={webhook ? "Edit Webhook" : "Create Webhook"}
      size="lg"
      dismissible={false}
    >
      <View style={[w.percent[100]]}>
        {/* Name */}
        <View style={[mb[4]]}>
          <Text
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            Name (optional)
          </Text>
          <Input
            value={formData.name}
            onChangeText={(text) =>
              setFormData((prev) => ({ ...prev, name: text }))
            }
            placeholder="Captain Hook"
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
                {option.label}
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
            <Button size="pill" onPress={addReplacement}>
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
          <Text>Cancel</Text>
        </Button>
        <Button onPress={handleSubmit} disabled={isLoading}>
          <Text>{isLoading ? "Saving..." : webhook ? "Update" : "Create"}</Text>
        </Button>
      </DialogFooter>
    </Dialog>
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
    <View style={[flex.values[1]]}>
      <ScrollView style={[flex.values[1]]}>
        <View style={[{ maxWidth: 800 }, mx.auto]}>
          {/* Header */}
          <View style={[mb[6]]}>
            <Text size="xl">Webhook Integrations</Text>
            <Text size="lg" style={[text.gray[400], mb[4]]}>
              Create webhooks to receive notifications when you go live or get
              chat messages.
            </Text>

            <View
              style={[layout.flex.row, layout.flex.justify.between, gap.all[3]]}
            >
              <Button
                onPress={handleCreate}
                size="pill"
                leftIcon={<Plus color={theme.colors.text} />}
              >
                <Text>Create Webhook</Text>
              </Button>

              <Button
                onPress={loadWebhooks}
                disabled={loading}
                leftIcon={<RefreshCw color={theme.colors.text} />}
                size="pill"
                variant="secondary"
              >
                <Text>Refresh</Text>
              </Button>
            </View>
          </View>

          {/* Content */}
          {loading ? (
            <Loading />
          ) : webhooks === null ? (
            <View style={[layout.flex.center, mt[8]]}>
              <Text style={[text.gray[600]]}>Failed to load webhooks</Text>
            </View>
          ) : webhooks.length === 0 ? (
            <View style={[layout.flex.center, mt[8]]}>
              <Text style={[text.gray[600], mb[4], { fontSize: 16 }]}>
                No webhooks yet!
              </Text>
              <Text
                style={[
                  text.gray[500],
                  mb[6],
                  { fontSize: 14, textAlign: "center" },
                ]}
              >
                Create your first webhook to start receiving notifications when
                you go live.
              </Text>
              <AQLink to={{ screen: "LiveDashboard" }}>
                <Text style={[text.blue[600], { fontSize: 14 }]}>
                  Need to set up streaming first? Visit the Live Dashboard
                </Text>
              </AQLink>
            </View>
          ) : (
            <>
              <View style={[mb[4]]}>
                <Text style={[text.gray[600], { fontSize: 14 }]}>
                  {webhooks.length} webhook{webhooks.length !== 1 && "s"}
                </Text>
              </View>
              {webhooks.map((webhook) => (
                <WebhookRow
                  key={webhook.id}
                  webhook={webhook}
                  onEdit={handleEdit}
                  onDelete={deleteWebhook}
                  isDeleting={deletingWebhooks.has(webhook.id)}
                />
              ))}
            </>
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
        title="Delete Webhook"
        dismissible={false}
      >
        <View style={[w.percent[100], mb[8], mt[2]]}>
          <Text style={[{ fontSize: 24 }]}>
            Are you sure you want to delete "
            {deleteDialog.webhook?.name || "Untitled Webhook"}"?
          </Text>
          <Text
            style={[text.gray[400], mt[4], { fontSize: 18, fontWeight: "700" }]}
          >
            This action cannot be undone.
          </Text>
          <Text style={[text.gray[400], { fontSize: 18, fontWeight: "700" }]}>
            The webhook will no longer receive events.
          </Text>
        </View>

        <View style={[layout.flex.row, layout.flex.justify.end, gap.all[3]]}>
          <Button
            variant="secondary"
            onPress={() => setDeleteDialog({ isVisible: false, webhook: null })}
            disabled={
              deleteDialog.webhook
                ? deletingWebhooks.has(deleteDialog.webhook.id)
                : false
            }
          >
            <Text>Cancel</Text>
          </Button>
          <Button
            variant="destructive"
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
                ? "Deleting..."
                : "Delete"}
            </Text>
          </Button>
        </View>
      </Dialog>
    </View>
  );
}
