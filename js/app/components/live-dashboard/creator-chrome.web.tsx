import {
  getFocusedRouteNameFromRoute,
  useNavigation,
  useNavigationState,
} from "@react-navigation/native";
import { Text, useTheme, zero } from "@streamplace/components";
import { breakpoints } from "@streamplace/components/src/lib/theme/tokens";
import SidebarItem from "components/sidebar/sidebar-item";
import {
  ArrowLeft,
  ArrowUpFromLine,
  Key,
  LayoutGrid,
  ListVideo,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
  Radio,
  Share2,
  Video,
  Webhook,
  X,
} from "lucide-react-native";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from "react";
import { useTranslation } from "react-i18next";
import { useWindowDimensions, View } from "react-native";
import { getStreamplaceNavigationAction } from "src/linking-config";
import { useStore } from "store";

const mainLinks = [
  { labelKey: "control-panel", href: "/dashboard", icon: LayoutGrid },
  { labelKey: "upload-videos", href: "/upload", icon: ArrowUpFromLine },
  { labelKey: "my-videos", href: "/upload/videos", icon: Video },
];
const settingsLinks = [
  { labelKey: "stream-settings", href: "/dashboard/stream", icon: Radio },
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

const CreatorTheatreContext = createContext<{
  theatre: boolean;
  setTheatre: (value: boolean) => void;
  entryRef: RefObject<HTMLButtonElement | null>;
} | null>(null);

export function useCreatorTheatre() {
  const context = useContext(CreatorTheatreContext);
  if (!context) throw new Error("Creator theatre requires creator chrome");
  return context;
}

export default function CreatorChrome({ children }: { children: ReactNode }) {
  const { theme } = useTheme();
  const { t } = useTranslation("settings");
  const navigation = useNavigation();
  const currentScreen = useNavigationState((state) => {
    const route = state.routes[state.index];
    return (
      getFocusedRouteNameFromRoute(route) ??
      (route.params as { screen?: string } | undefined)?.screen
    );
  });
  const currentHref =
    currentScreen === "CreatorStreamSettings"
      ? "/dashboard/stream"
      : "/dashboard";
  const { width } = useWindowDimensions();
  const narrow = width < breakpoints.md;
  const collapsed = useStore((state) => state.isCollapsed);
  const toggleSidebar = useStore((state) => state.toggleSidebar);
  const [open, setOpen] = useState(false);
  const entryRef = useRef<HTMLButtonElement>(null);
  const exitRef = useRef<HTMLButtonElement>(null);
  const moveFocus = useRef(false);
  const [theatre, setTheatreState] = useState(() => {
    try {
      return localStorage.getItem("streamplace:theatre") === "true";
    } catch {
      return false;
    }
  });
  const setTheatre = useCallback((value: boolean) => {
    moveFocus.current = true;
    setTheatreState(value);
    try {
      localStorage.setItem("streamplace:theatre", String(value));
    } catch {
      // The current session still works if storage is unavailable.
    }
  }, []);
  const openButton = useRef<HTMLButtonElement>(null);
  const closeButton = useRef<HTMLButtonElement>(null);
  const nav = useRef<HTMLElement>(null);

  useEffect(() => {
    if (moveFocus.current) {
      (theatre ? exitRef : entryRef).current?.focus();
      moveFocus.current = false;
    }
  }, [theatre]);

  useEffect(() => {
    if (!theatre) return;
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented) return;
      event.preventDefault();
      setTheatre(false);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [theatre, setTheatre]);

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
          active={item.href === currentHref}
          onPress={(event) => {
            event.preventDefault();
            if (item.href !== currentHref) navigate(item.href);
            else if (narrow) close();
          }}
        />
      </div>
    ));

  const chrome = (
    <View style={{ flex: 1, flexDirection: "row", minHeight: 0 }}>
      {!theatre && narrow && open && (
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
      {!theatre && (!narrow || open) && (
        <nav
          ref={nav}
          id="creator-navigation"
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
            aria-expanded={narrow ? undefined : !collapsed}
            aria-controls={narrow ? undefined : "creator-navigation"}
            title={
              narrow
                ? "Close creator navigation"
                : collapsed
                  ? "Expand creator navigation"
                  : "Collapse creator navigation"
            }
            onClick={() => (narrow ? close() : toggleSidebar())}
            style={{ ...buttonStyle, alignSelf: "flex-start" }}
          >
            {narrow ? (
              <X size={theme.spacing[6]} />
            ) : collapsed ? (
              <PanelLeftOpen size={theme.spacing[6]} />
            ) : (
              <PanelLeftClose size={theme.spacing[6]} />
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
      <View
        style={{ flex: 1, minWidth: 0, minHeight: 0, position: "relative" }}
      >
        {theatre && (
          <button
            ref={exitRef}
            type="button"
            aria-label="Exit theatre mode"
            onClick={() => setTheatre(false)}
            style={{
              ...buttonStyle,
              position: "absolute",
              top: theme.spacing[2],
              right: theme.spacing[2],
              zIndex: zero.atoms.zIndex[10].zIndex,
              padding: `0 ${theme.spacing[3]}px`,
              gap: theme.spacing[2],
              background: theme.colors.surface2,
              color: theme.colors.text1,
              ...theme.typography.universal.xs,
              lineHeight: `${theme.typography.universal.xs.lineHeight}px`,
            }}
          >
            <X size={theme.spacing[4]} aria-hidden />
            Exit theatre
          </button>
        )}
        {!theatre && narrow && (
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
  return (
    <CreatorTheatreContext.Provider value={{ theatre, setTheatre, entryRef }}>
      {chrome}
    </CreatorTheatreContext.Provider>
  );
}
