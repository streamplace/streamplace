import React, { useEffect, useState } from "react";
import { PlayerProps, PlayerStatus } from "./props";

export default function VideoRetry(
  props: PlayerProps & { children: React.ReactNode },
) {
  const [resetTime, setResetTime] = useState<number>(Date.now());
  const isPlaying = props.status === PlayerStatus.PLAYING;
  useEffect(() => {
    if (isPlaying) {
      return;
    }
    const handle = setTimeout(() => {
      // you've had long enough. try again!
      setResetTime(Date.now());
    }, 5000);
    return () => clearTimeout(handle);
  }, [isPlaying]);
  return <React.Fragment key={resetTime}>{props.children}</React.Fragment>;
}
