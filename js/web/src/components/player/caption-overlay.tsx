import { cn } from "@/lib/utils";
import {
  CAPTION_WEB_FONTS,
  captionColor,
  captionEdgeShadows,
  captionFontSize,
  captionTextShadowCss,
  type CaptionDisplayPrefs,
} from "@streamplace/core";
import { useEffect, useRef, useState, type CSSProperties } from "react";

/**
 * Caption lines in the viewer's display settings. All colors here are the
 * viewer's own choices (@streamplace/core captions/prefs), not theme
 * tokens.
 */
export function CaptionLines({
  lines,
  prefs,
  fontSize,
}: {
  lines: string[];
  prefs: CaptionDisplayPrefs;
  fontSize: number | string;
}) {
  const textRef = useRef<HTMLDivElement | null>(null);
  const [measuredSize, setMeasuredSize] = useState(0);
  useEffect(() => {
    if (textRef.current && typeof fontSize === "string")
      setMeasuredSize(parseFloat(getComputedStyle(textRef.current).fontSize));
  }, [fontSize]);
  const lineStyle: CSSProperties = {
    font: "inherit",
    color: captionColor(prefs.textColor, prefs.textOpacity),
    backgroundColor: captionColor(
      prefs.backgroundColor,
      prefs.backgroundOpacity,
    ),
    textShadow: captionTextShadowCss(
      captionEdgeShadows(
        prefs,
        typeof fontSize === "number" ? fontSize : measuredSize,
      ),
    ),
    // Background behind each line's glyphs, padded at the line ends, like
    // broadcast captions.
    boxDecorationBreak: "clone",
    WebkitBoxDecorationBreak: "clone",
    padding: "0 0.25em",
  };
  return (
    <div
      ref={textRef}
      className="max-w-[90%] rounded-sm px-2 py-1 text-center"
      style={{
        backgroundColor: captionColor(prefs.windowColor, prefs.windowOpacity),
        fontFamily: CAPTION_WEB_FONTS[prefs.font],
        fontVariant: prefs.font === "smallCaps" ? "small-caps" : undefined,
        fontSize,
        lineHeight: 1.3,
      }}
    >
      <p
        data-testid="caption-overlay-text"
        className="m-0 whitespace-pre-wrap"
        style={{ font: "inherit" }}
      >
        {lines.map((line, i) => (
          <span key={i} style={{ font: "inherit" }}>
            {i > 0 && <br />}
            <span style={lineStyle}>{line}</span>
          </span>
        ))}
      </p>
    </div>
  );
}

/**
 * Draws caption lines over the player, sized from the player's height and
 * lifted above the control bar while it shows.
 */
export function CaptionOverlay({
  lines,
  prefs,
  raised,
}: {
  lines: string[];
  prefs: CaptionDisplayPrefs;
  raised: boolean;
}) {
  const ref = useRef<HTMLDivElement | null>(null);
  const [height, setHeight] = useState(0);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const observer = new ResizeObserver(() => setHeight(el.clientHeight));
    observer.observe(el);
    setHeight(el.clientHeight);
    return () => observer.disconnect();
  }, []);

  return (
    <div
      ref={ref}
      data-testid="caption-overlay"
      className={cn(
        "pointer-events-none absolute inset-0 z-10 flex flex-col items-center justify-end px-4 transition-[padding] duration-200 ease-in-out",
        raised ? "pb-16" : "pb-6",
      )}
    >
      {lines.length > 0 && (
        <CaptionLines
          lines={lines}
          prefs={prefs}
          fontSize={`max(var(--text-xs), ${captionFontSize(prefs, height, 0)}px)`}
        />
      )}
    </div>
  );
}
