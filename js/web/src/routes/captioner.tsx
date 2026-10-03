import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useSession } from "@/lib/session";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { BrowserCue, CaptionModel } from "streamplace";
import { benchmarkCaptionModel, startBrowserCaptioner } from "streamplace";

export const Route = createFileRoute("/captioner")({
  component: CaptionerPage,
});

export function CaptionerPage() {
  const { t } = useTranslation("common");
  const { did, pdsAgent, signIn } = useSession();
  const [handle, setHandle] = useState("");
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([]);
  const [device, setDevice] = useState("");
  const [language, setLanguage] = useState("");
  const [model, setModel] = useState<CaptionModel>("tiny");
  const [offset, setOffset] = useState(0);
  const [status, setStatus] = useState("captioner-status-stopped");
  const [statusModel, setStatusModel] = useState<CaptionModel>("tiny");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [running, setRunning] = useState(false);
  const [speeds, setSpeeds] = useState<Partial<Record<CaptionModel, number>>>(
    {},
  );
  const [cues, setCues] = useState<BrowserCue[]>([]);
  const stopRef = useRef<(() => Promise<void>) | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const startAbortRef = useRef<AbortController | null>(null);
  const benchmarkAbortRef = useRef<AbortController | null>(null);
  const cancelled = useRef(false);
  const mediaAvailable = Boolean(navigator.mediaDevices?.getUserMedia);

  useEffect(() => {
    cancelled.current = false;
    if (mediaAvailable)
      void navigator.mediaDevices
        .enumerateDevices()
        .then((all) => {
          if (!cancelled.current)
            setDevices(all.filter((d) => d.kind === "audioinput"));
        })
        .catch((e: Error) => {
          if (!cancelled.current) setError(e.message);
        });
    return () => {
      cancelled.current = true;
      const startAbort = startAbortRef.current;
      startAbortRef.current = null;
      startAbort?.abort();
      const benchmarkAbort = benchmarkAbortRef.current;
      benchmarkAbortRef.current = null;
      benchmarkAbort?.abort();
      const stopSession = stopRef.current;
      stopRef.current = null;
      void stopSession?.().catch(console.error);
      const stream = streamRef.current;
      streamRef.current = null;
      stream?.getTracks().forEach((track) => track.stop());
    };
  }, [mediaAvailable]);

  const stop = async () => {
    const startAbort = startAbortRef.current;
    startAbortRef.current = null;
    startAbort?.abort();
    const stopSession = stopRef.current;
    stopRef.current = null;
    const stream = streamRef.current;
    streamRef.current = null;
    stream?.getTracks().forEach((track) => track.stop());
    setBusy(true);
    setRunning(false);
    try {
      await stopSession?.();
    } catch (e) {
      if (!cancelled.current) setError(String(e));
    } finally {
      if (!cancelled.current) {
        setBusy(false);
        setStatus("captioner-status-stopped");
      }
    }
  };

  const start = async () => {
    if (!pdsAgent || !did) return;
    startAbortRef.current?.abort();
    const controller = new AbortController();
    startAbortRef.current = controller;
    const isCurrent = () =>
      !cancelled.current &&
      !controller.signal.aborted &&
      startAbortRef.current === controller;
    setBusy(true);
    setError("");
    setStatus("captioner-status-loading");
    let stream: MediaStream | undefined;
    try {
      stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          deviceId: device ? { exact: device } : undefined,
          channelCount: 1,
        },
        video: false,
      });
      if (!isCurrent()) {
        stream.getTracks().forEach((track) => track.stop());
        return;
      }
      streamRef.current = stream;
      setRunning(true);
      const allDevices = await navigator.mediaDevices.enumerateDevices();
      if (!isCurrent()) {
        stream.getTracks().forEach((track) => track.stop());
        if (streamRef.current === stream) streamRef.current = null;
        return;
      }
      setDevices(allDevices.filter((d) => d.kind === "audioinput"));
      const session = await startBrowserCaptioner({
        nodeURL: window.location.origin,
        agent: pdsAgent,
        stream,
        model,
        language,
        offsetMs: offset,
        signal: controller.signal,
        onCue: (cue) => {
          if (!isCurrent()) return;
          setCues((before) =>
            [...before.filter((item) => item.id !== cue.id), cue].slice(-20),
          );
        },
        onSpeed: (rtf) => {
          if (!isCurrent()) return;
          setSpeeds((before) => ({ ...before, [model]: rtf }));
          setStatus(
            rtf < 1 ? "captioner-status-listening" : "captioner-status-slow",
          );
        },
        onError: (e) => {
          if (isCurrent()) setError(e.message);
        },
      });
      if (!isCurrent()) {
        await session.stop();
        stream.getTracks().forEach((track) => track.stop());
        if (streamRef.current === stream) streamRef.current = null;
        return;
      }
      stopRef.current = session.stop;
      setBusy(false);
    } catch (e) {
      stream?.getTracks().forEach((track) => track.stop());
      if (streamRef.current === stream) streamRef.current = null;
      if (startAbortRef.current === controller) {
        startAbortRef.current = null;
        if (!cancelled.current) {
          const intentionallyAborted =
            e !== null &&
            typeof e === "object" &&
            "name" in e &&
            e.name === "AbortError";
          if (!intentionallyAborted) setError(String(e));
          setRunning(false);
          setBusy(false);
          setStatus("captioner-status-stopped");
        }
      }
    }
  };

  const measure = async () => {
    benchmarkAbortRef.current?.abort();
    const controller = new AbortController();
    benchmarkAbortRef.current = controller;
    const isCurrent = () =>
      !cancelled.current &&
      !controller.signal.aborted &&
      benchmarkAbortRef.current === controller;
    setBusy(true);
    setError("");
    const measured: Partial<Record<CaptionModel, number>> = {};
    try {
      for (const name of ["tiny", "base", "small"] as const) {
        if (!isCurrent()) return;
        setStatusModel(name);
        setStatus("captioner-status-measuring");
        measured[name] = await benchmarkCaptionModel(
          window.location.origin,
          name,
          undefined,
          controller.signal,
        );
        if (!isCurrent()) return;
        setSpeeds({ ...measured });
      }
      const suggested =
        (["small", "base", "tiny"] as const).find(
          (name) => (measured[name] ?? Infinity) < 0.7,
        ) ?? "tiny";
      if (!isCurrent()) return;
      setModel(suggested);
      setStatusModel(suggested);
      setStatus("captioner-status-suggested");
    } catch (e) {
      const intentionallyAborted =
        e !== null &&
        typeof e === "object" &&
        "name" in e &&
        e.name === "AbortError";
      if (isCurrent() && !intentionallyAborted) setError(String(e));
    } finally {
      if (benchmarkAbortRef.current === controller) {
        benchmarkAbortRef.current = null;
        if (!cancelled.current) setBusy(false);
      }
    }
  };

  return (
    <main className="mx-auto flex w-full max-w-2xl flex-col gap-6 p-6">
      <h1 className="font-display text-2xl font-semibold">
        {t("captioner-title")}
      </h1>
      <p>{t("captioner-description")}</p>
      <p className="text-muted-foreground text-sm">
        {t("captioner-policy-description")}
      </p>
      {!did ? (
        <form
          className="flex gap-2"
          onSubmit={(event) => {
            event.preventDefault();
            void signIn(handle, "redirect").catch((e: Error) =>
              setError(e.message),
            );
          }}
        >
          <Input
            aria-label={t("captioner-handle")}
            placeholder={t("captioner-handle-placeholder")}
            value={handle}
            onChange={(e) => setHandle(e.target.value)}
            required
          />
          <Button type="submit">{t("captioner-sign-in")}</Button>
        </form>
      ) : (
        <p className="text-sm">{t("captioner-signed-in", { did })}</p>
      )}
      {!mediaAvailable && (
        <p role="alert">{t("captioner-microphone-unavailable")}</p>
      )}
      {!globalThis.crossOriginIsolated && (
        <p role="alert">{t("captioner-isolation-unavailable")}</p>
      )}
      <fieldset
        disabled={busy || running}
        className="border-border grid gap-4 rounded-lg border p-4"
      >
        <label className="grid gap-2">
          {t("captioner-microphone")}
          <select
            className="border-input bg-background rounded-md border p-2"
            value={device}
            onChange={(e) => setDevice(e.target.value)}
          >
            <option value="">{t("captioner-system-default")}</option>
            {devices.map((d, index) => (
              <option key={d.deviceId || index} value={d.deviceId}>
                {d.label ||
                  t("captioner-microphone-number", { number: index + 1 })}
              </option>
            ))}
          </select>
        </label>
        <label className="grid gap-2">
          {t("captioner-language")}
          <Input
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
            placeholder={t("captioner-language-placeholder")}
          />
        </label>
        <label className="grid gap-2">
          {t("captioner-model")}
          <select
            className="border-input bg-background rounded-md border p-2"
            value={model}
            onChange={(e) => setModel(e.target.value as CaptionModel)}
          >
            {(["tiny", "base", "small"] as const).map((name) => (
              <option key={name} value={name}>
                {name}
                {speeds[name] !== undefined
                  ? t("captioner-model-speed", {
                      factor: speeds[name]?.toFixed(2),
                    })
                  : ""}
              </option>
            ))}
          </select>
        </label>
        <label className="grid gap-2">
          {t("captioner-offset")}
          <Input
            type="number"
            value={offset}
            onChange={(e) => setOffset(Number(e.target.value))}
          />
        </label>
        <Button variant="outline" onClick={() => void measure()}>
          {t("captioner-measure")}
        </Button>
      </fieldset>
      <div className="flex items-center gap-4">
        <Button
          disabled={(busy && !running) || !did || !mediaAvailable}
          onClick={() => void (running ? stop() : start())}
        >
          {t(running ? "captioner-stop" : "captioner-start")}
        </Button>
        <span role="status">{t(status, { model: statusModel })}</span>
      </div>
      {error && (
        <p className="text-destructive" role="alert">
          {error}
        </p>
      )}
      <section
        aria-label={t("captioner-transcript")}
        aria-live="polite"
        className="border-border space-y-2 rounded-lg border p-4"
      >
        {cues.map((cue) => (
          <p key={cue.id} className={cue.final ? "" : "text-muted-foreground"}>
            {cue.text}
          </p>
        ))}
      </section>
      {did && (
        <p className="text-sm">
          {t("captioner-display-url")}{" "}
          <a
            className="underline"
            href={`/embed/captions/${encodeURIComponent(did)}`}
          >
            {window.location.origin}/embed/captions/{did}
          </a>
        </p>
      )}
      <details className="text-sm">
        <summary>{t("captioner-obs-title")}</summary>
        <p className="mt-2">{t("captioner-obs-description")}</p>
      </details>
    </main>
  );
}
