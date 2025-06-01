import { usePlayerSelectedRendition } from "features/player/playerSlice";
import React, { useEffect, useState } from "react";
import { useAppSelector } from "store/hooks";
import { PlayerProps, PlayerStatus } from "./props";

export default function VideoRetry(
  props: PlayerProps & { children: React.ReactNode },
) {
  const [resetTime, setResetTime] = useState<number>(Date.now());
  const [retryCount, setRetryCount] = useState(0);
  const isPlaying = props.status === PlayerStatus.PLAYING;
  const selectedRendition = useAppSelector(usePlayerSelectedRendition());

  useEffect(() => {
    if (isPlaying) {
      setRetryCount(0);
      return;
    }

    const baseDelay = 3000; // 3 seconds
    const maxDelay = 30000; // 30 seconds
    const delay = Math.min(baseDelay * Math.pow(2, retryCount), maxDelay);

    const handle = setTimeout(() => {
      // console.log(`retrying (attempt ${retryCount + 1}, delay: ${delay}ms)`);
      setResetTime(Date.now());
      setRetryCount((prev) => prev + 1);
      props.playerEvent(new Date().toISOString(), "retry", {
        delay,
      });
    }, delay);

    return () => clearTimeout(handle);
  }, [isPlaying, resetTime, retryCount]);

  return (
    <React.Fragment key={`${selectedRendition}-${resetTime}`}>
      {props.children}
    </React.Fragment>
  );
}
