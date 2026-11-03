import {
  Dashboard,
  useLivestreamStore,
  useProfile,
  useTheme,
  zero,
} from "@streamplace/components";
import {
  ProblemsWrapper,
  ProblemsWrapperRef,
} from "@streamplace/components/src/components/dashboard/problems";
import { breakpoints } from "@streamplace/components/src/lib/theme/tokens";
import { EmojiPicker } from "components/emoji-picker/emoji-picker";
import {
  Activity,
  AlertTriangle,
  Check,
  CircleHelp,
  Globe,
  MessageSquare,
  Plus,
  Radio,
  Shield,
  SlidersHorizontal,
  Video,
  X,
} from "lucide-react-native";
import { useRef, useState } from "react";
import { useWindowDimensions, View } from "react-native";
import { useEmojiData } from "utils/emoji";
import CreatorHealth from "./creator-health.web";
import {
  getWidgetPlacements,
  insertWidget,
  removeWidget,
  type LayoutNode,
  type WidgetNode,
} from "./creator-layout";
import { loadLayout, saveLayout } from "./creator-layout-storage";
import { useCreatorMetrics } from "./creator-metrics-provider.web";
import LivestreamPanel from "./livestream-panel";
import MultistreamStatus from "./multistream-status";
import StreamMonitor from "./stream-monitor";

const widgets = new Map([
  ["stream-monitor", { title: "Stream Monitor", icon: Video }],
  ["stream-health", { title: "Stream Health", icon: Activity }],
  ["chat", { title: "Chat", icon: MessageSquare }],
  ["stream-info", { title: "Stream Info", icon: Radio }],
  ["multistream", { title: "Multistream", icon: Globe }],
  ["problems", { title: "Problems", icon: AlertTriangle }],
  ["moderators", { title: "Moderators", icon: Shield }],
]);

export default function CreatorDashboard({ videoRef }: { videoRef: any }) {
  const { theme } = useTheme();
  const { width, height } = useWindowDimensions();
  const stacked = width < breakpoints.xl;
  const { status, history } = useCreatorMetrics();
  const profile = useProfile();
  const problems = useLivestreamStore((state) => state.problems);
  const emojiData = useEmojiData();
  const problemsRef = useRef<ProblemsWrapperRef>(null);
  const addRef = useRef<HTMLButtonElement>(null);
  const widgetIds = useRef(new WeakMap<WidgetNode, number>());
  const nextId = useRef(0);
  const [layout, setLayout] = useState(loadLayout);
  const [editing, setEditing] = useState(false);
  const [adding, setAdding] = useState(false);
  const [saveFailed, setSaveFailed] = useState(false);
  const [announcement, setAnnouncement] = useState("");
  const messagesPerMinute = history[history.length - 1]?.chatRate ?? null;
  const spacing = theme.spacing[4];
  const placements = getWidgetPlacements(layout);
  const available = [...widgets].filter(
    ([key]) => !placements.some(({ widget }) => widget.widget === key),
  );
  const buttonStyle = {
    minHeight: theme.touchTargets.minimum,
    padding: `0 ${theme.spacing[3]}px`,
    border: `${zero.borders.width.thin.borderWidth}px solid ${theme.colors.border}`,
    borderRadius: theme.borderRadius.md,
    color: theme.colors.text1,
    background: theme.colors.surface2,
    font: "inherit",
    cursor: "pointer",
    display: "inline-flex",
    alignItems: "center",
    gap: theme.spacing[2],
  };

  function updateLayout(next: LayoutNode, message: string) {
    setLayout(next);
    setSaveFailed(!saveLayout(next));
    setAnnouncement(message);
  }

  function remove(path: number[], label: string) {
    updateLayout(removeWidget(layout, path), `${label} removed`);
    requestAnimationFrame(() => addRef.current?.focus());
  }

  function add(key: string) {
    const widget: WidgetNode = { type: "leaf", widget: key };
    const target = placements
      .filter(({ width, height }) => width > 0 && height > 0)
      .at(-1)!;
    const next = insertWidget(layout, target.path, widget, "vertical", false);
    updateLayout(next, `${widgets.get(key)?.title} added`);
    setAdding(false);
    requestAnimationFrame(() => addRef.current?.focus());
  }

  function renderWidget(key: string) {
    switch (key) {
      case "stream-monitor":
        return (
          <StreamMonitor
            status={status}
            userProfile={profile}
            videoRef={videoRef}
          />
        );
      case "stream-health":
        return (
          <CreatorHealth
            onShowProblems={() => problemsRef.current?.setDismiss(false)}
          />
        );
      case "chat":
        return (
          <Dashboard.ChatPanel
            showTitle={false}
            isLive={status.phase === "live"}
            isConnected={status.incoming !== null}
            messagesPerMinute={messagesPerMinute}
            emojiData={emojiData}
            emojiPicker={(isOpen, onClose, onSelect) => (
              <EmojiPicker
                isOpen={isOpen}
                onClose={onClose}
                onSelect={onSelect}
                customEmoji={[]}
              />
            )}
          />
        );
      case "stream-info":
        return (
          <LivestreamPanel
            status={status}
            scrollable={!stacked}
            showModerators={false}
          />
        );
      case "multistream":
        return <MultistreamStatus />;
      case "moderators":
        return (
          <View testID="creator-moderators" style={{ flex: 1, minHeight: 0 }}>
            <Dashboard.ModeratorPanel
              isLive={status.incoming === true}
              showTitle={false}
            />
          </View>
        );
      case "problems":
        return (
          <div
            style={{
              padding: spacing,
              color: theme.colors.text1,
              background: theme.colors.surface1,
              borderRadius: theme.borderRadius.lg,
            }}
          >
            <strong>Problems</strong>
            <p>
              {problems.length
                ? `${problems.length} ${problems.length === 1 ? "issue" : "issues"}`
                : "No current problems"}
            </p>
            {problems.length > 0 && (
              <button
                type="button"
                style={buttonStyle}
                onClick={() => problemsRef.current?.setDismiss(false)}
              >
                View problems
              </button>
            )}
          </div>
        );
      case "blank":
        return (
          <div style={{ padding: spacing, color: theme.colors.text2 }}>
            Add a widget to your dashboard.
          </div>
        );
      default:
        return (
          <div style={{ padding: spacing, color: theme.colors.text2 }}>
            Unknown widget: {key}
          </div>
        );
    }
  }

  return (
    <ProblemsWrapper ref={problemsRef}>
      <div
        className="creator-dashboard"
        style={{
          display: "flex",
          flexDirection: "column",
          flex: 1,
          minHeight: 0,
          background: theme.colors.surface0,
          color: theme.colors.text1,
          ...theme.typography.universal.xs,
          lineHeight: `${theme.typography.universal.xs.lineHeight}px`,
        }}
      >
        <style>{`.creator-dashboard button:focus-visible { outline: ${zero.borders.width.medium.borderWidth}px solid ${theme.colors.focus}; outline-offset: ${zero.borders.width.medium.borderWidth}px; }`}</style>
        <div
          role="toolbar"
          aria-label="Creator dashboard controls"
          style={{
            display: "flex",
            flexWrap: "wrap",
            alignItems: "center",
            gap: theme.spacing[2],
            padding: `${theme.spacing[2]}px ${spacing}px`,
            background: theme.colors.surface1,
            borderBottom: `${zero.borders.width.thin.borderWidth}px solid ${theme.colors.borderSubtle}`,
          }}
        >
          <button
            type="button"
            style={buttonStyle}
            onClick={() => {
              setEditing(!editing);
              setAdding(false);
            }}
          >
            <span aria-hidden="true">
              {editing ? (
                <Check size={theme.spacing[4]} />
              ) : (
                <SlidersHorizontal size={theme.spacing[4]} />
              )}
            </span>
            {editing ? "Done customizing" : "Customize dashboard"}
          </button>
          {editing && (
            <button
              ref={addRef}
              type="button"
              style={buttonStyle}
              aria-expanded={adding}
              onClick={() => setAdding(!adding)}
            >
              <span aria-hidden="true">
                <Plus size={theme.spacing[4]} />
              </span>
              Add widget
            </button>
          )}
          {editing &&
            placements
              .filter(
                ({ widget, width, height }) =>
                  widget.widget !== "blank" && (width === 0 || height === 0),
              )
              .map(({ widget, path }) => (
                <button
                  key={path.join(".")}
                  type="button"
                  style={buttonStyle}
                  onClick={() =>
                    remove(
                      path,
                      widgets.get(widget.widget)?.title ?? widget.widget,
                    )
                  }
                >
                  Remove {widgets.get(widget.widget)?.title ?? widget.widget}
                </button>
              ))}
        </div>
        {editing && adding && (
          <div
            role="group"
            aria-label="Available widgets"
            onKeyDown={(event) => {
              if (event.key === "Escape") {
                setAdding(false);
                addRef.current?.focus();
              }
            }}
            style={{
              display: "flex",
              flexWrap: "wrap",
              gap: theme.spacing[2],
              padding: `${theme.spacing[2]}px ${spacing}px`,
            }}
          >
            {available.length ? (
              available.map(([key, { title, icon: Icon }]) => (
                <button
                  key={key}
                  type="button"
                  style={buttonStyle}
                  onClick={() => add(key)}
                >
                  <span aria-hidden="true">
                    <Icon size={theme.spacing[4]} />
                  </span>
                  Add {title}
                </button>
              ))
            ) : (
              <span>All widgets added</span>
            )}
            {available.length > 0 && (
              <span style={{ alignSelf: "center", color: theme.colors.text2 }}>
                New widgets appear after the last widget.
              </span>
            )}
          </div>
        )}
        {saveFailed && (
          <div
            role="alert"
            style={{
              padding: `${theme.spacing[2]}px ${spacing}px`,
              color: theme.colors.warning,
            }}
          >
            Could not save your dashboard layout. Changes will last until you
            leave this page.
          </div>
        )}
        <span
          aria-live="polite"
          style={{
            position: "absolute",
            width: 1,
            height: 1,
            overflow: "hidden",
            clipPath: "inset(50%)",
          }}
        >
          {announcement}
        </span>
        <div
          style={{ flex: 1, minHeight: 0, overflow: "auto", padding: spacing }}
        >
          <div
            style={{
              position: "relative",
              display: stacked ? "flex" : "block",
              flexDirection: "column",
              gap: spacing,
              height: stacked ? undefined : "100%",
              minHeight: 0,
            }}
          >
            {placements.map(
              ({
                widget,
                path,
                x,
                y,
                width: widgetWidth,
                height: widgetHeight,
              }) => {
                if (!widgetIds.current.has(widget))
                  widgetIds.current.set(widget, nextId.current++);
                const meta = widgets.get(widget.widget);
                const label =
                  meta?.title ??
                  (widget.widget === "blank" ? "Empty" : widget.widget);
                const Icon = meta?.icon ?? CircleHelp;
                const stackedHeight =
                  widget.widget === "stream-monitor"
                    ? Math.min(height * 0.45, width * 0.65)
                    : widget.widget === "chat"
                      ? theme.spacing[16] * 8
                      : widget.widget === "moderators"
                        ? theme.spacing[16] * 4
                        : undefined;
                return (
                  <section
                    key={widgetIds.current.get(widget)}
                    aria-label={`${label} widget`}
                    style={{
                      display: "flex",
                      flexDirection: "column",
                      minWidth: 0,
                      minHeight:
                        stacked && widget.widget === "stream-health"
                          ? theme.spacing[16] * 4
                          : 0,
                      ...(stacked
                        ? { height: stackedHeight }
                        : {
                            position: "absolute",
                            left: `calc(${x * 100}% + ${spacing / 2}px)`,
                            top: `calc(${y * 100}% + ${spacing / 2}px)`,
                            width: `calc(${widgetWidth * 100}% - ${spacing}px)`,
                            height: `calc(${widgetHeight * 100}% - ${spacing}px)`,
                          }),
                    }}
                  >
                    <div
                      style={{
                        display: "flex",
                        alignItems: "center",
                        gap: theme.spacing[2],
                        minHeight: theme.spacing[8],
                        background: theme.colors.surface1,
                        border: `${zero.borders.width.thin.borderWidth}px solid ${theme.colors.borderSubtle}`,
                        borderRadius: `${theme.borderRadius.md}px ${theme.borderRadius.md}px 0 0`,
                        paddingLeft: theme.spacing[3],
                        color: theme.colors.text2,
                      }}
                    >
                      <span aria-hidden="true">
                        <Icon size={theme.spacing[4]} />
                      </span>
                      <strong style={{ flex: 1, color: theme.colors.text1 }}>
                        {label}
                      </strong>
                      {editing && widget.widget !== "blank" && (
                        <button
                          type="button"
                          aria-label={`Remove ${label}`}
                          style={{
                            ...buttonStyle,
                            border: 0,
                            background: "transparent",
                            padding: `0 ${theme.spacing[3]}px`,
                          }}
                          onClick={() => remove(path, label)}
                        >
                          <span aria-hidden="true">
                            <X size={theme.spacing[4]} />
                          </span>
                        </button>
                      )}
                    </div>
                    <div
                      style={{
                        display: "flex",
                        flexDirection: "column",
                        flex: 1,
                        minHeight: 0,
                        minWidth: 0,
                        overflow: stacked ? "visible" : "auto",
                      }}
                    >
                      {renderWidget(widget.widget)}
                    </div>
                  </section>
                );
              },
            )}
          </div>
        </div>
      </div>
    </ProblemsWrapper>
  );
}
