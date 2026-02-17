import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import {
  AlertCircle,
  Ban,
  Check,
  Clock,
  Eye,
  EyeOff,
  FileText,
  Loader,
  MessageSquare,
  RefreshCw,
  Shield,
  X,
} from "lucide-react-native";
import { useCallback, useMemo, useState } from "react";
import { Pressable, ScrollView, Text, View } from "react-native";
import { Button, Dialog, DialogFooter, useToast } from "../../components/ui";
import { useAvatars } from "../../hooks/useAvatars";
import { atoms, tokens } from "../../lib/theme";
import {
  AuditLogEntry,
  useAuditLog,
  useUndoModerationAction,
} from "../../streamplace-store/audit-log";
import { usePDSAgent } from "../../streamplace-store/xrpc";
import { formatHandleWithAt } from "../../utils/format-handle";

// Parse AT URI to extract repo DID, collection, and rkey
function parseAtUri(
  uri: string,
): { repo: string; collection: string; rkey: string } | null {
  // Format: at://did/collection/rkey
  const match = uri.match(/^at:\/\/([^/]+)\/([^/]+)\/([^/]+)$/);
  if (!match) return null;
  return { repo: match[1], collection: match[2], rkey: match[3] };
}

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
  mt,
  px,
  py,
} = atoms;
const { typography } = tokens;

type ActionFilter = "all" | "bans" | "hidden";

interface AuditLogPanelProps {
  embedded?: boolean;
}

export default function AuditLogPanel({
  embedded = false,
}: AuditLogPanelProps) {
  const agent = usePDSAgent();
  const [actionFilter, setActionFilter] = useState<ActionFilter>("all");
  // Cache for revealed message content: messageUri -> text
  const [revealedMessages, setRevealedMessages] = useState<
    Record<string, string>
  >({});
  const [loadingMessages, setLoadingMessages] = useState<Set<string>>(
    new Set(),
  );

  const fetchMessageContent = useCallback(
    async (messageUri: string) => {
      if (!agent || revealedMessages[messageUri] !== undefined) return;

      const parsed = parseAtUri(messageUri);
      if (!parsed) return;

      setLoadingMessages((prev) => new Set(prev).add(messageUri));
      try {
        const response = await agent.com.atproto.repo.getRecord({
          repo: parsed.repo,
          collection: parsed.collection,
          rkey: parsed.rkey,
        });
        const record = response.data.value as { text?: string };
        setRevealedMessages((prev) => ({
          ...prev,
          [messageUri]: record.text ?? "[No text content]",
        }));
      } catch {
        setRevealedMessages((prev) => ({
          ...prev,
          [messageUri]: "[Failed to load message]",
        }));
      } finally {
        setLoadingMessages((prev) => {
          const next = new Set(prev);
          next.delete(messageUri);
          return next;
        });
      }
    },
    [agent, revealedMessages],
  );

  // Map filter to API action parameter
  const apiAction = useMemo(() => {
    switch (actionFilter) {
      case "bans":
        return undefined; // We'll filter client-side for both createBlock and deleteBlock
      case "hidden":
        return undefined; // We'll filter client-side for both createGate and deleteGate
      default:
        return undefined;
    }
  }, [actionFilter]);

  const { logs, isLoading, error, hasMore, loadMore, refresh } = useAuditLog({
    limit: 50,
    action: apiAction,
  });

  // Client-side filter for compound actions
  const filteredLogs = useMemo(() => {
    if (actionFilter === "bans") {
      return logs.filter(
        (l) => l.action === "createBlock" || l.action === "deleteBlock",
      );
    }
    if (actionFilter === "hidden") {
      return logs.filter(
        (l) => l.action === "createGate" || l.action === "deleteGate",
      );
    }
    return logs;
  }, [logs, actionFilter]);

  // Map gate URI -> hidden message URI from createGate entries
  const messageUriByGateUri = useMemo(() => {
    const map = new Map<string, string>();
    for (const log of logs) {
      if (log.action === "createGate" && log.resultUri && log.targetUri) {
        map.set(log.resultUri, log.targetUri);
      }
    }
    return map;
  }, [logs]);

  // Collect DIDs that need profile resolution (where handle === did, meaning unresolved)
  const unresolvedDids = useMemo(() => {
    const dids: string[] = [];
    for (const log of filteredLogs) {
      // Check moderator
      if (log.moderator?.did && log.moderator.handle === log.moderator.did) {
        dids.push(log.moderator.did);
      }
      // Check target
      if (
        log.targetProfile?.did &&
        log.targetProfile.handle === log.targetProfile.did
      ) {
        dids.push(log.targetProfile.did);
      }
      // Also check targetDid if no targetProfile
      if (log.targetDid && !log.targetProfile) {
        dids.push(log.targetDid);
      }
    }
    return [...new Set(dids)]; // Dedupe
  }, [filteredLogs]);

  // Resolve unresolved profiles using useAvatars
  const resolvedProfiles = useAvatars(unresolvedDids);

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
          <FileText size={18} color={textStyle.white.color} />
          <Text style={[textStyle.white, typography.universal.xl]}>
            Audit Log
          </Text>
        </View>
        <Pressable
          onPress={refresh}
          disabled={isLoading}
          style={[
            p[2],
            r.md,
            bg.neutral[800],
            borders.width.thin,
            borders.color.neutral[700],
            isLoading && { opacity: 0.5 },
          ]}
        >
          <RefreshCw size={16} color={textStyle.gray[400].color} />
        </Pressable>
      </View>

      {/* Filter Chips */}
      <View
        style={[
          layout.flex.row,
          gap.all[1],
          px[4],
          py[3],
          borders.bottom.width.thin,
          borders.bottom.color.neutral[700],
        ]}
      >
        {(
          [
            { label: "All", value: "all" },
            { label: "Bans", value: "bans" },
            { label: "Hidden", value: "hidden" },
          ] as const
        ).map(({ label, value }) => (
          <Button
            key={value}
            variant={actionFilter === value ? "primary" : "secondary"}
            size="pill"
            width="min"
            onPress={() => setActionFilter(value)}
            style={[r.md]}
          >
            <Text
              style={[
                actionFilter === value ? textStyle.white : textStyle.gray[300],
                typography.universal.sm,
              ]}
            >
              {label}
            </Text>
          </Button>
        ))}
      </View>

      {/* Content */}
      <ScrollView
        style={[flex.values[1], p[4]]}
        onScroll={(e) => {
          const { layoutMeasurement, contentOffset, contentSize } =
            e.nativeEvent;
          const paddingToBottom = 50;
          if (
            layoutMeasurement.height + contentOffset.y >=
            contentSize.height - paddingToBottom
          ) {
            if (!isLoading && hasMore) {
              loadMore();
            }
          }
        }}
        scrollEventThrottle={400}
      >
        {isLoading && filteredLogs.length === 0 && (
          <Text
            style={[
              textStyle.gray[400],
              typography.universal.sm,
              { textAlign: "center" },
            ]}
          >
            Loading audit logs...
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
            <Text style={[textStyle.red[400], typography.universal.xs]}>
              {error}
            </Text>
          </View>
        )}

        {!isLoading && filteredLogs.length === 0 && !error && (
          <View style={[layout.flex.center, p[6]]}>
            <FileText
              size={48}
              color={textStyle.gray[500].color}
              style={[mb[4]]}
            />
            <Text
              style={[
                textStyle.gray[400],
                typography.universal.sm,
                { textAlign: "center", marginBottom: 8 },
              ]}
            >
              No moderation actions yet
            </Text>
            <Text
              style={[
                textStyle.gray[500],
                typography.universal.xs,
                { textAlign: "center" },
              ]}
            >
              Actions by you and your moderators will appear here
            </Text>
          </View>
        )}

        {filteredLogs.map((entry, index) => {
          const messageUri =
            entry.action === "deleteGate"
              ? entry.targetUri
                ? messageUriByGateUri.get(entry.targetUri)
                : undefined
              : entry.targetUri;

          return (
            <AuditLogCard
              key={entry.id}
              entry={entry}
              streamerDid={agent?.did ?? ""}
              onUndo={refresh}
              isLast={index === filteredLogs.length - 1}
              resolvedProfiles={resolvedProfiles}
              revealedMessage={
                messageUri ? revealedMessages[messageUri] : undefined
              }
              isLoadingMessage={
                messageUri ? loadingMessages.has(messageUri) : false
              }
              onRevealMessage={fetchMessageContent}
              messageUri={messageUri}
            />
          );
        })}

        {isLoading && filteredLogs.length > 0 && (
          <Text
            style={[
              textStyle.gray[400],
              typography.universal.xs,
              { textAlign: "center", marginTop: 12 },
            ]}
          >
            Loading more...
          </Text>
        )}
      </ScrollView>
    </View>
  );
}

interface AuditLogCardProps {
  entry: AuditLogEntry;
  streamerDid: string;
  onUndo: () => void;
  isLast: boolean;
  resolvedProfiles: Record<string, ProfileViewDetailed>;
  revealedMessage?: string;
  isLoadingMessage?: boolean;
  onRevealMessage?: (messageUri: string) => void;
  messageUri?: string;
}

function AuditLogCard({
  entry,
  streamerDid,
  onUndo,
  isLast,
  resolvedProfiles,
  revealedMessage,
  isLoadingMessage,
  onRevealMessage,
  messageUri,
}: AuditLogCardProps) {
  const {
    undoBlock,
    undoGate,
    isLoading: undoLoading,
  } = useUndoModerationAction();
  const [showUndoConfirm, setShowUndoConfirm] = useState(false);
  const toast = useToast();

  const actionInfo = getActionInfo(entry.action);

  // Use resolved profile if available, otherwise fall back to entry data
  const getResolvedProfile = (
    profile: typeof entry.moderator | undefined,
    fallbackDid: string | undefined,
  ) => {
    const did = profile?.did || fallbackDid;
    if (did && resolvedProfiles[did]) {
      return resolvedProfiles[did];
    }
    return profile;
  };

  const moderatorProfile = getResolvedProfile(entry.moderator, undefined);
  const targetProfile = getResolvedProfile(
    entry.targetProfile,
    entry.targetDid,
  );

  const moderatorName = moderatorProfile
    ? formatHandleWithAt(moderatorProfile)
    : "Unknown";
  const targetName = targetProfile
    ? formatHandleWithAt(targetProfile)
    : (entry.targetDid ?? "N/A");

  const timeAgo = useMemo(() => {
    const date = new Date(entry.createdAt);
    const now = new Date();
    const diff = now.getTime() - date.getTime();
    const minutes = Math.floor(diff / 60000);
    const hours = Math.floor(diff / 3600000);
    const days = Math.floor(diff / 86400000);

    if (minutes < 1) return "just now";
    if (minutes < 60) return `${minutes}m ago`;
    if (hours < 24) return `${hours}h ago`;
    if (days < 7) return `${days}d ago`;
    return date.toLocaleDateString();
  }, [entry.createdAt]);

  const handleUndo = async () => {
    if (!entry.resultUri) return;

    try {
      if (entry.action === "createBlock") {
        await undoBlock(entry.resultUri, streamerDid);
        toast.show("Ban revoked", "The user has been unbanned.", {
          duration: 3,
        });
      } else if (entry.action === "createGate") {
        await undoGate(entry.resultUri, streamerDid);
        toast.show("Message unhidden", "The message is now visible.", {
          duration: 3,
        });
      }
      setShowUndoConfirm(false);
      // Note: We don't call onUndo() here because WebSocket events will
      // automatically update the list with the new delete entry and
      // mark the original entry as no longer undoable.
    } catch (err) {
      toast.show(
        "Failed to undo",
        err instanceof Error ? err.message : "An error occurred",
        { duration: 5 },
      );
    }
  };

  return (
    <View
      style={[
        p[3],
        bg.neutral[800],
        r.md,
        !isLast && mb[2],
        borders.width.thin,
        entry.success ? borders.color.neutral[700] : borders.color.red[700],
      ]}
    >
      {/* Top row: Action badge + time */}
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          mb[2],
          entry.canUndo && { paddingRight: 32 }, // Make room for delete button
        ]}
      >
        <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
          <View
            style={[
              layout.flex.row,
              layout.flex.alignCenter,
              gap.all[1],
              px[2],
              py[1],
              r.sm,
              actionInfo.bgColor,
              borders.width.thin,
              actionInfo.borderColor,
            ]}
          >
            {actionInfo.icon}
            <Text style={[actionInfo.textColor, typography.universal.xs]}>
              {actionInfo.label}
            </Text>
          </View>
          {!entry.success && (
            <View
              style={[
                layout.flex.row,
                layout.flex.alignCenter,
                gap.all[1],
                px[2],
                py[1],
                r.sm,
                bg.red[900],
                borders.width.thin,
                borders.color.red[700],
              ]}
            >
              <AlertCircle size={10} color={textStyle.red[400].color} />
              <Text style={[textStyle.red[400], typography.universal.xs]}>
                Failed
              </Text>
            </View>
          )}
        </View>
        <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[1]]}>
          <Clock size={12} color={textStyle.gray[400].color} />
          <Text style={[textStyle.gray[400], typography.universal.xs]}>
            {timeAgo}
          </Text>
        </View>
      </View>

      {/* Main content */}
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
        <Shield size={14} color={textStyle.gray[400].color} />
        <Text
          style={[textStyle.gray[300], typography.universal.sm]}
          numberOfLines={2}
        >
          <Text style={[textStyle.white, typography.universal.sm]}>
            {moderatorName}
          </Text>{" "}
          {actionInfo.verb}{" "}
          {(entry.action === "createBlock" ||
            entry.action === "deleteBlock" ||
            entry.action === "createGate" ||
            entry.action === "deleteGate") && (
            <Text style={[textStyle.white, typography.universal.sm]}>
              {targetName}
            </Text>
          )}
        </Text>
      </View>

      {/* Error message if failed */}
      {entry.errorMsg && (
        <View style={[mt[2], p[2], bg.red[950], r.sm]}>
          <Text style={[textStyle.red[400], typography.universal.xs]}>
            {entry.errorMsg}
          </Text>
        </View>
      )}

      {/* Reveal message button for gate actions */}
      {(entry.action === "createGate" || entry.action === "deleteGate") &&
        messageUri && (
          <View style={[mt[2]]}>
            {revealedMessage !== undefined ? (
              // Show revealed message content
              <View
                style={[
                  p[2],
                  bg.neutral[700],
                  r.sm,
                  layout.flex.row,
                  gap.all[2],
                  { alignItems: "flex-start" },
                ]}
              >
                <MessageSquare
                  size={12}
                  color={textStyle.gray[400].color}
                  style={{ marginTop: 2 }}
                />
                <Text
                  style={[
                    textStyle.gray[300],
                    typography.universal.xs,
                    { flex: 1 },
                  ]}
                >
                  {revealedMessage}
                </Text>
              </View>
            ) : (
              // Show reveal button
              <Pressable
                onPress={() => onRevealMessage?.(messageUri)}
                disabled={isLoadingMessage}
                style={[
                  layout.flex.row,
                  layout.flex.alignCenter,
                  gap.all[1],
                  p[2],
                  bg.neutral[700],
                  r.sm,
                  isLoadingMessage && { opacity: 0.5 },
                ]}
              >
                {isLoadingMessage ? (
                  <Loader size={12} color={textStyle.gray[400].color} />
                ) : (
                  <Eye size={12} color={textStyle.gray[400].color} />
                )}
                <Text style={[textStyle.gray[400], typography.universal.xs]}>
                  {isLoadingMessage ? "Loading..." : "Reveal message"}
                </Text>
              </Pressable>
            )}
          </View>
        )}

      {/* Delete button - positioned at top right */}
      {entry.canUndo && (
        <Pressable
          onPress={() => setShowUndoConfirm(true)}
          disabled={undoLoading}
          style={[
            { position: "absolute", top: 8, right: 8 },
            p[1],
            r.sm,
            bg.red[900],
            borders.width.thin,
            borders.color.red[700],
            undoLoading && { opacity: 0.5 },
          ]}
        >
          <X size={14} color={textStyle.red[400].color} />
        </Pressable>
      )}

      {/* Undo confirmation dialog */}
      <Dialog
        open={showUndoConfirm}
        onOpenChange={setShowUndoConfirm}
        title={
          entry.action === "createBlock" ? "Unban User?" : "Unhide Message?"
        }
        description={
          entry.action === "createBlock"
            ? `This will remove the ban on ${targetName}. They will be able to chat again.`
            : "This will make the hidden message visible again."
        }
        dismissible={false}
      >
        <DialogFooter>
          <Button
            width="min"
            variant="secondary"
            onPress={() => setShowUndoConfirm(false)}
            disabled={undoLoading}
          >
            <Text>Cancel</Text>
          </Button>
          <Button
            width="min"
            variant="primary"
            onPress={handleUndo}
            disabled={undoLoading}
          >
            <Text>{undoLoading ? "Removing..." : "Remove"}</Text>
          </Button>
        </DialogFooter>
      </Dialog>
    </View>
  );
}

function getActionInfo(action: string) {
  switch (action) {
    case "createBlock":
      return {
        label: "Banned",
        verb: "banned",
        icon: <Ban size={10} color={textStyle.red[400].color} />,
        bgColor: bg.red[900],
        borderColor: borders.color.red[700],
        textColor: textStyle.red[400],
      };
    case "deleteBlock":
      return {
        label: "Unbanned",
        verb: "unbanned",
        icon: <Check size={10} color={textStyle.green[400].color} />,
        bgColor: bg.green[900],
        borderColor: borders.color.green[700],
        textColor: textStyle.green[400],
      };
    case "createGate":
      return {
        label: "Hidden",
        verb: "hid message from",
        icon: <EyeOff size={10} color={textStyle.yellow[400].color} />,
        bgColor: bg.yellow[900],
        borderColor: borders.color.yellow[700],
        textColor: textStyle.yellow[400],
      };
    case "deleteGate":
      return {
        label: "Unhidden",
        verb: "unhid message from",
        icon: <Eye size={10} color={textStyle.green[400].color} />,
        bgColor: bg.green[900],
        borderColor: borders.color.green[700],
        textColor: textStyle.green[400],
      };
    case "updateLivestream":
      return {
        label: "Title update",
        verb: "updated stream title",
        icon: <RefreshCw size={10} color={textStyle.blue[400].color} />,
        bgColor: bg.blue[900],
        borderColor: borders.color.blue[700],
        textColor: textStyle.blue[400],
      };
    default:
      return {
        label: action,
        verb: "performed action",
        icon: <Shield size={10} color={textStyle.gray[400].color} />,
        bgColor: bg.neutral[800],
        borderColor: borders.color.neutral[600],
        textColor: textStyle.gray[400],
      };
  }
}
