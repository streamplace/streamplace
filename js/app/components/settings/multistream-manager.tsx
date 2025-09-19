import {
  Button,
  Dialog,
  DialogFooter,
  Input,
  Text,
} from "@streamplace/components";
import { ThemeProvider } from "@streamplace/components/src/lib/theme/theme";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import {
  bg,
  borders,
  flex,
  gap,
  h,
  layout,
  mb,
  mt,
  mx,
  p,
  pt,
  r,
  text,
  w,
} from "@streamplace/components/src/ui";
import { Edit3, Plus, RefreshCw, Trash2 } from "@tamagui/lucide-icons";
import Loading from "components/loading/loading";
import { useEffect, useState } from "react";
import { Alert, Pressable, ScrollView, Switch, View } from "react-native";
import {
  PlaceStreamMultistreamDefs,
  PlaceStreamMultistreamTarget,
} from "streamplace";
import { timeAgo } from "utils/timeAgo";

interface MultistreamTargetViewHydrated
  extends PlaceStreamMultistreamDefs.TargetView {
  record: PlaceStreamMultistreamTarget.Record;
}

const mulistreamTitle = (target?: MultistreamTargetViewHydrated) => {
  if (!target) {
    return "Untitled Target";
  }
  if (target.record.name) {
    return target.record.name;
  }
  if (target.record.url) {
    try {
      const u = new URL(target.record.url);
      return u.host;
    } catch (error) {
      return "Untitled Target";
    }
  }
  return "Untitled Target";
};

export default function MultistreamManager() {
  const agent = usePDSAgent();
  const [loading, setLoading] = useState(true);
  const [targets, setTargets] = useState<
    MultistreamTargetViewHydrated[] | null
  >(null);
  const [editingTarget, setEditingTarget] = useState<
    MultistreamTargetViewHydrated | undefined
  >(undefined);
  const [showForm, setShowForm] = useState(false);
  const [formLoading, setFormLoading] = useState(false);
  const [deleteDialog, setDeleteDialog] = useState<{
    isVisible: boolean;
    target: MultistreamTargetViewHydrated | null;
    isLoading: boolean;
  }>({ isVisible: false, target: null, isLoading: false });
  const [deletingTargets, setDeletingTargets] = useState<Set<string>>(
    new Set(),
  );
  const [formError, setFormError] = useState<string>("");

  const loadMultistreamTargets = async () => {
    if (!agent) return;

    try {
      setLoading(true);
      const targetViews = await agent.place.stream.multistream.listTargets({
        limit: 50,
      });
      setTargets(targetViews.data.targets as MultistreamTargetViewHydrated[]);
    } catch (error) {
      console.error("Failed to load multistream targets:", error);
      Alert.alert(
        "Error",
        "Failed to load multistream targets. Please try again.",
      );
    } finally {
      setLoading(false);
    }
  };

  const createMultistreamTarget = async (
    record: PlaceStreamMultistreamTarget.Record,
  ) => {
    if (!agent) return;
    try {
      setFormError("");
      setFormLoading(true);
      await agent.place.stream.multistream.createTarget({
        multistreamTarget: {
          ...record,
          createdAt: new Date().toISOString(),
        },
      });
      setShowForm(false);
      await loadMultistreamTargets();
      setFormLoading(false);
    } catch (error) {
      setFormError(error.message);
    } finally {
      setFormLoading(false);
    }
  };

  const editMultistreamTarget = async (
    uri: string,
    record: PlaceStreamMultistreamTarget.Record,
  ) => {
    if (!agent) return;
    try {
      setFormError("");
      setFormLoading(true);
      await agent.place.stream.multistream.putTarget({
        multistreamTarget: record,
        rkey: uri.split("/").pop() || "",
      });
      setShowForm(false);
      await loadMultistreamTargets();
      setFormLoading(false);
    } catch (error) {
      console.error("Failed to edit multistream target:", error);
      setFormError(error.message);
    } finally {
      setFormLoading(false);
    }
  };

  const deleteMultistreamTarget = async (uri: string) => {
    if (!agent) return;
    try {
      setFormError("");
      setDeletingTargets((prev) => new Set(prev).add(uri));
      await agent.place.stream.multistream.deleteTarget({
        rkey: uri.split("/").pop() || "",
      });
      setShowForm(false);
      await loadMultistreamTargets();
      setDeleteDialog({ isVisible: false, target: null, isLoading: false });
    } catch (error) {
      console.error("Failed to delete multistream target:", error);
      setFormError(error.message);
    } finally {
      setDeletingTargets((prev) => {
        const newSet = new Set(prev);
        newSet.delete(uri);
        return newSet;
      });
    }
  };

  useEffect(() => {
    loadMultistreamTargets();
  }, [agent]);

  const handleEdit = (target: MultistreamTargetViewHydrated) => {
    setEditingTarget(target);
    setShowForm(true);
  };

  const handleCreate = () => {
    setEditingTarget(undefined);
    setShowForm(true);
  };

  return (
    <ThemeProvider>
      <View style={[flex.values[1]]}>
        <ScrollView style={[flex.values[1]]}>
          <View style={[{ maxWidth: 800 }, mx.auto]}>
            {/* Header */}
            <View style={[mb[6]]}>
              <Text style={[mb[2], { fontSize: 24, fontWeight: "700" }]}>
                Multistream Targets
              </Text>
              <Text style={[text.gray[400], mb[4], { fontSize: 14 }]}>
                Automatically push your Streamplace livestreams to other
                streaming services like Twitch or YouTube.
              </Text>
              <View style={[layout.flex.row, gap.all[3]]}>
                <Button onPress={handleCreate} size="sm" leftIcon={<Plus />}>
                  <Text>Create Multistream Target</Text>
                </Button>

                <Button
                  onPress={loadMultistreamTargets}
                  disabled={loading}
                  leftIcon={<RefreshCw />}
                  size="sm"
                >
                  <Text>Refresh</Text>
                </Button>
              </View>
            </View>
          </View>

          {/* Content */}
          {loading && !targets ? (
            <Loading />
          ) : targets === null ? (
            <View style={[layout.flex.center, mt[8]]}>
              <Text style={[text.gray[600]]}>
                Failed to load multistream targets
              </Text>
            </View>
          ) : targets.length === 0 ? (
            <View style={[layout.flex.center, mt[8]]}>
              <Text style={[text.gray[600], mb[4], { fontSize: 16 }]}>
                No targets yet!
              </Text>
            </View>
          ) : (
            <>
              <View style={[mb[4]]}>
                <Text style={[text.gray[600], { fontSize: 14 }]}>
                  {targets.length} target{targets.length !== 1 && "s"}
                </Text>
              </View>
              {targets.map((target) => (
                <MultistreamRow
                  key={target.uri}
                  target={target}
                  onEdit={handleEdit}
                  onDelete={() =>
                    setDeleteDialog({
                      isVisible: true,
                      target,
                      isLoading: false,
                    })
                  }
                  isDeleting={false}
                  // onEdit={handleEdit}
                  // isDeleting={deletingWebhooks.has(webhook.id)}
                />
              ))}
            </>
          )}
        </ScrollView>
        <MultistreamTargetForm
          target={editingTarget}
          isVisible={showForm}
          onClose={() => {
            setShowForm(false);
          }}
          onSubmit={(record: PlaceStreamMultistreamTarget.Record) => {
            if (editingTarget) {
              editMultistreamTarget(editingTarget.uri, record);
            } else {
              createMultistreamTarget(record);
            }
          }}
          isLoading={formLoading}
          formError={formError}
        />

        <MultistreamTargetDeleteDialog
          target={deleteDialog.target || undefined}
          isVisible={deleteDialog.isVisible}
          onClose={() =>
            setDeleteDialog({
              isVisible: false,
              target: null,
              isLoading: false,
            })
          }
          onSubmit={() =>
            deleteDialog.target &&
            deleteMultistreamTarget(deleteDialog.target.uri)
          }
          isLoading={deleteDialog.isLoading}
          formError={formError}
        />
      </View>
    </ThemeProvider>
  );
}

export function MultistreamRow({
  target,
  onEdit,
  onDelete,
  isDeleting,
}: {
  target: MultistreamTargetViewHydrated;
  onEdit: (target: MultistreamTargetViewHydrated) => void;
  onDelete: (uri: string) => void;
  isDeleting: boolean;
}) {
  return (
    <View
      style={[
        flex.shrink[1],
        borders.width.thin,
        borders.color.gray[200],
        bg.neutral[800],
        r.xl,
        p[4],
        mb[3],
        layout.flex.column,
        gap.all[3],
        { opacity: isDeleting ? 0.5 : target.record.active ? 1 : 0.7 },
      ]}
    >
      {/* Header */}
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
        ]}
      >
        <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
          <View
            style={[
              w[3],
              h[3],
              r.full,
              { backgroundColor: target.record.active ? "#22c55e" : "#6b7280" },
            ]}
          />
          <Text style={[{ fontSize: 16, fontWeight: "600" }]}>
            {mulistreamTitle(target)}
          </Text>
        </View>

        <View style={[layout.flex.row, gap.all[2]]}>
          <Pressable
            style={[
              bg.gray[100],
              p[2],
              r.md,
              layout.flex.center,
              { minWidth: 32, minHeight: 32 },
            ]}
            onPress={() => onEdit(target)}
            disabled={isDeleting}
          >
            <Edit3 size={16} color="#374151" />
          </Pressable>

          <Pressable
            style={[
              bg.red[800],
              p[2],
              r.md,
              layout.flex.center,
              { minWidth: 32, minHeight: 32 },
            ]}
            onPress={() => onDelete(target.uri)}
            disabled={isDeleting}
          >
            <Trash2 size={16} />
          </Pressable>
        </View>
      </View>

      {/* URL */}
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
        <Text style={[text.gray[300], { fontSize: 12 }]}>URL:</Text>
        <Text
          style={[text.gray[400], { fontSize: 12, fontFamily: "monospace" }]}
          numberOfLines={1}
        >
          {target.record.url.length > 50
            ? target.record.url.slice(0, 45) +
              "..." +
              target.record.url.slice(target.record.url.length - 5)
            : target.record.url}
        </Text>
      </View>

      {/* Status info */}
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          pt[2],
          borders.top.width.thin,
          borders.top.color.gray[100],
        ]}
      >
        <Text style={[text.gray[400], { fontSize: 11 }]}>
          Created {timeAgo(new Date(target.record.createdAt))}
        </Text>
      </View>
    </View>
  );
}

function MultistreamTargetForm({
  target,
  isVisible,
  onClose,
  onSubmit,
  isLoading,
  formError,
}: {
  target?: MultistreamTargetViewHydrated;
  isVisible: boolean;
  onClose: () => void;
  onSubmit: (record: PlaceStreamMultistreamTarget.Record) => void;
  isLoading: boolean;
  formError: string;
}) {
  const [formData, setFormData] = useState<PlaceStreamMultistreamTarget.Record>(
    {
      $type: "place.stream.multistream.target",
      name: target?.record.name || "",
      url: target?.record.url || "",
      active: target?.record.active ?? true,
      createdAt: target?.record.createdAt || "",
    },
  );

  const [errors, setErrors] = useState<Record<string, string>>({});

  // Update form data when webhook prop changes (for editing)
  useEffect(() => {
    setErrors({});
    if (target) {
      setFormData({
        $type: "place.stream.multistream.target",
        name: target.record.name || "",
        url: target.record.url || "",
        active: target.record.active ?? true,
        createdAt: target.record.createdAt || "",
      });
    } else {
      // Reset form for new webhook
      setFormData({
        $type: "place.stream.multistream.target",
        name: "",
        url: "",
        active: true,
        createdAt: "",
      });
    }
  }, [target, isVisible]);

  const validateForm = () => {
    const newErrors: Record<string, string> = {};

    if (!formData.url.trim()) {
      newErrors.url = "URL is required";
    } else if (!formData.url.match(/^rtmps?:\/\/.+/)) {
      newErrors.url = "URL must start with rtmp:// or rtmps://";
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = () => {
    if (validateForm()) {
      onSubmit(formData);
    }
  };

  return (
    <Dialog
      open={isVisible}
      onOpenChange={(open) => !open && onClose()}
      title={target ? "Edit Target" : "Create Target"}
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
            placeholder="My Multistream Target"
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
              setFormData((prev) => ({
                ...prev,
                url: text.trim().replaceAll(/\n/g, ""),
              }))
            }
            placeholder="rtmps://example.com:443/live/foo"
            multiline
          />
          <Text style={[text.red[600], mt[1], { fontSize: 12 }]}>
            &nbsp;{errors.url}
          </Text>
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
        <Text style={[text.red[600], mt[1], { fontSize: 12 }]}>
          &nbsp;{formError}
        </Text>
      </View>

      <DialogFooter>
        <Button variant="secondary" onPress={onClose} disabled={isLoading}>
          <Text>Cancel</Text>
        </Button>
        <Button onPress={handleSubmit} disabled={isLoading}>
          <Text>{isLoading ? "Saving..." : target ? "Update" : "Create"}</Text>
        </Button>
      </DialogFooter>
    </Dialog>
  );
}

const MultistreamTargetDeleteDialog = ({
  target,
  isVisible,
  onClose,
  onSubmit,
  isLoading,
  formError,
}: {
  target?: MultistreamTargetViewHydrated;
  isVisible: boolean;
  onClose: () => void;
  onSubmit: () => void;
  isLoading: boolean;
  formError: string;
}) => {
  return (
    <Dialog
      open={isVisible}
      onOpenChange={(open) => !open && onClose()}
      title="Delete Webhook"
      dismissible={false}
    >
      <View style={[w.percent[100], mb[8], mt[2]]}>
        <Text style={[{ fontSize: 24 }]}>
          Are you sure you want to delete "{mulistreamTitle(target)}"?
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

      <Text style={[text.red[600], mt[1], { fontSize: 12 }]}>
        &nbsp;{formError}
      </Text>

      <View style={[layout.flex.row, layout.flex.justify.end, gap.all[3]]}>
        <Button
          variant="secondary"
          onPress={() => onClose()}
          disabled={isLoading}
        >
          <Text>Cancel</Text>
        </Button>
        <Button variant="destructive" onPress={onSubmit} disabled={isLoading}>
          <Text style={[text.white, { fontSize: 14, fontWeight: "500" }]}>
            {isLoading ? "Deleting..." : "Delete"}
          </Text>
        </Button>
      </View>
    </Dialog>
  );
};
