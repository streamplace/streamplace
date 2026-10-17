import { Text, toast, useTheme } from "@streamplace/components";
import { shadows } from "@streamplace/components/src/lib/theme/tokens";
import { Type } from "lucide-react-native";
import { type ReactNode, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Pressable, View } from "react-native";
import {
  LogoMark,
  markSvgString,
  useCustomMark,
  wordmarkSvgString,
} from "./logo";

/**
 * Vercel-style right-click menu on the logo: copy the mark or wordmark as SVG,
 * or open the brand page. Web only — wraps the logo and opens on contextmenu.
 */
export function LogoBrandMenu({ children }: { children: ReactNode }) {
  const anchorRef = useRef<HTMLElement | null>(null);
  const [pos, setPos] = useState<{ x: number; y: number } | null>(null);
  const { theme } = useTheme();

  // Open at the logo's bottom-left on right-click, suppressing the browser menu.
  useEffect(() => {
    const node = anchorRef.current;
    if (!node) return;
    const onContext = (e: MouseEvent) => {
      e.preventDefault();
      const r = node.getBoundingClientRect();
      setPos({ x: r.left, y: r.bottom + 8 });
    };
    node.addEventListener("contextmenu", onContext);
    return () => node.removeEventListener("contextmenu", onContext);
  }, []);

  // Dismiss on any click, scroll, resize, or Escape.
  useEffect(() => {
    if (!pos) return;
    const close = () => setPos(null);
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && close();
    window.addEventListener("click", close);
    window.addEventListener("keydown", onKey);
    window.addEventListener("resize", close);
    window.addEventListener("scroll", close, true);
    return () => {
      window.removeEventListener("click", close);
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("resize", close);
      window.removeEventListener("scroll", close, true);
    };
  }, [pos]);

  const copy = async (svg: string, label: string) => {
    setPos(null);
    try {
      await navigator.clipboard.writeText(svg);
      toast.show(`${label} copied as SVG`, undefined, { variant: "success" });
    } catch {
      toast.show("Couldn't copy to clipboard", undefined, { variant: "error" });
    }
  };

  // A node's uploaded SVG logo is what gets copied; a raster upload has no
  // SVG to offer, so the item is dropped rather than copying the default.
  const custom = useCustomMark();
  const logoSvg = custom.svg ?? (custom.uri ? null : markSvgString());
  const items = [
    ...(logoSvg
      ? [
          {
            key: "logo",
            label: "Copy Logo as SVG",
            icon: <LogoMark size={18} color={theme.colors.text1} />,
            onPress: () => copy(logoSvg, "Logo"),
          },
        ]
      : []),
    {
      key: "wordmark",
      label: "Copy Wordmark as SVG",
      icon: <Type size={18} color={theme.colors.text2} />,
      onPress: () => copy(wordmarkSvgString(), "Wordmark"),
    },
    // {
    //   key: "guidelines",
    //   label: "Brand Guidelines",
    //   icon: <BookOpen size={18} color={theme.colors.text2} />,
    //   onPress: () => {
    //     setPos(null);
    //     Linking.openURL("https://staging.stp.lc/brand");
    //   },
    // },
  ];

  return (
    <>
      <View ref={anchorRef as any} style={{ flexShrink: 1, minWidth: 0 }}>
        {children}
      </View>
      {pos &&
        createPortal(
          <View
            style={[
              shadows.lg,
              {
                position: "fixed" as any,
                left: pos.x,
                top: pos.y,
                zIndex: 200000,
                minWidth: 244,
                padding: 6,
                gap: 2,
                backgroundColor: theme.colors.surface2,
                borderColor: theme.colors.borderSubtle,
                borderWidth: 1,
                borderRadius: theme.borderRadius.lg,
              },
            ]}
          >
            {items.map((it) => (
              <MenuItem
                key={it.key}
                label={it.label}
                icon={it.icon}
                onPress={it.onPress}
              />
            ))}
          </View>,
          document.body,
        )}
    </>
  );
}

function MenuItem({
  label,
  icon,
  onPress,
}: {
  label: string;
  icon: ReactNode;
  onPress: () => void;
}) {
  const { theme } = useTheme();
  const [hovered, setHovered] = useState(false);
  return (
    <Pressable
      onPress={onPress}
      onHoverIn={() => setHovered(true)}
      onHoverOut={() => setHovered(false)}
      style={{
        flexDirection: "row",
        alignItems: "center",
        gap: 12,
        paddingVertical: 8,
        paddingHorizontal: 10,
        borderRadius: theme.borderRadius.md,
        backgroundColor: hovered ? theme.colors.surface3 : "transparent",
      }}
    >
      <View style={{ width: 20, alignItems: "center" }}>{icon}</View>
      <Text size="sm" weight="medium">
        {label}
      </Text>
    </Pressable>
  );
}
