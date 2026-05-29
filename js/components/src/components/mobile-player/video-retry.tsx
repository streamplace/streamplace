import React, { useEffect, useRef, useState } from "react";
import { PlayerStatus, usePlayerStore } from "../..";

export default function VideoRetry(props: { children: React.ReactNode }) {
  const retryTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const [retries, setRetries] = useState(0);
  const playing = usePlayerStore((x) => x.status === PlayerStatus.PLAYING);
  const mode = usePlayerStore((x) => x.mode);
  const [lastChange, setLastChange] = useState(Date.now());
  const [tick, setTick] = useState(0);
  const status = usePlayerStore((x) => x.status);

  useEffect(() => {
    setLastChange(Date.now());
  }, [status]);

  useEffect(() => {
    if (mode === "vod") return;
    if (!playing) {
      const handle = setInterval(() => {
        setTick(Date.now());
      }, 1000);
      return () => clearInterval(handle);
    }
  }, [setTick, playing]);

  const stalledFor5sec = !playing && tick - lastChange > 5000;

  useEffect(() => {
    if (!playing) {
      const doRetry = () => {
        console.log("Retrying video playback...");
        setRetries((prevRetries) => prevRetries + 1);
      };
      const jitter = 2000 + Math.random() * 1500;
      retryTimeoutRef.current = setTimeout(doRetry, jitter);
    }

    return () => {
      if (retryTimeoutRef.current) {
        console.log("Clearing retry timeout");
        clearTimeout(retryTimeoutRef.current);
        retryTimeoutRef.current = null;
      }
    };
  }, [!playing, mode]);

  useEffect(() => {
    if (stalledFor5sec) {
      console.log("Stalled for 5 seconds, retrying...");
      setRetries((r) => r + 1);
    }
  }, [stalledFor5sec]);

  return <React.Fragment key={retries}>{props.children}</React.Fragment>;
}
