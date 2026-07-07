import { place } from "streamplace";
import { LivestreamProblem } from "./livestream-state";

const VARIANCE_THRESHOLD = 0.5;
const DURATION_THRESHOLD = 5000000000; // 5s in ns

const detectVariableSegmentLength = (
  segments: place.stream.segment.Main[],
): { variable: boolean; duration: boolean } => {
  if (segments.length < 3) {
    // Need at least 3 segments to detect variability
    return { variable: false, duration: false };
  }

  const durations = segments
    .map((segment) => segment.duration)
    .filter(
      (duration): duration is number => duration !== undefined && duration > 0,
    );

  if (durations.length < 3) {
    return { variable: false, duration: false };
  }

  // Calculate mean
  const mean =
    durations.reduce((sum: number, duration: number) => sum + duration, 0) /
    durations.length;

  // Calculate standard deviation
  const variance =
    durations.reduce((sum: number, duration: number) => {
      const diff = duration - mean;
      return sum + diff * diff;
    }, 0) / durations.length;
  const stdDev = Math.sqrt(variance);

  // Calculate coefficient of variation (CV)
  const cv = stdDev / mean;

  // CV > 0.5 indicates high variability
  // This threshold can be adjusted based on testing
  return {
    variable: cv > VARIANCE_THRESHOLD,
    duration: mean > DURATION_THRESHOLD,
  };
};

export const findProblems = (
  segments: place.stream.segment.Main[],
): LivestreamProblem[] => {
  const problems: LivestreamProblem[] = [];
  let hasBFrames = false;
  for (const segment of segments) {
    const video = segment.video?.[0];
    if (!video) {
      // i mean yes this is a problem but it can't happen yet
      continue;
    }
    if (video.bframes === true) {
      hasBFrames = true;
      break;
    }
  }
  if (hasBFrames) {
    problems.push({
      code: "bframes",
      message:
        "Your stream contains B-Frames, which are not supported in Streamplace. Your stream will stutter.",
      severity: "error",
      link: "https://stream.place/docs/guides/start-streaming/obs/#obs-configuration",
    });
  }

  const { variable, duration } = detectVariableSegmentLength(segments);
  if (variable) {
    problems.push({
      code: "variable_segment_length",
      message:
        "Your stream contains variable segment lengths, which may cause playback issues.",
      severity: "warning",
      link: "https://stream.place/docs/guides/start-streaming/obs/#obs-configuration",
    });
  }
  if (duration) {
    problems.push({
      code: "long_segments",
      message:
        "Your stream contains long segments (>5s). This will work fine, but increases the delay of the livestream.",
      severity: "warning",
      link: "https://stream.place/docs/guides/start-streaming/obs/#obs-configuration",
    });
  }

  return problems;
};
