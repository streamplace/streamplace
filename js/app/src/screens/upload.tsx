import { Text, zero } from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { useCallback, useRef, useState } from "react";
import { Pressable, ScrollView, View } from "react-native";
import * as tus from "tus-js-client";

type Status =
  | { kind: "idle" }
  | { kind: "creating" }
  | { kind: "uploading"; pct: number; bytesSent: number; bytesTotal: number }
  | { kind: "done"; uploadUrl: string }
  | { kind: "error"; message: string };

export default function UploadScreen() {
  const agent = usePDSAgent();
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const uploadRef = useRef<tus.Upload | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [status, setStatus] = useState<Status>({ kind: "idle" });

  const pick = useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  const handleFileInputChange = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const f = event.target.files?.[0] ?? null;
      setFile(f);
      setStatus({ kind: "idle" });
      event.target.value = "";
    },
    [],
  );

  const start = useCallback(async () => {
    if (!agent || !file) return;
    if (!agent.did) {
      setStatus({ kind: "error", message: "Not logged in" });
      return;
    }
    setStatus({ kind: "creating" });
    try {
      const res = await agent.place.stream.media.createUpload({
        size: file.size,
        mimeType: file.type || "application/octet-stream",
        filename: file.name,
      });
      if (!res.success) throw new Error("createUpload failed");
      const { uploadUrl, uploadToken } = res.data;

      await new Promise<void>((resolve, reject) => {
        const upload = new tus.Upload(file, {
          uploadUrl,
          retryDelays: [0, 1000, 3000, 5000],
          headers: { Authorization: `Bearer ${uploadToken}` },
          metadata: { filename: file.name, filetype: file.type },
          onError(err) {
            reject(err);
          },
          onProgress(bytesSent, bytesTotal) {
            setStatus({
              kind: "uploading",
              pct: bytesTotal > 0 ? (bytesSent / bytesTotal) * 100 : 0,
              bytesSent,
              bytesTotal,
            });
          },
          onSuccess() {
            resolve();
          },
        });
        uploadRef.current = upload;
        upload.start();
      });

      setStatus({ kind: "done", uploadUrl });
    } catch (err) {
      console.error("upload failed", err);
      setStatus({
        kind: "error",
        message: err instanceof Error ? err.message : String(err),
      });
    } finally {
      uploadRef.current = null;
    }
  }, [agent, file]);

  const cancel = useCallback(() => {
    uploadRef.current?.abort();
    uploadRef.current = null;
    setStatus({ kind: "idle" });
  }, []);

  return (
    <ScrollView contentContainerStyle={[zero.p[4], zero.gap.all[4]]}>
      <Text size="xl">Resumable upload (TUS)</Text>
      <Text>
        Developer test page for /api/upload. Pick a file, hit upload, watch
        bytes go.
      </Text>

      <Pressable
        onPress={pick}
        style={[
          zero.p[4],
          { backgroundColor: "#222", borderRadius: 8 },
          zero.layout.flex.center,
        ]}
      >
        <Text>{file ? `Selected: ${file.name}` : "Choose a file"}</Text>
        {file && (
          <Text size="sm">
            {file.type || "unknown"} — {humanBytes(file.size)}
          </Text>
        )}
      </Pressable>

      {file && status.kind !== "uploading" && (
        <Pressable
          onPress={start}
          style={[
            zero.p[4],
            { backgroundColor: "#0a7", borderRadius: 8 },
            zero.layout.flex.center,
          ]}
        >
          <Text>
            {status.kind === "creating" ? "Creating upload…" : "Start upload"}
          </Text>
        </Pressable>
      )}

      {status.kind === "uploading" && (
        <View style={[zero.gap.all[2]]}>
          <Text>
            Uploading: {status.pct.toFixed(1)}% ({humanBytes(status.bytesSent)}{" "}
            / {humanBytes(status.bytesTotal)})
          </Text>
          <View
            style={[{ height: 8, backgroundColor: "#333", borderRadius: 4 }]}
          >
            <View
              style={[
                {
                  height: 8,
                  width: `${status.pct}%`,
                  backgroundColor: "#0a7",
                  borderRadius: 4,
                },
              ]}
            />
          </View>
          <Pressable
            onPress={cancel}
            style={[
              zero.p[3],
              { backgroundColor: "#722", borderRadius: 8 },
              zero.layout.flex.center,
            ]}
          >
            <Text>Cancel</Text>
          </Pressable>
        </View>
      )}

      {status.kind === "done" && (
        <Text style={{ color: "#0f0" }}>
          Upload complete: {status.uploadUrl}
        </Text>
      )}

      {status.kind === "error" && (
        <Text style={{ color: "#f88" }}>Error: {status.message}</Text>
      )}

      <input
        type="file"
        ref={fileInputRef}
        onChange={handleFileInputChange}
        style={{ display: "none" }}
      />
    </ScrollView>
  );
}

function humanBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
}
