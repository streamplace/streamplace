import { Platform } from "react-native";

let timeOffset = 0;
let hasWarned = false;
let OriginalDate: DateConstructor = Date;

const CLOCK_DRIFT_THRESHOLD_MS = 5000; // 5 seconds

export function getTimeOffset(): number {
  return timeOffset;
}

export function setTimeOffset(offset: number): void {
  timeOffset = offset;
}

export function checkClockDrift(serverTime: string): {
  hasDrift: boolean;
  driftMs: number;
  driftSeconds: number;
} {
  const serverDate = new Date(serverTime);
  const clientDate = new Date();
  const drift = Math.abs(serverDate.getTime() - clientDate.getTime());

  if (drift > CLOCK_DRIFT_THRESHOLD_MS) {
    const driftSeconds = Math.round(drift / 1000);
    if (!hasWarned) {
      hasWarned = true;
      console.warn(
        `clock drift detected: ${driftSeconds}s difference from server time. ` +
          `this may cause issues with time-sensitive operations. ` +
          `please sync your system clock.`,
      );
    }
    return { hasDrift: true, driftMs: drift, driftSeconds };
  } else {
    return {
      hasDrift: false,
      driftMs: drift,
      driftSeconds: Math.round(drift / 1000),
    };
  }
}

export function syncTimeWithServer(
  serverTime: string,
  networkLatencyMs: number,
): void {
  const serverDate = new OriginalDate(serverTime);
  const clientDate = new OriginalDate();
  const offset = serverDate.getTime() - clientDate.getTime() - networkLatencyMs;

  setTimeOffset(offset);
}

export function getSyncedDate(): Date {
  const now = new Date();
  if (timeOffset !== 0) {
    return new Date(now.getTime() + timeOffset);
  }
  return now;
}

export function getSystemDate(): Date {
  return new OriginalDate();
}

export function getSystemTime(): number {
  return OriginalDate.now();
}

export function initializeTimeSync(): void {
  if (Platform.OS !== "web") {
    return;
  }

  // store original Date
  OriginalDate = Date;
  const OriginalDatePrototype = OriginalDate.prototype;

  // create patched Date constructor
  function PatchedDate(this: any, ...args: any[]): any {
    // If called as a function (no `new`), forward to original Date to get the string form
    if (!(this instanceof PatchedDate)) {
      return OriginalDate.apply(undefined, args as any);
    }

    // If called as a constructor, construct a Date with synced time when no args provided
    if (args.length === 0) {
      const syncedTime = OriginalDate.now() + timeOffset;
      return Reflect.construct(OriginalDate, [syncedTime], PatchedDate);
    }

    // Otherwise construct with the provided arguments
    return Reflect.construct(OriginalDate, args, PatchedDate);
  }

  // copy static methods
  PatchedDate.now = function (): number {
    return OriginalDate.now() + timeOffset;
  };

  PatchedDate.parse = OriginalDate.parse;
  PatchedDate.UTC = OriginalDate.UTC;

  // copy prototype
  PatchedDate.prototype = OriginalDatePrototype;

  // replace global Date
  (globalThis as any).Date = PatchedDate;
}
