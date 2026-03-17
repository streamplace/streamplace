import React, { useEffect, useRef, useState } from "react";
import { PlayerStatus, usePlayerStore } from "../..";

export default function VideoRetry(props: { children: React.ReactNode }) {
  const retryTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const [retries, setRetries] = useState(0);
  const playing = usePlayerStore((x) => x.status === PlayerStatus.PLAYING);
  const mode = usePlayerStore((x) => x.mode);

  useEffect(() => {
    if (mode === "vod") return;
    if (!playing) {
      const jitter = 2000 + Math.random() * 1500;
      retryTimeoutRef.current = setTimeout(() => {
        console.log("Retrying video playback...");
        setRetries((prevRetries) => prevRetries + 1);
      }, jitter);
    }

    return () => {
      if (retryTimeoutRef.current) {
        console.log("Clearing retry timeout");
        clearTimeout(retryTimeoutRef.current);
        retryTimeoutRef.current = null;
      }
    };
  }, [!playing, mode]);

  return <React.Fragment key={retries}>{props.children}</React.Fragment>;
}
