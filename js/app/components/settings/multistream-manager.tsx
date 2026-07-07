import {
  Button,
  Dialog,
  DialogFooter,
  Input,
  Text,
  TFunction,
  useTranslation,
  zero,
} from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { gap, layout, mb, mt, text, w } from "@streamplace/components/src/ui";
import Loading from "components/loading/loading";
import { Plus, RefreshCw } from "lucide-react-native";
import { useEffect, useState } from "react";
import { Alert, ScrollView, Switch, View } from "react-native";
import { place } from "streamplace";
import { timeAgo } from "utils/timeAgo";
import { SettingsListItem } from "./settings-list-item";

interface MultistreamTargetViewHydrated
  extends place.stream.multistream.defs.TargetView {
  record: place.stream.multistream.target.Main;
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
      const targetViews = await agent.client.call(
        place.stream.multistream.listTargets,
        { limit: 50 },
      );
      setTargets(targetViews.targets as MultistreamTargetViewHydrated[]);
    } catch (error) {
      console.error("Failed to load multistream targets:", error);
      Alert.alert("Error", t("failed-load-multistream-targets"));
    } finally {
      setLoading(false);
    }
  };

  const createMultistreamTarget = async (
    record: place.stream.multistream.target.Main,
  ) => {
    if (!agent) return;
    try {
      setFormError("");
      setFormLoading(true);
      await agent.client.call(place.stream.multistream.createTarget, {
        multistreamTarget: {
          ...record,
          createdAt: new Date().toISOString() as any,
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
    record: place.stream.multistream.target.Main,
  ) => {
    if (!agent) return;
    try {
      setFormError("");
      setFormLoading(true);
      await agent.client.call(place.stream.multistream.putTarget, {
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
      await agent.client.call(place.stream.multistream.putTarget, {
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
      await agent.client.call(place.stream.multistream.deleteTarget, {
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
      <ScrollView>
        <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
          <View style={{ maxWidth: 800, width: "100%" }}>
            {/* Header */}
            <View style={[mb[6]]}>
              <Text style={[mb[2], { fontSize: 24, fontWeight: "700" }]}>
                {t("multistream-targets")}
              </Text>
              <Text style={[text.gray[400], mb[4], { fontSize: 14 }]}>
                {t("multistream-description")}
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
                  <Text>{t("create-multistream-target")}</Text>
                </Button>

                <Button
                  onPress={loadMultistreamTargets}
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
          </View>

          {/* Content */}
          {loading && !targets ? (
            <Loading />
          ) : targets === null ? (
            <View style={[layout.flex.center, mt[8]]}>
              <Text style={[text.gray[600]]}>
                {t("failed-load-multistream-targets")}
              </Text>
            </View>
          ) : targets.length === 0 ? (
            <View style={[layout.flex.center, mt[8]]}>
              <Text style={[text.gray[600], mb[4], { fontSize: 16 }]}>
                {t("no-multistream-targets-yet")}
              </Text>
            </View>
          ) : (
            <>
              <View style={[mb[4]]}>
                <Text style={[text.gray[600], { fontSize: 14 }]}>
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
        onSubmit={(record: place.stream.multistream.target.Main) => {
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
  // Determine latest event status for footer
  const getStatusInfo = () => {
    if (target.latestEvent) {
      return (
        <View style={[layout.flex.row, gap.all[4]]}>
          <Text style={[text.gray[400], { fontSize: 11 }]}>
            {t("status")}: {target.latestEvent.status}
          </Text>
          <Text style={[text.gray[400], { fontSize: 11 }]}>
            {timeAgo(new Date(target.latestEvent.createdAt))}
          </Text>
        </View>
      );
    }
    return null;
  };

  return (
    <SettingsListItem
      title={multistreamTitle(target, t)}
      url={redactMultistreamTargetURL(target.record.url)}
      active={target.record.active}
      isDeleting={isDeleting}
      isToggling={isToggling}
      footer={{
        left: `${t("created")} ${timeAgo(new Date(target.record.createdAt))}`,
        right: getStatusInfo(),
      }}
      onEdit={() => onEdit(target)}
      onDelete={() => onDelete(target.uri)}
      onToggle={(active) => onToggle(target, active)}
    />
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
  onSubmit: (record: place.stream.multistream.target.Main) => void;
  isLoading: boolean;
  formError: string;
}) {
  const { t } = useTranslation("settings");
  const [formData, setFormData] =
    useState<place.stream.multistream.target.Main>({
      $type: "place.stream.multistream.target",
      name: target?.record.name || "",
      url: target?.record.url || "",
      active: target?.record.active ?? true,
      createdAt: target?.record.createdAt || "",
    } as any);

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
      } as any);
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

  let displayUrl: string = formData.url;
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
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
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
            style={[text.gray[300], mb[2], { fontSize: 14, fontWeight: "500" }]}
          >
            {t("rtmp-target-url")} *
          </Text>
          <Input
            value={displayUrl}
            onChangeText={(text) => {
              setChangedTargetUrl(true);
              setFormData(
                (prev) =>
                  ({
                    ...prev,
                    url: text.trim().replaceAll(/\n/g, ""),
                  }) as any,
              );
            }}
            placeholder={
              target
                ? redactMultistreamTargetURL(target.record.url)
                : "rtmps://example.com:443/live/foo"
            }
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
            {t("active")}
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
        <Button
          variant="secondary"
          onPress={onClose}
          disabled={isLoading}
          width="min"
        >
          <Text>Cancel</Text>
        </Button>
        <Button onPress={handleSubmit} disabled={isLoading} width="min">
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
  const { t } = useTranslation("settings");
  return (
    <Dialog
      open={isVisible}
      onOpenChange={(open) => !open && onClose()}
      title="Delete Webhook"
      dismissible={false}
    >
      <View style={[w.percent[100], mb[8], mt[2]]}>
        <Text style={[{ fontSize: 24 }]}>
          {t("multistream-delete-target-confirmation", {
            target: multistreamTitle(target, t),
          })}
        </Text>
        <Text
          style={[text.gray[400], mt[4], { fontSize: 18, fontWeight: "700" }]}
        >
          {t("this-action-cannot-be-undone")}
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
            {isLoading ? t("deleting") : t("delete")}
          </Text>
        </Button>
      </View>
    </Dialog>
  );
};
