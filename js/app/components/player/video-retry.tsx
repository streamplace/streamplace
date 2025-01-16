import React, { useEffect, useState } from "react";
import { PlayerProps, PlayerStatus } from "./props";

export default function VideoRetry(
  props: PlayerProps & { children: React.ReactNode },
) {
  const [resetTime, setResetTime] = useState<number>(Date.now());
  const [retryCount, setRetryCount] = useState(0);
  const isPlaying = props.status === PlayerStatus.PLAYING;

  useEffect(() => {
    if (isPlaying) {
      setRetryCount(0);
      return;
    }

    const baseDelay = 10000; // 10 seconds
    const maxDelay = 30000; // 30 seconds
    const delay = Math.min(baseDelay * Math.pow(2, retryCount), maxDelay);

    const handle = setTimeout(() => {
      // console.log(`retrying (attempt ${retryCount + 1}, delay: ${delay}ms)`);
      setResetTime(Date.now());
      setRetryCount((prev) => prev + 1);
    }, delay);

    return () => clearTimeout(handle);
  }, [isPlaying, resetTime, retryCount]);

  return <React.Fragment key={resetTime}>{props.children}</React.Fragment>;
}
