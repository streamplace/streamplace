import {
  Accessibility,
  Feedback,
  type DragEndEvent,
  type DragStartEvent,
} from "@dnd-kit/dom";
import {
  DragDropProvider,
  useDraggable,
  type DragMoveEvent,
} from "@dnd-kit/react";
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
  GripVertical,
  MessageSquare,
  Plus,
  Radio,
  RectangleHorizontal,
  Shield,
  SlidersHorizontal,
  Video,
  type LucideIcon,
} from "lucide-react-native";
import {
  useCallback,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
  type PointerEvent,
} from "react";
import { useWindowDimensions, View, type ViewStyle } from "react-native";
import { useEmojiData } from "utils/emoji";
import { useCreatorTheatre } from "./creator-chrome.web";
import CreatorHealth from "./creator-health.web";
import {
  DEFAULT_LAYOUT,
  getDropSide,
  getLayoutGeometry,
  insertWidget,
  moveWidget,
  removeWidget,
  replaceWidget,
  resizeAdjacent,
  reuseWidgets,
  type LayoutNode,
  type SplitBoundary,
  type SplitNode,
  type WidgetNode,
} from "./creator-layout";
import {
  loadLayout,
  loadPresets,
  saveLayout,
  savePresets,
} from "./creator-layout-storage";
import { useCreatorMetrics } from "./creator-metrics-provider.web";
import CreatorWidgetFrame from "./creator-widget-frame.web";
import LivestreamPanel from "./livestream-panel";
import MultistreamStatus from "./multistream-status";
import StreamMonitor from "./stream-monitor";

const widgets = new Map([
  [
    "stream-monitor",
    { title: "Stream Monitor", icon: Video, minWidth: 5, minHeight: 3 },
  ],
  [
    "stream-health",
    { title: "Stream Health", icon: Activity, minWidth: 4, minHeight: 4 },
  ],
  ["chat", { title: "Chat", icon: MessageSquare, minWidth: 4, minHeight: 5 }],
  [
    "stream-info",
    { title: "Stream Info", icon: Radio, minWidth: 3.5, minHeight: 3.5 },
  ],
  [
    "multistream",
    { title: "Multistream", icon: Globe, minWidth: 4, minHeight: 2 },
  ],
  [
    "problems",
    { title: "Problems", icon: AlertTriangle, minWidth: 4, minHeight: 2 },
  ],
  [
    "moderators",
    { title: "Moderators", icon: Shield, minWidth: 3.5, minHeight: 2.5 },
  ],
]);
const widgetChoices = [...widgets].map(([key, { title }]) => ({ key, title }));
const attachedPanelStyle: ViewStyle = {
  borderTopLeftRadius: zero.atoms.borderRadius.none,
  borderTopRightRadius: zero.atoms.borderRadius.none,
};

type DropOperation = Pick<
  DragMoveEvent["operation"],
  "source" | "target" | "position"
>;

function dropSide(operation: DropOperation) {
  const { source, target, position } = operation;
  if (!source || !target || source.id === target.id || !target.element) return;
  const rect = target.element.getBoundingClientRect();
  if (!rect.width || !rect.height) return;
  return getDropSide(
    (position.current.x - rect.left) / rect.width,
    (position.current.y - rect.top) / rect.height,
  );
}

function describeDrop(operation: DragMoveEvent["operation"]) {
  const { source, target } = operation;
  const side = dropSide(operation);
  if (!source) return;
  if (!side || !target)
    return `Moving ${source.data?.label ?? "widget"}. No drop target.`;
  const direction =
    side.direction === "horizontal"
      ? side.first
        ? "left of"
        : "right of"
      : side.first
        ? "above"
        : "below";
  return `Moving ${source.data?.label ?? "widget"} ${direction} ${target.data?.label ?? "widget"}.`;
}

const dragAccessibility = Accessibility.configure({
  screenReaderInstructions: {
    draggable:
      "Press space to pick up a widget. Use arrow keys to move it, and hold shift to move faster. Press space to drop it, or escape to cancel.",
  },
  announcements: {
    dragstart: ({ operation }: DragStartEvent) =>
      `Picked up ${operation.source?.data?.label ?? "widget"}.`,
    dragmove: ({ operation }: DragMoveEvent) => describeDrop(operation),
    dragover: ({ operation }: DragMoveEvent) =>
      dropSide(operation) ? describeDrop(operation) : undefined,
    dragend: ({ operation, canceled }: DragEndEvent) =>
      canceled
        ? "Move canceled."
        : !dropSide(operation)
          ? "No drop target."
          : undefined,
  },
});
const dragFeedback = Feedback.configure({
  dropAnimation: { duration: 0 },
  keyboardTransition: null,
});

export default function CreatorDashboard({ videoRef }: { videoRef: any }) {
  const { theme } = useTheme();
  const { theatre, setTheatre, entryRef } = useCreatorTheatre();
  const { width, height } = useWindowDimensions();
  const stacked = width < breakpoints.xl;
  const { status, history } = useCreatorMetrics();
  const profile = useProfile();
  const problems = useLivestreamStore((state) => state.problems);
  const emojiData = useEmojiData();
  const problemsRef = useRef<ProblemsWrapperRef>(null);
  const addRef = useRef<HTMLButtonElement>(null);
  const layoutsRef = useRef<HTMLButtonElement>(null);
  const canvasRef = useRef<HTMLDivElement>(null);
  const resizing = useRef<{
    pointerId: number;
    start: number;
    size: number;
    boundary: SplitBoundary;
    initial: LayoutNode;
    latest: LayoutNode;
    minBefore: number;
    minAfter: number;
  } | null>(null);
  const widgetIds = useRef(new WeakMap<WidgetNode, number>());
  const nextId = useRef(0);
  const [layout, setLayout] = useState(loadLayout);
  const [presetStorage] = useState(loadPresets);
  const [presets, setPresets] = useState(presetStorage.presets);
  const [editing, setEditing] = useState(false);
  const [adding, setAdding] = useState(false);
  const [layoutsOpen, setLayoutsOpen] = useState(false);
  const [namingLayout, setNamingLayout] = useState(false);
  const [presetName, setPresetName] = useState("");
  const [presetSaveFailed, setPresetSaveFailed] = useState(false);
  const [drop, setDrop] = useState<{
    id: string;
    direction: "horizontal" | "vertical";
    first: boolean;
    allowed: boolean;
  } | null>(null);
  const [placementError, setPlacementError] = useState("");
  const [saveFailed, setSaveFailed] = useState(false);
  const [announcement, setAnnouncement] = useState("");
  const messagesPerMinute = history[history.length - 1]?.chatRate ?? null;
  const spacing = theme.spacing[4];
  const gutter = theme.spacing[2];
  const { widgets: placements, boundaries } = getLayoutGeometry(layout);
  const available = [...widgets].filter(
    ([key]) => !placements.some(({ widget }) => widget.widget === key),
  );
  const widgetId = (widget: WidgetNode) => {
    if (!widgetIds.current.has(widget))
      widgetIds.current.set(widget, nextId.current++);
    return `widget-${widgetIds.current.get(widget)}`;
  };
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

  function applySavedLayout(saved: LayoutNode, message: string) {
    updateLayout(reuseWidgets(layout, saved), message);
    setAdding(false);
    setPlacementError("");
  }

  function savePreset() {
    const name = presetName.trim();
    if (!name || presetStorage.unreadable) return;
    const next = [
      ...presets.filter((preset) => preset.name !== name),
      { name, layout },
    ];
    setPresets(next);
    const saved = savePresets(next);
    setPresetSaveFailed(!saved);
    setPresetName("");
    setNamingLayout(false);
    setAnnouncement(saved ? `${name} saved` : `${name} kept for this session`);
    requestAnimationFrame(() => layoutsRef.current?.focus());
  }

  function deletePreset(name: string) {
    if (presetStorage.unreadable) return;
    const next = presets.filter((preset) => preset.name !== name);
    setPresets(next);
    const saved = savePresets(next);
    setPresetSaveFailed(!saved);
    setAnnouncement(
      saved ? `${name} deleted` : `${name} removed for this session`,
    );
    requestAnimationFrame(() => layoutsRef.current?.focus());
  }

  function remove(path: number[], label: string) {
    updateLayout(removeWidget(layout, path), `${label} removed`);
    requestAnimationFrame(() => addRef.current?.focus());
  }

  function replace(path: number[], key: string) {
    if (!widgets.has(key)) return;
    const next = replaceWidget(layout, path, key);
    if (next === layout) return;
    const replacement = getLayoutGeometry(next).widgets.find(
      (item) =>
        item.path.length === path.length &&
        item.path.every((part, index) => part === path[index]),
    );
    updateLayout(next, `${widgets.get(key)?.title} selected`);
    if (replacement)
      requestAnimationFrame(() =>
        document
          .querySelector<HTMLSelectElement>(
            `[data-change-widget="${widgetId(replacement.widget)}"]`,
          )
          ?.focus(),
      );
  }

  function add(key: string) {
    const targets = placements
      .filter(({ width, height }) => width > 0 && height > 0)
      .reverse();
    const sourceId = `available-${key}`;
    const target =
      targets.find(
        ({ widget }) =>
          preparePlacement(sourceId, widgetId(widget), "vertical", false)?.fits,
      ) ?? targets[0];
    if (target && place(sourceId, widgetId(target.widget), "vertical", false)) {
      requestAnimationFrame(() => addRef.current?.focus());
    }
  }

  function preparePlacement(
    sourceId: string,
    targetId: string,
    direction: "horizontal" | "vertical",
    first: boolean,
  ) {
    const source = placements.find(
      ({ widget }) => widgetId(widget) === sourceId,
    );
    const target = placements.find(
      ({ widget }) => widgetId(widget) === targetId,
    );
    const key = sourceId.startsWith("available-") ? sourceId.slice(10) : "";
    if (
      !target ||
      (!source && !available.some(([id]) => id === key)) ||
      source?.widget === target.widget
    )
      return null;
    const widget: WidgetNode = source?.widget ?? { type: "leaf", widget: key };
    const next = source
      ? moveWidget(layout, source.path, target.path, direction, first)
      : insertWidget(layout, target.path, widget, direction, first);
    if (next === layout) return null;
    const bounds = canvasRef.current?.getBoundingClientRect();
    const fits = stacked
      ? direction === "vertical"
      : !!bounds &&
        getLayoutGeometry(next)
          .widgets.filter(
            ({ widget: placed }) =>
              placed === widget || placed === target.widget,
          )
          .every(({ widget, width, height }) => {
            const meta = widgets.get(widget.widget);
            if (!meta) return true;
            return (
              bounds.width * width - spacing >=
                meta.minWidth * theme.spacing[16] &&
              bounds.height * height - spacing >=
                meta.minHeight * theme.spacing[16]
            );
          });
    return { next, source: widget, fits, adding: !source };
  }

  function place(
    sourceId: string,
    targetId: string,
    direction: "horizontal" | "vertical",
    first: boolean,
  ) {
    const candidate = preparePlacement(sourceId, targetId, direction, first);
    if (!candidate) return false;
    if (!candidate.fits) {
      setPlacementError(
        "There is not enough room to place these widgets there.",
      );
      return false;
    }
    setPlacementError("");
    updateLayout(
      candidate.next,
      `${widgets.get(candidate.source.widget)?.title ?? candidate.source.widget} ${candidate.adding ? "added" : "moved"}`,
    );
    if (candidate.adding) setAdding(false);
    return true;
  }

  function dropAt(operation: DropOperation) {
    const { source, target } = operation;
    const side = dropSide(operation);
    if (!side || !source || !target) return null;
    const candidate = preparePlacement(
      String(source.id),
      String(target.id),
      side.direction,
      side.first,
    );
    return candidate
      ? { id: String(target.id), ...side, allowed: candidate.fits }
      : null;
  }

  function minimumExtent(
    node: LayoutNode,
    direction: SplitNode["direction"],
  ): number {
    if (node.type === "leaf") {
      const meta = widgets.get(node.widget);
      return meta
        ? (direction === "horizontal" ? meta.minWidth : meta.minHeight) *
            theme.spacing[16]
        : theme.touchTargets.minimum;
    }
    const sizes = node.children.map((child) => minimumExtent(child, direction));
    return node.direction === direction
      ? sizes.reduce((sum, size) => sum + size, 0) +
          spacing * (sizes.length - 1)
      : Math.max(...sizes);
  }

  function resizeLimits(boundary: SplitBoundary) {
    const bounds = canvasRef.current?.getBoundingClientRect();
    const size =
      boundary.direction === "horizontal"
        ? (bounds?.width ?? 0) * boundary.width
        : (bounds?.height ?? 0) * boundary.height;
    if (!size) return null;
    const minBefore =
      (minimumExtent(
        boundary.node.children[boundary.index],
        boundary.direction,
      ) +
        spacing) /
      size;
    const minAfter =
      (minimumExtent(
        boundary.node.children[boundary.index + 1],
        boundary.direction,
      ) +
        spacing) /
      size;
    const total =
      boundary.node.ratio[boundary.index] +
      boundary.node.ratio[boundary.index + 1];
    return minBefore + minAfter <= total
      ? { size, minBefore, minAfter, total }
      : null;
  }

  function startResize(
    event: PointerEvent<HTMLDivElement>,
    boundary: SplitBoundary,
  ) {
    const limits = resizeLimits(boundary);
    if (!limits || event.button !== 0) return;
    event.preventDefault();
    event.currentTarget.focus();
    event.currentTarget.setPointerCapture(event.pointerId);
    resizing.current = {
      ...limits,
      boundary,
      pointerId: event.pointerId,
      start:
        boundary.direction === "horizontal" ? event.clientX : event.clientY,
      initial: layout,
      latest: layout,
    };
  }

  function updateResize(event: PointerEvent<HTMLDivElement>) {
    const current = resizing.current;
    if (!current || current.pointerId !== event.pointerId) return;
    const position =
      current.boundary.direction === "horizontal"
        ? event.clientX
        : event.clientY;
    current.latest = resizeAdjacent(
      current.initial,
      current.boundary.path,
      current.boundary.index,
      (position - current.start) / current.size,
      current.minBefore,
      current.minAfter,
    );
    setLayout(current.latest);
  }

  function finishResize(
    event: PointerEvent<HTMLDivElement>,
    cancelled: boolean,
  ) {
    const current = resizing.current;
    if (!current || current.pointerId !== event.pointerId) return;
    if (cancelled) setLayout(current.initial);
    else {
      updateResize(event);
      if (current.latest !== current.initial) {
        setSaveFailed(!saveLayout(current.latest));
        setAnnouncement("Panel sizes updated");
      }
    }
    resizing.current = null;
  }

  function keyboardResize(
    event: KeyboardEvent<HTMLDivElement>,
    boundary: SplitBoundary,
  ) {
    const limits = resizeLimits(boundary);
    if (!limits) return;
    const negative =
      boundary.direction === "horizontal" ? "ArrowLeft" : "ArrowUp";
    const positive =
      boundary.direction === "horizontal" ? "ArrowRight" : "ArrowDown";
    const delta =
      event.key === negative
        ? -0.05
        : event.key === positive
          ? 0.05
          : event.key === "Home"
            ? -1
            : event.key === "End"
              ? 1
              : 0;
    if (!delta) return;
    event.preventDefault();
    const next = resizeAdjacent(
      layout,
      boundary.path,
      boundary.index,
      delta,
      limits.minBefore,
      limits.minAfter,
    );
    if (next !== layout) updateLayout(next, "Panel sizes updated");
  }

  function renderWidget(key: string) {
    switch (key) {
      case "stream-monitor":
        return (
          <StreamMonitor
            status={status}
            userProfile={profile}
            videoRef={videoRef}
            style={attachedPanelStyle}
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
            style={attachedPanelStyle}
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
            showTitle={false}
            style={attachedPanelStyle}
          />
        );
      case "multistream":
        return <MultistreamStatus style={attachedPanelStyle} />;
      case "moderators":
        return (
          <View testID="creator-moderators" style={{ flex: 1, minHeight: 0 }}>
            <Dashboard.ModeratorPanel
              isLive={status.incoming === true}
              showTitle={false}
              style={attachedPanelStyle}
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

  const dashboard = (
    <ProblemsWrapper ref={problemsRef}>
      <div
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
        <style>{`
          .creator-widget-grip {
            min-width: ${theme.spacing[8]}px;
            min-height: ${theme.spacing[8]}px;
          }
          .creator-widget-grip[data-editing="true"] {
            min-width: ${theme.touchTargets.minimum}px;
            min-height: ${theme.touchTargets.minimum}px;
          }
          @media (pointer: coarse) {
            .creator-widget-grip {
              min-width: ${theme.touchTargets.minimum}px;
              min-height: ${theme.touchTargets.minimum}px;
            }
          }
        `}</style>
        <div
          role="toolbar"
          aria-label="Creator dashboard controls"
          style={{
            display: theatre ? "none" : "flex",
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
              setLayoutsOpen(false);
              setNamingLayout(false);
              setPlacementError("");
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
          <button
            ref={entryRef}
            type="button"
            aria-label="Theatre mode"
            title="Theatre mode"
            style={buttonStyle}
            onClick={() => {
              setEditing(false);
              setAdding(false);
              setLayoutsOpen(false);
              setNamingLayout(false);
              setPlacementError("");
              setTheatre(true);
            }}
          >
            <span aria-hidden="true">
              <RectangleHorizontal size={theme.spacing[4]} />
            </span>
            Theatre
          </button>
          {editing && (
            <button
              ref={addRef}
              type="button"
              style={buttonStyle}
              aria-expanded={adding}
              onClick={() => {
                setAdding(!adding);
                setLayoutsOpen(false);
                setNamingLayout(false);
                setPlacementError("");
              }}
            >
              <span aria-hidden="true">
                <Plus size={theme.spacing[4]} />
              </span>
              Add widget
            </button>
          )}
          {editing && (
            <button
              ref={layoutsRef}
              type="button"
              style={buttonStyle}
              aria-expanded={layoutsOpen}
              onClick={() => {
                setLayoutsOpen(!layoutsOpen);
                setNamingLayout(false);
                setAdding(false);
                setPlacementError("");
              }}
            >
              Layouts
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
        {editing && layoutsOpen && (
          <div
            role="group"
            aria-label="Saved layouts"
            onKeyDown={(event) => {
              if (event.key === "Escape") {
                event.preventDefault();
                if (namingLayout) setNamingLayout(false);
                else setLayoutsOpen(false);
                layoutsRef.current?.focus();
              }
            }}
            style={{
              display: "flex",
              flexWrap: "wrap",
              alignItems: "center",
              gap: theme.spacing[2],
              padding: `${theme.spacing[2]}px ${spacing}px`,
            }}
          >
            {presets.length > 0 && (
              <select
                aria-label="Load layout"
                value=""
                style={buttonStyle}
                onChange={(event) => {
                  const preset = presets.find(
                    ({ name }) => name === event.target.value,
                  );
                  if (preset)
                    applySavedLayout(preset.layout, `${preset.name} loaded`);
                }}
              >
                <option value="">Load layout…</option>
                {presets.map((preset, index) => (
                  <option key={`${preset.name}-${index}`} value={preset.name}>
                    {preset.name}
                  </option>
                ))}
              </select>
            )}
            {namingLayout ? (
              <form
                style={{ display: "flex", gap: theme.spacing[2] }}
                onSubmit={(event) => {
                  event.preventDefault();
                  savePreset();
                }}
              >
                <input
                  autoFocus
                  aria-label="Layout name"
                  value={presetName}
                  onChange={(event) => setPresetName(event.target.value)}
                  placeholder="Layout name"
                  style={{ ...buttonStyle, cursor: "text" }}
                />
                <button
                  type="submit"
                  style={buttonStyle}
                  disabled={!presetName.trim()}
                >
                  Save
                </button>
              </form>
            ) : (
              <button
                type="button"
                style={buttonStyle}
                disabled={presetStorage.unreadable}
                onClick={() => setNamingLayout(true)}
              >
                Save layout
              </button>
            )}
            {namingLayout &&
              presets.some(({ name }) => name === presetName.trim()) && (
                <span style={{ color: theme.colors.text2 }}>
                  Saving will replace the existing layout.
                </span>
              )}
            {presets.map((preset, index) => (
              <button
                key={`${preset.name}-${index}`}
                type="button"
                style={buttonStyle}
                disabled={presetStorage.unreadable}
                aria-label={`Delete ${preset.name}`}
                onClick={() => deletePreset(preset.name)}
              >
                Delete {preset.name}
              </button>
            ))}
            <button
              type="button"
              style={buttonStyle}
              onClick={() =>
                applySavedLayout(DEFAULT_LAYOUT, "Default layout restored")
              }
            >
              Reset layout
            </button>
            <span style={{ color: theme.colors.text2 }}>
              Layouts are saved in this browser.
            </span>
          </div>
        )}
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
              available.map(([key, { title, icon }]) => (
                <AvailableWidget
                  key={key}
                  widgetKey={key}
                  title={title}
                  icon={icon}
                  style={buttonStyle}
                  onClick={() => add(key)}
                />
              ))
            ) : (
              <span>All widgets added</span>
            )}
            {available.length > 0 && (
              <span style={{ alignSelf: "center", color: theme.colors.text2 }}>
                Drag to place a widget, or select it to add where there is room.
              </span>
            )}
          </div>
        )}
        {presetStorage.unreadable && (
          <div
            role="alert"
            style={{
              padding: `${theme.spacing[2]}px ${spacing}px`,
              color: theme.colors.warning,
            }}
          >
            Could not read all saved layouts. Your stored layouts were left
            unchanged.
          </div>
        )}
        {presetSaveFailed && (
          <div
            role="alert"
            style={{
              padding: `${theme.spacing[2]}px ${spacing}px`,
              color: theme.colors.warning,
            }}
          >
            Could not save your layouts. Changes will last until you leave this
            page.
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
        {placementError && (
          <div
            role="alert"
            style={{
              padding: `${theme.spacing[2]}px ${spacing}px`,
              color: theme.colors.warning,
            }}
          >
            {placementError}
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
          style={{
            flex: 1,
            minHeight: 0,
            overflow: "auto",
            padding: gutter,
          }}
        >
          <div
            ref={canvasRef}
            style={{
              position: "relative",
              display: stacked ? "flex" : "block",
              flexDirection: "column",
              gap: gutter,
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
                const id = widgetId(widget);
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
                  <CreatorWidgetFrame
                    key={id}
                    id={id}
                    label={label}
                    widgetKey={widget.widget}
                    choices={widgetChoices}
                    icon={Icon}
                    editing={editing}
                    blank={widget.widget === "blank"}
                    dropSide={drop?.id === id ? drop : null}
                    onRemove={() => remove(path, label)}
                    onReplace={(key) => replace(path, key)}
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
                            left: `calc(${x * 100}% + ${gutter / 2}px)`,
                            top: `calc(${y * 100}% + ${gutter / 2}px)`,
                            width: `calc(${widgetWidth * 100}% - ${gutter}px)`,
                            height: `calc(${widgetHeight * 100}% - ${gutter}px)`,
                          }),
                    }}
                  >
                    {renderWidget(widget.widget)}
                  </CreatorWidgetFrame>
                );
              },
            )}
            {editing &&
              !stacked &&
              boundaries.map((boundary) => {
                const horizontal = boundary.direction === "horizontal";
                const total =
                  boundary.node.ratio[boundary.index] +
                  boundary.node.ratio[boundary.index + 1];
                const limits = resizeLimits(boundary);
                return (
                  <div
                    key={`${boundary.path.join(".")}:${boundary.index}`}
                    role="separator"
                    aria-label={horizontal ? "Resize columns" : "Resize rows"}
                    aria-orientation={horizontal ? "vertical" : "horizontal"}
                    aria-valuenow={
                      total
                        ? Math.round(
                            (100 * boundary.node.ratio[boundary.index]) / total,
                          )
                        : 0
                    }
                    aria-valuemin={
                      limits
                        ? Math.ceil((100 * limits.minBefore) / limits.total)
                        : 0
                    }
                    aria-valuemax={
                      limits
                        ? Math.floor(
                            (100 * (limits.total - limits.minAfter)) /
                              limits.total,
                          )
                        : 100
                    }
                    tabIndex={0}
                    onPointerDown={(event) => startResize(event, boundary)}
                    onPointerMove={updateResize}
                    onPointerUp={(event) => finishResize(event, false)}
                    onPointerCancel={(event) => finishResize(event, true)}
                    onKeyDown={(event) => keyboardResize(event, boundary)}
                    style={{
                      position: "absolute",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      zIndex: zero.atoms.zIndex[10].zIndex,
                      touchAction: "none",
                      cursor: horizontal ? "col-resize" : "row-resize",
                      ...(horizontal
                        ? {
                            left: `calc(${boundary.x * 100}% - ${spacing / 2}px)`,
                            top: `calc(${boundary.y * 100}% + ${gutter / 2}px)`,
                            width: spacing,
                            height: `calc(${boundary.height * 100}% - ${gutter}px)`,
                          }
                        : {
                            left: `calc(${boundary.x * 100}% + ${gutter / 2}px)`,
                            top: `calc(${boundary.y * 100}% - ${spacing / 2}px)`,
                            width: `calc(${boundary.width * 100}% - ${gutter}px)`,
                            height: spacing,
                          }),
                    }}
                  >
                    <div
                      aria-hidden="true"
                      style={{
                        background: theme.colors.borderStrong,
                        width: horizontal
                          ? zero.borders.width.medium.borderWidth
                          : "100%",
                        height: horizontal
                          ? "100%"
                          : zero.borders.width.medium.borderWidth,
                      }}
                    />
                  </div>
                );
              })}
          </div>
        </div>
      </div>
    </ProblemsWrapper>
  );

  return (
    <DragDropProvider
      plugins={(defaults) =>
        defaults.map((plugin) =>
          plugin === Accessibility
            ? dragAccessibility
            : plugin === Feedback
              ? dragFeedback
              : plugin,
        )
      }
      onCollision={({ collisions }, manager) => {
        const collision = collisions[0];
        const target = collision
          ? manager.registry.droppables.get(collision.id)
          : null;
        setDrop(
          dropAt({
            source: manager.dragOperation.source,
            target: target ?? null,
            position: manager.dragOperation.position,
          }),
        );
      }}
      onDragEnd={({ operation, canceled }) => {
        const side = canceled ? null : dropAt(operation);
        if (side && operation.source)
          place(
            String(operation.source.id),
            side.id,
            side.direction,
            side.first,
          );
        setDrop(null);
      }}
    >
      {dashboard}
    </DragDropProvider>
  );
}

function AvailableWidget({
  widgetKey,
  title,
  icon: Icon,
  style,
  onClick,
}: {
  widgetKey: string;
  title: string;
  icon: LucideIcon;
  style: CSSProperties;
  onClick: () => void;
}) {
  const { theme } = useTheme();
  const { ref, handleRef, isDragging } = useDraggable({
    id: `available-${widgetKey}`,
    data: { label: title },
  });
  const setHandle = useCallback(
    (element: HTMLButtonElement | null) => {
      ref(element);
      handleRef(element);
    },
    [ref, handleRef],
  );
  return (
    <div
      style={{
        display: "inline-flex",
        opacity: isDragging ? zero.atoms.opacity[50].opacity : undefined,
      }}
    >
      <button
        ref={setHandle}
        type="button"
        aria-label={`Drag ${title} to add`}
        style={{
          ...style,
          cursor: "grab",
          borderRight: 0,
          borderRadius: `${theme.borderRadius.md}px 0 0 ${theme.borderRadius.md}px`,
        }}
      >
        <GripVertical size={theme.spacing[4]} aria-hidden />
      </button>
      <button
        type="button"
        style={{
          ...style,
          borderRadius: `0 ${theme.borderRadius.md}px ${theme.borderRadius.md}px 0`,
        }}
        onClick={onClick}
      >
        <span aria-hidden="true">
          <Icon size={theme.spacing[4]} />
        </span>
        Add {title}
      </button>
    </div>
  );
}
