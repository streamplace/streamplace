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
import { useWindowDimensions, View } from "react-native";
import { useEmojiData } from "utils/emoji";
import CreatorHealth from "./creator-health.web";
import {
  getDropSide,
  getLayoutGeometry,
  insertWidget,
  moveWidget,
  removeWidget,
  replaceWidget,
  resizeAdjacent,
  type LayoutNode,
  type SplitBoundary,
  type SplitNode,
  type WidgetNode,
} from "./creator-layout";
import { loadLayout, saveLayout } from "./creator-layout-storage";
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
    { title: "Stream Info", icon: Radio, minWidth: 4, minHeight: 5 },
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
    { title: "Moderators", icon: Shield, minWidth: 5, minHeight: 3.5 },
  ],
]);
const widgetChoices = [...widgets].map(([key, { title }]) => ({ key, title }));

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
  const moveSelectRef = useRef<HTMLSelectElement>(null);
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
  const [editing, setEditing] = useState(false);
  const [adding, setAdding] = useState(false);
  const [moving, setMoving] = useState<WidgetNode | null>(null);
  const [moveTarget, setMoveTarget] = useState("");
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
  const { widgets: placements, boundaries } = getLayoutGeometry(layout);
  const available = [...widgets].filter(
    ([key]) => !placements.some(({ widget }) => widget.widget === key),
  );
  const widgetId = (widget: WidgetNode) => {
    if (!widgetIds.current.has(widget))
      widgetIds.current.set(widget, nextId.current++);
    return `widget-${widgetIds.current.get(widget)}`;
  };
  const targetOptions = moving
    ? placements.filter(({ widget }) => widget !== moving)
    : [];
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
    const widget: WidgetNode = { type: "leaf", widget: key };
    const target = placements
      .filter(({ width, height }) => width > 0 && height > 0)
      .at(-1)!;
    const next = insertWidget(layout, target.path, widget, "vertical", false);
    updateLayout(next, `${widgets.get(key)?.title} added`);
    setAdding(false);
    requestAnimationFrame(() => addRef.current?.focus());
  }

  function openMove(widget: WidgetNode) {
    const target = placements.find((item) => item.widget !== widget);
    if (!target) return;
    setMoving(widget);
    setMoveTarget(widgetId(target.widget));
    setAdding(false);
    setPlacementError("");
    requestAnimationFrame(() => moveSelectRef.current?.focus());
  }

  function closeMove(widget: WidgetNode) {
    setMoving(null);
    setPlacementError("");
    requestAnimationFrame(() =>
      document
        .querySelector<HTMLButtonElement>(
          `[data-move-widget="${widgetId(widget)}"]`,
        )
        ?.focus(),
    );
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
  ): boolean {
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

  function dropAt(operation: DragMoveEvent["operation"]) {
    const { source, target, position } = operation;
    if (!source || !target || source.id === target.id || !target.element)
      return null;
    const rect = target.element.getBoundingClientRect();
    if (!rect.width || !rect.height) return null;
    const side = getDropSide(
      (position.current.x - rect.left) / rect.width,
      (position.current.y - rect.top) / rect.height,
    );
    if (!side) return null;
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
              setMoving(null);
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
          {editing && (
            <button
              ref={addRef}
              type="button"
              style={buttonStyle}
              aria-expanded={adding}
              onClick={() => {
                setAdding(!adding);
                setMoving(null);
              }}
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
                Drag to place a widget, or select it to add after the last
                widget.
              </span>
            )}
          </div>
        )}
        {editing && moving && (
          <div
            role="group"
            aria-label={`Move ${widgets.get(moving.widget)?.title ?? moving.widget}`}
            onKeyDown={(event) => {
              if (event.key === "Escape") closeMove(moving);
            }}
            style={{
              display: "flex",
              flexWrap: "wrap",
              alignItems: "center",
              gap: theme.spacing[2],
              padding: `${theme.spacing[2]}px ${spacing}px`,
            }}
          >
            <strong>
              Moving {widgets.get(moving.widget)?.title ?? moving.widget}
            </strong>
            <label>
              Move next to{" "}
              <select
                ref={moveSelectRef}
                aria-label="Move next to"
                value={moveTarget}
                onChange={(event) => setMoveTarget(event.target.value)}
                style={buttonStyle}
              >
                {targetOptions.map(({ widget }) => (
                  <option key={widgetId(widget)} value={widgetId(widget)}>
                    {widgets.get(widget.widget)?.title ??
                      (widget.widget === "blank" ? "Empty" : widget.widget)}
                  </option>
                ))}
              </select>
            </label>
            {(
              [
                ["left", "horizontal", true],
                ["right", "horizontal", false],
                ["above", "vertical", true],
                ["below", "vertical", false],
              ] as const
            ).map(([label, direction, first]) => (
              <button
                key={label}
                type="button"
                style={buttonStyle}
                onClick={() => {
                  if (place(widgetId(moving), moveTarget, direction, first))
                    closeMove(moving);
                }}
              >
                Move {label}
              </button>
            ))}
            <button
              type="button"
              style={buttonStyle}
              onClick={() => closeMove(moving)}
            >
              Cancel
            </button>
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
            padding: spacing,
          }}
        >
          <div
            ref={canvasRef}
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
                    canMove={placements.length > 1}
                    dropSide={drop?.id === id ? drop : null}
                    onMove={() => openMove(widget)}
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
                            left: `calc(${x * 100}% + ${spacing / 2}px)`,
                            top: `calc(${y * 100}% + ${spacing / 2}px)`,
                            width: `calc(${widgetWidth * 100}% - ${spacing}px)`,
                            height: `calc(${widgetHeight * 100}% - ${spacing}px)`,
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
                            top: `calc(${boundary.y * 100}% + ${spacing / 2}px)`,
                            width: spacing,
                            height: `calc(${boundary.height * 100}% - ${spacing}px)`,
                          }
                        : {
                            left: `calc(${boundary.x * 100}% + ${spacing / 2}px)`,
                            top: `calc(${boundary.y * 100}% - ${spacing / 2}px)`,
                            width: `calc(${boundary.width * 100}% - ${spacing}px)`,
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
      onDragMove={({ operation }) => setDrop(dropAt(operation))}
      onDragOver={({ operation }) => setDrop(dropAt(operation))}
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
