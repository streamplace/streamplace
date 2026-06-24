// Sentry capture helper.
//
// The web app initialises @sentry/react conditionally in main.tsx (only
// when VITE_SENTRY_DSN is set). When uninitialised, captureException is
// a no-op, so we don't need to gate the call ourselves.
//
// `captureError` is the one place code that catches a known error should
// route through when it wants the error to be reported. It builds a
// real Error so Sentry can group by stack, but preserves the original
// message and a caller-supplied context bag.
//
// `log.warn` / `log.error` are for one-off developer logs that aren't
// necessarily errors. They go to console only — use `captureError` if
// the message should reach Sentry.
import * as Sentry from "@sentry/react";

export function captureError(
  message: string,
  context?: Record<string, unknown>,
): void {
  if (import.meta.env.DEV) {
    // Don't spam Sentry with dev noise — console is enough.
    console.error(message, context);
    return;
  }
  const err = new Error(message);
  if (context) {
    Sentry.withScope((scope) => {
      scope.setExtras(context);
      Sentry.captureException(err);
    });
  } else {
    Sentry.captureException(err);
  }
}

export const log = {
  warn: (message: string, ...rest: unknown[]) => {
    console.warn(message, ...rest);
  },
  error: (message: string, ...rest: unknown[]) => {
    console.error(message, ...rest);
  },
};
