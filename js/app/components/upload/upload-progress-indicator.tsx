import { Text, useTheme } from "@streamplace/components";
import { AlertCircle, ArrowUp, CheckCircle2, X } from "lucide-react-native";
import { useSyncExternalStore } from "react";
import { Pressable, View } from "react-native";
import {
  cancelUpload,
  dismissUpload,
  getUploads,
  subscribeUploads,
  UploadJob,
} from "utils/upload-manager";

// Floating upload status card, pinned to the lower-right corner of the app
// shell. Lives outside the navigators so it keeps rendering progress while
// the user moves between screens; the uploads themselves live in
// utils/upload-manager.

function humanBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

function UploadRow({ job }: { job: UploadJob }) {
  const { theme } = useTheme();
  const pct = job.bytesTotal > 0 ? (job.bytesSent / job.bytesTotal) * 100 : 0;

  return (
    <View style={{ gap: 6 }}>
      <View
        style={{
          flexDirection: "row",
          alignItems: "center",
          gap: 8,
        }}
      >
        {job.status === "uploading" && (
          <ArrowUp size={16} color={theme.colors.primary} />
        )}
        {job.status === "done" && (
          <CheckCircle2 size={16} color={theme.colors.success} />
        )}
        {job.status === "error" && (
          <AlertCircle size={16} color={theme.colors.destructive} />
        )}
        <Text
          size="sm"
          numberOfLines={1}
          style={{ flex: 1, fontWeight: "600" }}
        >
          {job.filename}
        </Text>
        <Pressable
          onPress={() =>
            job.status === "uploading"
              ? cancelUpload(job.id)
              : dismissUpload(job.id)
          }
          hitSlop={8}
        >
          <X size={16} color={theme.colors.mutedForeground} />
        </Pressable>
      </View>
      {job.status === "uploading" && (
        <>
          <View
            style={{
              height: 6,
              borderRadius: 3,
              backgroundColor: theme.colors.muted,
              overflow: "hidden",
              width: "100%",
            }}
          >
            <View
              style={{
                height: 6,
                width: `${pct}%`,
                backgroundColor: theme.colors.primary,
                borderRadius: 3,
              }}
            />
          </View>
          <Text size="xs" color="muted">
            {pct.toFixed(1)}% — {humanBytes(job.bytesSent)} /{" "}
            {humanBytes(job.bytesTotal)}
          </Text>
        </>
      )}
      {job.status === "done" && (
        <Text size="xs" style={{ color: theme.colors.success }}>
          Upload complete — processing continues on the server
        </Text>
      )}
      {job.status === "error" && (
        <Text size="xs" color="destructive" numberOfLines={3}>
          {job.error || "Upload failed"}
        </Text>
      )}
    </View>
  );
}

export default function UploadProgressIndicator() {
  const { theme } = useTheme();
  const jobs = useSyncExternalStore(subscribeUploads, getUploads, getUploads);

  if (jobs.length === 0) return null;

  return (
    <View
      pointerEvents="box-none"
      style={{
        position: "absolute",
        bottom: 16,
        right: 16,
        zIndex: 1000,
        width: 320,
        maxWidth: "90%",
      }}
    >
      <View
        style={{
          backgroundColor: theme.colors.background,
          borderColor: theme.colors.border,
          borderWidth: 1,
          borderRadius: 12,
          padding: 12,
          gap: 12,
          ...theme.shadows.lg,
        }}
      >
        {jobs.map((job) => (
          <UploadRow key={job.id} job={job} />
        ))}
      </View>
    </View>
  );
}
