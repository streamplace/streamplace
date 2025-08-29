// Super quick and dirty hooks to get error reporting in

import { Platform } from "react-native";
import pkg from "../package.json";

let errorReportingConfig = {
  ip: "",
};

// random string set on load
let randomString = Math.random().toString(36);

// get it from cdn-cgi/trace
// cloudflare only!
const fetchIp = async () => {
  try {
    const res = await fetch("https://api.ipify.org?format=json");
    const j = await res.json();
    errorReportingConfig.ip = j.ip;
  } catch (e) {
    // ignore
  }
};
fetchIp();

const getReportingUrl = (): string => {
  return "/api/player-event";
};

export const register = () => {
  console.log("file registered, error reporting should be ok");
};

const getExtraInfo = () => {
  return {
    os: Platform.OS,
    osVersion: Platform.Version,
    appVersion: pkg.version,
    environment: __DEV__ ? "development" : "production",
    ip: errorReportingConfig.ip,
  };
};

const getPlayerId = (): string => {
  if (errorReportingConfig.ip) {
    const encoded = "app-" + btoa(errorReportingConfig.ip);
    return encoded;
  }

  return `app-${randomString}`;
};

const sendPlayerEvent = async (
  eventType: string,
  message: string,
  meta: any = {},
) => {
  try {
    const reportingURL = getReportingUrl();

    const playerEventData = {
      time: new Date().toISOString(),
      playerId: getPlayerId(),
      eventType,
      meta: {
        message,
        timestamp: new Date().toISOString(),
        userAgent:
          typeof navigator !== "undefined" ? navigator.userAgent : undefined,
        url: typeof window !== "undefined" ? window.location?.href : undefined,
        ...getExtraInfo(),
        ...meta,
      },
    };

    await fetch(reportingURL, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(playerEventData),
    });
  } catch (e) {
    console.warn("Failed to send error event:", e);
  }
};

// Hook into React Native unhandled errors
// @ts-expect-error
if (global.ErrorUtils) {
  // @ts-expect-error
  const defaultHandler = global.ErrorUtils.getGlobalHandler();
  // @ts-expect-error
  global.ErrorUtils.setGlobalHandler(
    (error: { message: string; stack: any }, isFatal: any) => {
      sendPlayerEvent("unhandled-error", error.message, {
        stack: error.stack,
        isFatal,
        source: "react-native-error-utils",
      });

      if (defaultHandler) {
        defaultHandler(error, isFatal);
      }
    },
  );
}
if (typeof process !== "undefined" && process.on) {
  process.on("uncaughtException", (error: Error) => {
    sendPlayerEvent("uncaught-exception", error.message, {
      stack: error.stack,
      source: "node-uncaught-exception",
    });
  });

  process.on("unhandledRejection", (reason: any, promise: Promise<any>) => {
    const message = reason instanceof Error ? reason.message : String(reason);
    const stack = reason instanceof Error ? reason.stack : undefined;

    sendPlayerEvent(
      "unhandled-rejection",
      `Unhandled Promise Rejection: ${message}`,
      {
        stack,
        source: "node-unhandled-rejection",
        reason: reason instanceof Error ? undefined : reason,
      },
    );
  });
}

// Hook into uncaught errors (Browser)
if (typeof window !== "undefined" && Platform.OS === "web") {
  window.addEventListener("error", (event: ErrorEvent) => {
    sendPlayerEvent("javascript-error", event.message, {
      stack: event.error?.stack,
      source: "browser-window-error",
      filename: event.filename,
      lineno: event.lineno,
      colno: event.colno,
    });
  });

  window.addEventListener(
    "unhandledrejection",
    (event: PromiseRejectionEvent) => {
      const message =
        event.reason instanceof Error
          ? event.reason.message
          : String(event.reason);
      const stack =
        event.reason instanceof Error ? event.reason.stack : undefined;

      sendPlayerEvent(
        "unhandled-rejection",
        `Unhandled Promise Rejection: ${message}`,
        {
          stack,
          source: "browser-unhandled-rejection",
          reason: event.reason instanceof Error ? undefined : event.reason,
        },
      );
    },
  );
}

// Report console errors and warnings
const originalError = console.error;
const originalWarn = console.warn;

console.error = (...args: any[]) => {
  const message = args.join(" ");
  sendPlayerEvent("console-error", message, {
    source: "console-error",
    args: args,
  });
  originalError(...args);
};

console.warn = (...args: any[]) => {
  const message = args.join(" ");
  sendPlayerEvent("console-warning", message, {
    source: "console-warning",
    args: args,
  });
  originalWarn(...args);
};
