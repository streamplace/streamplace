import { useNavigation } from "@react-navigation/native";
import { Text, useTheme } from "@streamplace/components";
import { breakpoints } from "@streamplace/components/src/lib/theme/tokens";
import SidebarItem from "components/sidebar/sidebar-item";
import {
  ArrowLeft,
  ArrowUpFromLine,
  Key,
  LayoutGrid,
  ListVideo,
  Menu,
  Radio,
  Share2,
  Video,
  Webhook,
  X,
} from "lucide-react-native";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useWindowDimensions, View } from "react-native";
import { getStreamplaceNavigationAction } from "src/linking-config";

const mainLinks = [
  { labelKey: "control-panel", href: "/dashboard", icon: LayoutGrid },
  { labelKey: "upload-videos", href: "/upload", icon: ArrowUpFromLine },
  { labelKey: "my-videos", href: "/upload/videos", icon: Video },
];
const settingsLinks = [
  { labelKey: "stream-settings", href: "/settings/streaming", icon: Radio },
  {
    labelKey: "key-manager",
    href: "/settings/streaming/key-management",
    icon: Key,
  },
  {
    labelKey: "multistream",
    href: "/settings/streaming/multistream",
    icon: Share2,
  },
  {
    labelKey: "recommendations-to-others",
    href: "/settings/streaming/recommendations",
    icon: ListVideo,
  },
  { labelKey: "webhooks", href: "/settings/streaming/webhooks", icon: Webhook },
];

export default function CreatorChrome({ children }: { children: ReactNode }) {
  const { theme } = useTheme();
  const { t } = useTranslation("settings");
  const navigation = useNavigation();
  const { width } = useWindowDimensions();
  const narrow = width < breakpoints.md;
  const [collapsed, setCollapsed] = useState(false);
  const [open, setOpen] = useState(false);
  const openButton = useRef<HTMLButtonElement>(null);
  const closeButton = useRef<HTMLButtonElement>(null);
  const nav = useRef<HTMLElement>(null);

  useEffect(() => {
    if (narrow && open) closeButton.current?.focus();
  }, [narrow, open]);

  useEffect(() => {
    if (!narrow) setOpen(false);
  }, [narrow]);

  const close = () => {
    setOpen(false);
    openButton.current?.focus();
  };
  const navigate = (href: string) => {
    const action = getStreamplaceNavigationAction(href);
    if (action) navigation.dispatch(action);
    setOpen(false);
  };
  const buttonStyle = {
    minWidth: theme.touchTargets.minimum,
    minHeight: theme.touchTargets.minimum,
    display: "inline-flex",
    alignItems: "center",
    justifyContent: "center",
    border: 0,
    borderRadius: theme.borderRadius.md,
    background: "transparent",
    color: theme.colors.text2,
    cursor: "pointer",
  } as const;
  const links = (items: typeof mainLinks) =>
    items.map((item) => (
      <div
        key={item.href}
        title={!narrow && collapsed ? t(item.labelKey) : undefined}
      >
        <SidebarItem
          icon={item.icon}
          href={item.href}
          label={t(item.labelKey)}
          collapsed={!narrow && collapsed}
          active={item.href === "/dashboard"}
          onPress={(event) => {
            event.preventDefault();
            if (item.href !== "/dashboard") navigate(item.href);
            else if (narrow) close();
          }}
        />
      </div>
    ));

  return (
    <View style={{ flex: 1, flexDirection: "row", minHeight: 0 }}>
      {narrow && open && (
        <button
          type="button"
          aria-label="Dismiss creator navigation"
          tabIndex={-1}
          onClick={close}
          style={{
            position: "fixed",
            inset: 0,
            border: 0,
            background: theme.colors.overlay,
            zIndex: 10,
          }}
        />
      )}
      {(!narrow || open) && (
        <nav
          ref={nav}
          data-testid="creator-navigation"
          aria-label="Creator navigation"
          role={narrow ? "dialog" : undefined}
          aria-modal={narrow ? true : undefined}
          onKeyDown={(event) => {
            if (!narrow) return;
            if (event.key === "Escape") {
              event.preventDefault();
              close();
            } else if (event.key === "Tab") {
              const focusable = nav.current?.querySelectorAll<HTMLElement>(
                "a[href], button:not([disabled])",
              );
              if (!focusable?.length) return;
              const first = focusable[0];
              const last = focusable[focusable.length - 1];
              if (event.shiftKey && document.activeElement === first) {
                event.preventDefault();
                last.focus();
              } else if (!event.shiftKey && document.activeElement === last) {
                event.preventDefault();
                first.focus();
              }
            }
          }}
          style={{
            display: "flex",
            flexDirection: "column",
            flexShrink: 0,
            width:
              narrow || !collapsed ? theme.spacing[16] * 4 : theme.spacing[16],
            padding: theme.spacing[2],
            boxSizing: "border-box",
            overflowY: "auto",
            background: theme.colors.surface0,
            borderRight: `1px solid ${theme.colors.borderSubtle}`,
            ...(narrow
              ? { position: "fixed", top: 0, bottom: 0, left: 0, zIndex: 11 }
              : {}),
          }}
        >
          <button
            ref={closeButton}
            type="button"
            aria-label={
              narrow
                ? "Close creator navigation"
                : collapsed
                  ? "Expand creator navigation"
                  : "Collapse creator navigation"
            }
            onClick={() => (narrow ? close() : setCollapsed(!collapsed))}
            style={{ ...buttonStyle, alignSelf: "flex-start" }}
          >
            {narrow ? (
              <X size={theme.spacing[6]} />
            ) : (
              <Menu size={theme.spacing[6]} />
            )}
          </button>
          <div title={!narrow && collapsed ? "Back to Streamplace" : undefined}>
            <SidebarItem
              icon={ArrowLeft}
              label="Back to Streamplace"
              href="/"
              collapsed={!narrow && collapsed}
              active={false}
              onPress={(event) => {
                event.preventDefault();
                navigate("/");
              }}
            />
          </div>
          <View style={{ marginTop: theme.spacing[4] }}>
            {links(mainLinks)}
          </View>
          {(narrow || !collapsed) && (
            <Text
              color="muted"
              size="sm"
              style={{
                marginTop: theme.spacing[6],
                marginHorizontal: theme.spacing[3],
                marginBottom: theme.spacing[2],
              }}
            >
              {t("settings-group", { defaultValue: "Settings" })}
            </Text>
          )}
          <View>{links(settingsLinks)}</View>
        </nav>
      )}
      <View style={{ flex: 1, minWidth: 0, minHeight: 0 }}>
        {narrow && (
          <View
            style={{
              flexDirection: "row",
              alignItems: "center",
              gap: theme.spacing[2],
              paddingHorizontal: theme.spacing[2],
              borderBottomWidth: 1,
              borderBottomColor: theme.colors.borderSubtle,
              backgroundColor: theme.colors.surface0,
            }}
          >
            <button
              ref={openButton}
              type="button"
              aria-label="Open creator navigation"
              aria-expanded={open}
              onClick={() => setOpen(true)}
              style={buttonStyle}
            >
              <Menu size={theme.spacing[6]} />
            </button>
            <Text weight="medium">Creator dashboard</Text>
          </View>
        )}
        {children}
      </View>
    </View>
  );
}
