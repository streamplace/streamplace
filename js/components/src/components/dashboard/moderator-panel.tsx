import { Check, Shield, Trash2, UserPlus } from "lucide-react-native";
import { useEffect, useMemo, useState } from "react";
import { Pressable, ScrollView, Text, View } from "react-native";
import {
  Button,
  Dialog,
  DialogFooter,
  Input,
  ResponsiveDialog,
  useToast,
} from "../../components/ui";
import { useAvatars } from "../../hooks/useAvatars";
import { atoms } from "../../lib/theme";
import {
  useAddModerator,
  useListModerators,
  useRemoveModerator,
} from "../../streamplace-store/moderator-management";
import { formatHandleWithAt } from "../../utils/format-handle";

const {
  flex,
  bg,
  r,
  borders,
  p,
  text: textStyle,
  layout,
  gap,
  mb,
  my,
  px,
  py,
} = atoms;

interface ModeratorPanelProps {
  isLive?: boolean;
  embedded?: boolean; // If true, removes outer container styling (for use inside other panels)
}

export default function ModeratorPanel({
  isLive,
  embedded = false,
}: ModeratorPanelProps) {
  const { moderators, isLoading, error, refresh } = useListModerators();
  const [showAddDialog, setShowAddDialog] = useState(false);

  // Collect all moderator DIDs for batch fetching profiles
  const moderatorDIDs = useMemo(
    () => moderators.map((mod) => mod.value.moderator),
    [moderators],
  );

  const containerStyle = embedded
    ? [flex.values[1], layout.flex.column]
    : [
        flex.values[1],
        bg.neutral[900],
        r.lg,
        borders.width.thin,
        borders.color.neutral[700],
        layout.flex.column,
      ];

  return (
    <View style={containerStyle}>
      {/* Header */}
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          borders.bottom.width.thin,
          borders.bottom.color.neutral[700],
          p[4],
        ]}
      >
        <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
          <Shield size={18} color="#ffffff" />
          <Text style={[textStyle.white, { fontSize: 18, fontWeight: "600" }]}>
            Moderators
          </Text>
        </View>
        <Pressable
          onPress={() => setShowAddDialog(true)}
          style={[
            layout.flex.row,
            layout.flex.alignCenter,
            gap.all[2],
            bg.blue[600],
            px[3],
            py[2],
            r.md,
          ]}
        >
          <UserPlus size={16} color="#ffffff" />
          <Text style={[textStyle.white, { fontSize: 13, fontWeight: "500" }]}>
            Add
          </Text>
        </Pressable>
      </View>

      {/* Content */}
      <ScrollView style={[flex.values[1], p[4]]}>
        {isLoading && moderators.length === 0 && (
          <Text
            style={[textStyle.gray[400], { fontSize: 14, textAlign: "center" }]}
          >
            Loading moderators...
          </Text>
        )}

        {error && (
          <View
            style={[
              bg.red[900],
              p[3],
              r.md,
              borders.width.thin,
              borders.color.red[700],
            ]}
          >
            <Text style={[textStyle.red[400], { fontSize: 13 }]}>{error}</Text>
          </View>
        )}

        {!isLoading && moderators.length === 0 && !error && (
          <View style={[layout.flex.center, p[6]]}>
            <Shield size={48} color="#6b7280" style={[mb[4]]} />
            <Text
              style={[
                textStyle.gray[400],
                { fontSize: 14, textAlign: "center", marginBottom: 8 },
              ]}
            >
              No moderators yet
            </Text>
            <Text
              style={[
                textStyle.gray[500],
                { fontSize: 12, textAlign: "center" },
              ]}
            >
              Add moderators to help manage your chat
            </Text>
          </View>
        )}

        {moderators.map((mod, index) => (
          <ModeratorCard
            key={mod.rkey}
            moderator={mod}
            onRemove={refresh}
            isLast={index === moderators.length - 1}
            moderatorDIDs={moderatorDIDs}
          />
        ))}
      </ScrollView>

      {/* Add Moderator Dialog */}
      <AddModeratorDialog
        visible={showAddDialog}
        onClose={() => setShowAddDialog(false)}
        onSuccess={() => {
          setShowAddDialog(false);
          refresh();
        }}
      />
    </View>
  );
}

interface ModeratorCardProps {
  moderator: {
    uri: string;
    cid: string;
    value: {
      moderator: string;
      permissions: string[];
      createdAt: string;
      expirationTime?: string;
    };
    rkey: string;
  };
  onRemove: () => void;
  isLast: boolean;
  moderatorDIDs: string[]; // All moderator DIDs for batch fetching
}

function ModeratorCard({
  moderator,
  onRemove,
  isLast,
  moderatorDIDs,
}: ModeratorCardProps) {
  const { removeModerator, isLoading } = useRemoveModerator();
  const [showConfirm, setShowConfirm] = useState(false);
  const toast = useToast();

  // Use useAvatars hook to batch-fetch profiles for all moderators
  const profiles = useAvatars(moderatorDIDs);
  const profile = profiles[moderator.value.moderator];

  // Format display name using existing utility
  const displayName = useMemo(() => {
    if (profile) {
      return formatHandleWithAt(profile);
    }
    return moderator.value.moderator; // Fall back to DID
  }, [profile, moderator.value.moderator]);

  const handleRemove = async () => {
    try {
      await removeModerator(moderator.rkey);
      setShowConfirm(false);
      toast.show(
        "Moderator removed",
        "The moderator has been successfully removed.",
        { duration: 3 },
      );
      onRemove();
    } catch (err) {
      console.error("Failed to remove moderator:", err);
      toast.show(
        "Error removing moderator",
        err instanceof Error ? err.message : "Failed to remove moderator",
        { duration: 5 },
      );
    }
  };

  const isExpired =
    moderator.value.expirationTime &&
    new Date(moderator.value.expirationTime) < new Date();

  return (
    <View
      style={[
        layout.flex.row,
        layout.flex.spaceBetween,
        layout.flex.alignCenter,
        p[3],
        bg.neutral[800],
        r.md,
        !isLast && mb[2],
        borders.width.thin,
        borders.color.neutral[700],
      ]}
    >
      <View style={[flex.values[1]]}>
        <Text
          style={[
            textStyle.white,
            { fontSize: 14, fontWeight: "500", marginBottom: 4 },
          ]}
          numberOfLines={1}
        >
          {displayName}
        </Text>
        <View style={[layout.flex.row, { flexWrap: "wrap" }, gap.all[1]]}>
          {moderator.value.permissions.map((perm) => (
            <View
              key={perm}
              style={[
                bg.blue[900],
                px[2],
                py[1],
                r.sm,
                borders.width.thin,
                borders.color.blue[700],
              ]}
            >
              <Text
                style={[
                  textStyle.blue[300],
                  { fontSize: 11, fontWeight: "500" },
                ]}
              >
                {perm}
              </Text>
            </View>
          ))}
          {isExpired && (
            <View
              style={[
                bg.red[900],
                px[2],
                py[1],
                r.sm,
                borders.width.thin,
                borders.color.red[700],
              ]}
            >
              <Text
                style={[
                  textStyle.red[300],
                  { fontSize: 11, fontWeight: "500" },
                ]}
              >
                Expired
              </Text>
            </View>
          )}
        </View>
        {moderator.value.expirationTime && !isExpired && (
          <Text style={[textStyle.gray[400], { fontSize: 11, marginTop: 4 }]}>
            Expires:{" "}
            {new Date(moderator.value.expirationTime).toLocaleDateString()}
          </Text>
        )}
      </View>
      <Pressable
        onPress={() => setShowConfirm(true)}
        disabled={isLoading}
        style={[
          p[2],
          r.md,
          bg.red[900],
          borders.width.thin,
          borders.color.red[700],
          isLoading && { opacity: 0.5 },
        ]}
      >
        <Trash2 size={16} color="#f87171" />
      </Pressable>

      {/* Confirm Delete Dialog */}
      <Dialog
        open={showConfirm}
        onOpenChange={setShowConfirm}
        title="Remove Moderator?"
        description="This will revoke all moderation permissions for this user."
        dismissible={false}
      >
        <DialogFooter>
          <Button
            width="min"
            variant="secondary"
            onPress={() => setShowConfirm(false)}
            disabled={isLoading}
          >
            <Text>Cancel</Text>
          </Button>
          <Button
            width="min"
            variant="destructive"
            onPress={handleRemove}
            disabled={isLoading}
          >
            <Text>{isLoading ? "Removing..." : "Remove"}</Text>
          </Button>
        </DialogFooter>
      </Dialog>
    </View>
  );
}

interface AddModeratorDialogProps {
  visible: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

function AddModeratorDialog({
  visible,
  onClose,
  onSuccess,
}: AddModeratorDialogProps) {
  const { addModerator, isLoading } = useAddModerator();
  const [moderatorDID, setModeratorDID] = useState("");
  const [permissions, setPermissions] = useState({
    ban: false,
    hide: false,
    "livestream.manage": false,
    "message.pin": false,
  });
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  // Reset form when dialog closes
  useEffect(() => {
    if (!visible) {
      setModeratorDID("");
      setPermissions({
        ban: false,
        hide: false,
        "livestream.manage": false,
        "message.pin": false,
      });
      setError(null);
    }
  }, [visible]);

  // Clear error when user starts typing
  const handleDIDChange = (text: string) => {
    setModeratorDID(text);
    if (error) setError(null);
  };

  const handleAdd = async () => {
    setError(null);

    if (!moderatorDID.trim()) {
      setError("Please enter a DID or handle");
      return;
    }

    const selectedPermissions = Object.entries(permissions)
      .filter(([_, enabled]) => enabled)
      .map(([perm]) => perm) as (
      | "ban"
      | "hide"
      | "livestream.manage"
      | "message.pin"
    )[];

    if (selectedPermissions.length === 0) {
      setError("Please select at least one permission");
      return;
    }

    try {
      await addModerator({
        moderatorDID: moderatorDID.trim(),
        permissions: selectedPermissions,
      });
      toast.show(
        "Moderator added",
        "The new moderator has been added successfully.",
        { duration: 3 },
      );
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to add moderator");
    }
  };

  return (
    <ResponsiveDialog
      open={visible}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title="Add Moderator"
      description="Enter the DID or handle of the user you want to add as a moderator and select their permissions."
      size="md"
      dismissible={false}
    >
      {/* DID Input */}
      <View style={[my[4]]}>
        <Text style={[textStyle.gray[300], { fontSize: 13, marginBottom: 8 }]}>
          Moderator DID or Handle
        </Text>
        <Input
          value={moderatorDID}
          onChangeText={handleDIDChange}
          placeholder="did:plc:... or @handle.bsky.social"
          onSubmitEditing={handleAdd}
          returnKeyType="done"
        />
      </View>

      {/* Permissions */}
      <View style={[mb[4]]}>
        <Text style={[textStyle.gray[300], { fontSize: 13, marginBottom: 8 }]}>
          Permissions
        </Text>
        <View style={[gap.all[2]]}>
          <Pressable
            onPress={() => setPermissions((p) => ({ ...p, ban: !p.ban }))}
            style={[
              layout.flex.row,
              layout.flex.alignCenter,
              p[3],
              r.md,
              bg.neutral[800],
              borders.width.thin,
              borders.color.neutral[700],
            ]}
          >
            <View
              style={[
                {
                  width: 20,
                  height: 20,
                  borderRadius: 4,
                },
                borders.width.thin,
                borders.color.neutral[600],
                permissions.ban ? bg.blue[600] : bg.neutral[900],
                layout.flex.center,
                { marginRight: 12 },
              ]}
            >
              {permissions.ban && <Check size={12} color="white" />}
            </View>
            <View>
              <Text
                style={[textStyle.white, { fontSize: 14, fontWeight: "500" }]}
              >
                Ban Users
              </Text>
              <Text style={[textStyle.gray[400], { fontSize: 12 }]}>
                Block users from chat
              </Text>
            </View>
          </Pressable>

          <Pressable
            onPress={() => setPermissions((p) => ({ ...p, hide: !p.hide }))}
            style={[
              layout.flex.row,
              layout.flex.alignCenter,
              p[3],
              r.md,
              bg.neutral[800],
              borders.width.thin,
              borders.color.neutral[700],
            ]}
          >
            <View
              style={[
                {
                  width: 20,
                  height: 20,
                  borderRadius: 4,
                },
                borders.width.thin,
                borders.color.neutral[600],
                permissions.hide ? bg.blue[600] : bg.neutral[900],
                layout.flex.center,
                { marginRight: 12 },
              ]}
            >
              {permissions.hide && (
                <Text style={[textStyle.white, { fontSize: 12 }]}>✓</Text>
              )}
            </View>
            <View>
              <Text
                style={[textStyle.white, { fontSize: 14, fontWeight: "500" }]}
              >
                Hide Messages
              </Text>
              <Text style={[textStyle.gray[400], { fontSize: 12 }]}>
                Hide individual chat messages
              </Text>
            </View>
          </Pressable>

          <Pressable
            onPress={() =>
              setPermissions((p) => ({
                ...p,
                "livestream.manage": !p["livestream.manage"],
              }))
            }
            style={[
              layout.flex.row,
              layout.flex.alignCenter,
              p[3],
              r.md,
              bg.neutral[800],
              borders.width.thin,
              borders.color.neutral[700],
            ]}
          >
            <View
              style={[
                {
                  width: 20,
                  height: 20,
                  borderRadius: 4,
                },
                borders.width.thin,
                borders.color.neutral[600],
                permissions["livestream.manage"]
                  ? bg.blue[600]
                  : bg.neutral[900],
                layout.flex.center,
                { marginRight: 12 },
              ]}
            >
              {permissions["livestream.manage"] && (
                <Text style={[textStyle.white, { fontSize: 12 }]}>✓</Text>
              )}
            </View>
            <View>
              <Text
                style={[textStyle.white, { fontSize: 14, fontWeight: "500" }]}
              >
                Manage Livestream
              </Text>
              <Text style={[textStyle.gray[400], { fontSize: 12 }]}>
                Update stream title
              </Text>
            </View>
          </Pressable>

          <Pressable
            onPress={() =>
              setPermissions((p) => ({
                ...p,
                "message.pin": !p["message.pin"],
              }))
            }
            style={[
              layout.flex.row,
              layout.flex.alignCenter,
              p[3],
              r.md,
              bg.neutral[800],
              borders.width.thin,
              borders.color.neutral[700],
            ]}
          >
            <View
              style={[
                {
                  width: 20,
                  height: 20,
                  borderRadius: 4,
                },
                borders.width.thin,
                borders.color.neutral[600],
                permissions["message.pin"] ? bg.blue[600] : bg.neutral[900],
                layout.flex.center,
                { marginRight: 12 },
              ]}
            >
              {permissions["message.pin"] && (
                <Text style={[textStyle.white, { fontSize: 12 }]}>✓</Text>
              )}
            </View>
            <View>
              <Text
                style={[textStyle.white, { fontSize: 14, fontWeight: "500" }]}
              >
                Pin Messages
              </Text>
              <Text style={[textStyle.gray[400], { fontSize: 12 }]}>
                Pin and unpin chat messages
              </Text>
            </View>
          </Pressable>
        </View>
      </View>

      {/* Error */}
      {error && (
        <View
          style={[
            bg.red[900],
            p[3],
            r.md,
            borders.width.thin,
            borders.color.red[700],
            mb[4],
          ]}
        >
          <Text style={[textStyle.red[400], { fontSize: 13 }]}>{error}</Text>
        </View>
      )}

      {/* Actions */}
      <DialogFooter>
        <Button
          width="min"
          variant="secondary"
          onPress={onClose}
          disabled={isLoading}
        >
          Cancel
        </Button>
        <Button
          width="min"
          variant="primary"
          onPress={handleAdd}
          disabled={
            isLoading ||
            !moderatorDID.trim() ||
            Object.values(permissions).every((p) => !p)
          }
        >
          {isLoading ? "Adding..." : "Add Moderator"}
        </Button>
      </DialogFooter>
    </ResponsiveDialog>
  );
}
