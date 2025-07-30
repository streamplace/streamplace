import React, { useEffect } from "react";
import { usePlayerStore, useSegment, useStreamplaceStore } from "../..";

import { useRef } from "react";

export default function VideoRetry(props: { children: React.ReactNode }) {
  const lastSegmentRef = useRef<string | null>(null);
  const retryTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const segment = useSegment();

  const offline = usePlayerStore((x) => x.offline);
  const spurl = useStreamplaceStore((x) => x.url);

  useEffect(() => {
    if (
      lastSegmentRef.current !== null &&
      segment &&
      segment.startTime !== lastSegmentRef.current &&
      offline
    ) {
      const jitter = 500 + Math.random() * 1500;
      retryTimeoutRef.current = setTimeout(() => {
        lastSegmentRef.current = segment?.startTime;
      }, jitter);
    }
  }, [offline, segment, spurl]);

  return (
    <React.Fragment key={lastSegmentRef.current}>
      {props.children}
    </React.Fragment>
  );
}
