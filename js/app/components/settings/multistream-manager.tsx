import {
  Button,
  Dialog,
  DialogFooter,
  IconButton,
  Input,
  Switch,
  Text,
  TFunction,
  useTranslation,
  zero,
} from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { EmptyState, EmptyStateTile } from "components/empty-state";
import Loading from "components/loading/loading";
import { Edit3, Plus, Radio, RefreshCw, Trash2 } from "lucide-react-native";
import { useEffect, useState } from "react";
import { Alert, ScrollView, View } from "react-native";
import {
  PlaceStreamMultistreamDefs,
  PlaceStreamMultistreamTarget,
} from "streamplace";
import { timeAgo } from "utils/timeAgo";
import { SettingsViewHeader } from "./components/settings-view-header";

const { layout, gap, mb, mt, w, h, p, r, px, py, pt, borders, flex } = zero;

interface MultistreamTargetViewHydrated
  extends PlaceStreamMultistreamDefs.TargetView {
  record: PlaceStreamMultistreamTarget.Record;
}

const redactMultistreamTargetURL = (url: string) => {
  try {
    const u = new URL(url);
    return `${u.protocol}//${u.host}/redacted`;
  } catch (error) {
    return "parsing failed";
  }
};

const multistreamTitle = (
  target: MultistreamTargetViewHydrated | undefined,
  t: TFunction,
) => {
  if (!target) {
    return t("untitled-multistream-target");
  }
  if (target.record.name) {
    return target.record.name;
  }
  if (target.record.url) {
    try {
      const u = new URL(target.record.url);
      return u.host;
    } catch (error) {
      return t("untitled-multistream-target");
    }
  }
  return t("untitled-multistream-target");
};

export default function MultistreamManager() {
  const { theme } = zero.useTheme();
  const { t } = useTranslation("settings");
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
  const [togglingTargets, setTogglingTargets] = useState<Set<string>>(
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
      Alert.alert("Error", t("failed-load-multistream-targets"));
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

  const toggleMultistreamTarget = async (
    target: MultistreamTargetViewHydrated,
    newActiveState: boolean,
  ) => {
    if (!agent) return;
    try {
      setTogglingTargets((prev) => new Set(prev).add(target.uri));
      await agent.place.stream.multistream.putTarget({
        multistreamTarget: {
          ...target.record,
          active: newActiveState,
        },
        rkey: target.uri.split("/").pop() || "",
      });
      await loadMultistreamTargets();
    } catch (error) {
      console.error("Failed to toggle multistream target:", error);
      Alert.alert("Error", t("failed-toggle-multistream-target"));
    } finally {
      setTogglingTargets((prev) => {
        const newSet = new Set(prev);
        newSet.delete(target.uri);
        return newSet;
      });
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
    <>
      <ScrollView contentContainerStyle={{ flexGrow: 1, alignItems: "center" }}>
        <View
          style={[
            { maxWidth: 800, width: "100%" },
            flex.values[1],
            layout.flex.column,
            zero.px[2],
            zero.py[2],
          ]}
        >
            {/* Header */}
            <SettingsViewHeader
              title={t("multistream-targets")}
              description={t("multistream-description")}
              action={
                <View style={{ flexDirection: "row", gap: 8 }}>
                  <Button
                    size="sm"
                    width="min"
                    variant="secondary"
                    leftIcon={<RefreshCw size={16} />}
                    onPress={loadMultistreamTargets}
                    disabled={loading}
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
            {loading && !targets ? (
              <Loading />
            ) : targets === null ? (
              <View style={[layout.flex.center, mt[8]]}>
                <Text style={{ color: theme.colors.text3 }}>
                  {t("failed-load-multistream-targets")}
                </Text>
              </View>
            ) : targets.length === 0 ? (
              <EmptyState
                illustration={<EmptyStateTile icon={Radio} />}
                title="No multistream targets yet"
                subtitle="Add a destination to restream to Twitch, YouTube, and more."
                action={
                  <Button
                    size="sm"
                    leftIcon={<Plus size={16} />}
                    onPress={handleCreate}
                  >
                    {t("create-multistream-target")}
                  </Button>
                }
              />
            ) : (
              <>
                <View style={[mb[4]]}>
                  <Text size="sm" style={{ color: theme.colors.text3 }}>
                    {t("multistream-targets-count", { count: targets.length })}
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
                    onToggle={toggleMultistreamTarget}
                    isDeleting={deletingTargets.has(target.uri)}
                    isToggling={togglingTargets.has(target.uri)}
                  />
                ))}
              </>
            )}
          </View>
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
    </>
  );
}

export function MultistreamRow({
  target,
  onEdit,
  onDelete,
  onToggle,
  isDeleting,
  isToggling,
}: {
  target: MultistreamTargetViewHydrated;
  onEdit: (target: MultistreamTargetViewHydrated) => void;
  onDelete: (uri: string) => void;
  onToggle: (target: MultistreamTargetViewHydrated, active: boolean) => void;
  isDeleting: boolean;
  isToggling: boolean;
}) {
  const { t } = useTranslation("settings");
  const { theme } = zero.useTheme();
  const active = target.record.active;

  // Determine latest event status for footer
  const getStatusInfo = () => {
    if (target.latestEvent) {
      return (
        <View style={[layout.flex.row, gap.all[3]]}>
          <Text size="xs" style={{ color: theme.colors.text3 }}>
            {t("status")}: {target.latestEvent.status}
          </Text>
          <Text size="xs" style={{ color: theme.colors.text3 }}>
            {timeAgo(new Date(target.latestEvent.createdAt))}
          </Text>
        </View>
      );
    }
    return null;
  };

  return (
    <View
      style={[
        borders.width.thin,
        { borderColor: theme.colors.borderSubtle },
        { backgroundColor: theme.colors.surface1 },
        r.lg,
        p[4],
        mb[3],
        { opacity: isDeleting ? 0.5 : 1 },
      ]}
    >
      {/* Top: name / URL + actions */}
      <View
        style={[layout.flex.row, layout.flex.spaceBetween, gap.all[3]]}
      >
        <View style={[flex.values[1], gap.all[1]]}>
          {/* Name + status pill */}
          <View
            style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
          >
            <Text size="lg" weight="semibold" numberOfLines={1}>
              {multistreamTitle(target, t)}
            </Text>
            <View
              style={[
                layout.flex.row,
                layout.flex.alignCenter,
                gap.all[1],
                px[2],
                py[1],
                r.full,
                { backgroundColor: theme.colors.surface2 },
              ]}
            >
              <View
                style={[
                  w[2],
                  h[2],
                  r.full,
                  {
                    backgroundColor: active
                      ? theme.colors.primary
                      : theme.colors.text3,
                  },
                ]}
              />
              <Text
                size="xs"
                style={{
                  color: active ? theme.colors.primary : theme.colors.text3,
                }}
              >
                {active ? t("active") : t("inactive")}
              </Text>
            </View>
          </View>

          {/* URL */}
          <View
            style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
          >
            <Text size="sm" color="muted">
              URL:
            </Text>
            <Text
              size="sm"
              style={{ color: theme.colors.text3 }}
              numberOfLines={1}
              ellipsizeMode="middle"
            >
              {redactMultistreamTargetURL(target.record.url)}
            </Text>
          </View>
        </View>

        {/* Actions */}
        <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[1]]}>
          <Switch
            value={active}
            onValueChange={(next) => onToggle(target, next)}
            disabled={isDeleting || isToggling}
          />
          <IconButton
            size="sm"
            variant="ghost"
            accessibilityLabel="Edit multistream target"
            onPress={() => onEdit(target)}
            disabled={isDeleting}
          >
            <Edit3 size={18} color={theme.colors.text2} />
          </IconButton>
          <IconButton
            size="sm"
            variant="ghost"
            accessibilityLabel="Delete multistream target"
            onPress={() => onDelete(target.uri)}
            disabled={isDeleting}
          >
            <Trash2 size={18} color={theme.colors.danger} />
          </IconButton>
        </View>
      </View>

      {/* Footer */}
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          pt[3],
          mt[3],
          borders.top.width.thin,
          { borderTopColor: theme.colors.borderSubtle },
        ]}
      >
        <Text size="xs" style={{ color: theme.colors.text3 }}>
          {t("created")} {timeAgo(new Date(target.record.createdAt))}
        </Text>
        {getStatusInfo()}
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
  const { t } = useTranslation("settings");
  const { theme } = zero.useTheme();
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
  const [changedTargetUrl, setChangedTargetUrl] = useState(false);

  // Update form data when webhook prop changes (for editing)
  useEffect(() => {
    setErrors({});
    setChangedTargetUrl(false);
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

  let displayUrl = formData.url;
  if (target && !changedTargetUrl) {
    displayUrl = "";
  }

  return (
    <Dialog
      open={isVisible}
      onOpenChange={(open) => !open && onClose()}
      title={
        target ? t("multistream-edit-target") : t("multistream-create-target")
      }
      size="lg"
      dismissible={false}
    >
      <View style={[w.percent[100]]}>
        {/* Name */}
        <View style={[mb[4]]}>
          <Text
            size="sm"
            weight="medium"
            style={[{ color: theme.colors.text2 }, mb[2]]}
          >
            {t("rtmp-target-name")} ({t("optional")})
          </Text>
          <Input
            value={formData.name}
            onChangeText={(text) =>
              setFormData((prev) => ({ ...prev, name: text }))
            }
            placeholder={t("rtmp-target-name-placeholder")}
          />
        </View>

        {/* URL */}
        <View style={[mb[4]]}>
          <Text
            size="sm"
            weight="medium"
            style={[{ color: theme.colors.text2 }, mb[2]]}
          >
            {t("rtmp-target-url")} *
          </Text>
          <Input
            value={displayUrl}
            onChangeText={(text) => {
              setChangedTargetUrl(true);
              setFormData((prev) => ({
                ...prev,
                url: text.trim().replaceAll(/\n/g, ""),
              }));
            }}
            placeholder={
              target
                ? redactMultistreamTargetURL(target.record.url)
                : "rtmps://example.com:443/live/foo"
            }
            multiline
          />
          <Text size="xs" style={[{ color: theme.colors.danger }, mt[1]]}>
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
          <Text
            size="sm"
            weight="medium"
            style={{ color: theme.colors.text2 }}
          >
            {t("active")}
          </Text>
          <Switch
            value={formData.active}
            onValueChange={(active) =>
              setFormData((prev) => ({ ...prev, active }))
            }
          />
        </View>
        <Text size="xs" style={[{ color: theme.colors.danger }, mt[1]]}>
          &nbsp;{formError}
        </Text>
      </View>

      <DialogFooter>
        <Button
          variant="secondary"
          onPress={onClose}
          disabled={isLoading}
          width="min"
        >
          Cancel
        </Button>
        <Button onPress={handleSubmit} disabled={isLoading} width="min">
          {isLoading ? "Saving..." : target ? "Update" : "Create"}
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
  const { t } = useTranslation("settings");
  const { theme } = zero.useTheme();
  return (
    <Dialog
      open={isVisible}
      onOpenChange={(open) => !open && onClose()}
      title="Delete Target"
      dismissible={false}
    >
      <View style={[w.percent[100], mb[8], mt[2]]}>
        <Text size="lg" weight="semibold">
          {t("multistream-delete-target-confirmation", {
            target: multistreamTitle(target, t),
          })}
        </Text>
        <Text
          weight="semibold"
          style={[{ color: theme.colors.text3 }, mt[4], { fontSize: 18 }]}
        >
          {t("this-action-cannot-be-undone")}
        </Text>
      </View>

      <Text size="xs" style={[{ color: theme.colors.danger }, mt[1]]}>
        &nbsp;{formError}
      </Text>

      <DialogFooter>
        <Button
          variant="secondary"
          onPress={() => onClose()}
          disabled={isLoading}
          width="min"
        >
          Cancel
        </Button>
        <Button
          variant="danger"
          onPress={onSubmit}
          disabled={isLoading}
          width="min"
        >
          {isLoading ? t("deleting") : t("delete")}
        </Button>
      </DialogFooter>
    </Dialog>
  );
};
