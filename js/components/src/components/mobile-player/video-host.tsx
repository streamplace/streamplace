import {
  createContext,
  ReactNode,
  useContext,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { LivestreamContext } from "../../livestream-store/context";
import { PlayerContext } from "../../player-store/context";
import { PlayerStore, usePlayerContext } from "../../player-store/player-store";
import { VideoContext } from "../../video-store/context";

// A player's video (the <video> element and the hls.js or WebRTC session
// feeding it) is rendered here, near the top of the app, instead of wherever
// the layout puts the player. The layout renders a <HostedVideo> slot and the
// video's DOM node is moved into it, so when a window resize moves the player
// to another branch of the layout, the new slot picks up the same playing
// video instead of loading it again.

type Host = {
  attach(store: PlayerStore, box: HTMLElement): void;
  detach(store: PlayerStore, box: HTMLElement): void;
  render(store: PlayerStore, content: ReactNode): void;
};

type Hosted = {
  key: string;
  container: HTMLDivElement;
  release?: ReturnType<typeof setTimeout>;
};

const VideoHostContext = createContext<Host | null>(null);

let nextKey = 0;

export function VideoHostProvider({ children }: { children: ReactNode }) {
  const parking = useRef<HTMLDivElement>(null);
  const [hosted] = useState(() => new Map<PlayerStore, Hosted>());
  const [contents, setContents] = useState<ReadonlyMap<PlayerStore, ReactNode>>(
    () => new Map(),
  );

  const host = useMemo<Host>(
    () => ({
      attach(store, box) {
        let h = hosted.get(store);
        if (!h) {
          const container = document.createElement("div");
          container.style.width = "100%";
          container.style.height = "100%";
          h = { key: String(nextKey++), container };
          hosted.set(store, h);
        }
        clearTimeout(h.release);
        box.appendChild(h.container);
      },
      detach(store, box) {
        const h = hosted.get(store);
        if (!h || h.container.parentNode !== box) return;
        // A <video> moved within the document keeps playing. A slot that
        // remounted elsewhere attaches it again in the same commit, before
        // this fires; otherwise the player is gone.
        parking.current?.appendChild(h.container);
        h.release = setTimeout(() => {
          hosted.delete(store);
          h.container.remove();
          setContents((prev) => {
            const next = new Map(prev);
            next.delete(store);
            return next;
          });
        });
      },
      render(store, content) {
        setContents((prev) => new Map(prev).set(store, content));
      },
    }),
    [hosted],
  );

  return (
    <VideoHostContext.Provider value={host}>
      {children}
      <div ref={parking} style={{ display: "none" }} />
      {[...contents].map(([store, content]) => {
        const h = hosted.get(store);
        return h && createPortal(content, h.container, h.key);
      })}
    </VideoHostContext.Provider>
  );
}

// Shows the current player's video, rendered by the VideoHostProvider when
// the app has one, or in place when it doesn't.
export function HostedVideo({ children }: { children: ReactNode }) {
  const host = useContext(VideoHostContext);
  const players = usePlayerContext();
  const livestream = useContext(LivestreamContext);
  const video = useContext(VideoContext);
  const store = players.players[Object.keys(players.players)[0]];
  const box = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    if (!host) return;
    const el = box.current!;
    host.attach(store, el);
    return () => host.detach(store, el);
  }, [host, store]);

  useLayoutEffect(() => {
    // The host is above the screen's stores; pass their contexts along.
    host?.render(
      store,
      <PlayerContext.Provider value={players}>
        <LivestreamContext.Provider value={livestream}>
          <VideoContext.Provider value={video}>
            {children}
          </VideoContext.Provider>
        </LivestreamContext.Provider>
      </PlayerContext.Provider>,
    );
  }, [host, store, players, livestream, video, children]);

  if (!host) return <>{children}</>;
  return <div ref={box} style={{ width: "100%", height: "100%" }} />;
}
