import React, { useEffect, useRef, useState } from "react";
import { usePlayerStore } from "../..";

export default function VideoRetry(props: { children: React.ReactNode }) {
  const retryTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const [retries, setRetries] = useState(0);
  const [hasStarted, setHasStarted] = useState(false);

  const offline = usePlayerStore((x) => x.offline);

  useEffect(() => {
    if (!offline && !hasStarted) {
      console.log("Player is online. Marking as started.");
      setHasStarted(true);
    }

    if (offline) {
      console.log("Player is offline. Incrementing retries.");
      setRetries((prevRetries) => prevRetries + 1);

      const jitter = 500 + Math.random() * 1500;
      retryTimeoutRef.current = setTimeout(() => {
        console.log("Retrying video playback...");
      }, jitter);
    }

    return () => {
      if (retryTimeoutRef.current) {
        console.log("Clearing retry timeout");
        clearTimeout(retryTimeoutRef.current);
      }
    };
  }, [offline, hasStarted]);

  return <React.Fragment key={retries}>{props.children}</React.Fragment>;
}
