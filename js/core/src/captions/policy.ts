import { place } from "streamplace";

export type CaptionPolicy = place.stream.metadata.captionPolicy.Main;

/** How the canonical caption track is produced; absent means auto. */
export type CaptionMode = "auto" | "ingest" | "off";

/** The dashboard's view of a streamer's caption policy, defaults applied. */
export interface CaptionPolicySettings {
  mode: CaptionMode;
  allowNodeCaptions: boolean;
  languages: string[];
}

export function readCaptionPolicy(
  policy: CaptionPolicy | undefined,
): CaptionPolicySettings {
  const canonical = policy?.canonical;
  return {
    mode:
      canonical === "ingest" ? "ingest" : canonical === "off" ? "off" : "auto",
    allowNodeCaptions: policy?.allowNodeCaptions !== false,
    languages: policy?.languages ?? [],
  };
}

/** The captionPolicy object to store in the metadata configuration. */
export function buildCaptionPolicy(
  settings: CaptionPolicySettings,
): CaptionPolicy {
  return {
    canonical: settings.mode,
    allowNodeCaptions: settings.allowNodeCaptions,
    ...(settings.languages.length > 0
      ? { languages: settings.languages.slice(0, 4) }
      : {}),
  };
}
