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
  flex,
  gap,
  layout,
  mb,
  mt,
  mx,
  text,
  w,
} from "@streamplace/components/src/ui";
import Loading from "components/loading/loading";
import { Plus, RefreshCw } from "lucide-react-native";
import { useEffect, useState } from "react";
import { Alert, ScrollView, Switch, View } from "react-native";
import {
  PlaceStreamMultistreamDefs,
  PlaceStreamMultistreamTarget,
} from "streamplace";
import { timeAgo } from "utils/timeAgo";
import { SettingsListItem } from "./settings-list-item";

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
      Alert.alert(
        "Error",
        "Failed to toggle multistream target. Please try again.",
      );
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
                  onToggle={toggleMultistreamTarget}
                  isDeleting={deletingTargets.has(target.uri)}
                  isToggling={togglingTargets.has(target.uri)}
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
  // Determine latest event status for footer
  const getStatusInfo = () => {
    if (target.latestEvent) {
      return (
        <View style={[layout.flex.row, gap.all[4]]}>
          <Text style={[text.gray[400], { fontSize: 11 }]}>
            Status: {target.latestEvent.status}
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
      title={mulistreamTitle(target)}
      url={target.record.url}
      active={target.record.active}
      isDeleting={isDeleting}
      isToggling={isToggling}
      footer={{
        left: `Created ${timeAgo(new Date(target.record.createdAt))}`,
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
