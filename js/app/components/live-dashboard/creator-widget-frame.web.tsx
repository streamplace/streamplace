import { useDraggable, useDroppable } from "@dnd-kit/react";
import { useTheme, zero } from "@streamplace/components";
import {
  ChevronDown,
  GripVertical,
  Move,
  X,
  type LucideIcon,
} from "lucide-react-native";
import { useCallback, type CSSProperties, type ReactNode } from "react";
import type { SplitNode } from "./creator-layout";

export default function CreatorWidgetFrame({
  id,
  label,
  widgetKey,
  choices,
  icon: Icon,
  editing,
  blank,
  canMove,
  dropSide,
  style,
  onMove,
  onRemove,
  onReplace,
  children,
}: {
  id: string;
  label: string;
  widgetKey: string;
  choices: { key: string; title: string }[];
  icon: LucideIcon;
  editing: boolean;
  blank: boolean;
  canMove: boolean;
  dropSide: {
    direction: SplitNode["direction"];
    first: boolean;
    allowed: boolean;
  } | null;
  style: CSSProperties;
  onMove: () => void;
  onRemove: () => void;
  onReplace: (key: string) => void;
  children: ReactNode;
}) {
  const { theme } = useTheme();
  const {
    ref: dragRef,
    handleRef,
    isDragging,
  } = useDraggable({
    id,
    disabled: !editing || blank,
  });
  const { ref: dropRef } = useDroppable({ id, disabled: !editing });
  const setHandle = useCallback(
    (element: HTMLButtonElement | null) => {
      dragRef(element);
      handleRef(element);
    },
    [dragRef, handleRef],
  );
  const iconButton: CSSProperties = {
    display: "inline-flex",
    alignItems: "center",
    justifyContent: "center",
    minWidth: theme.touchTargets.minimum,
    minHeight: theme.touchTargets.minimum,
    border: 0,
    background: "transparent",
    color: theme.colors.text2,
    cursor: "pointer",
  };

  return (
    <section
      ref={dropRef}
      aria-label={`${label} widget`}
      style={{
        ...style,
        position: style.position ?? "relative",
        opacity: isDragging ? zero.atoms.opacity[50].opacity : undefined,
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: editing ? theme.spacing[1] : theme.spacing[2],
          minHeight: theme.spacing[8],
          background: theme.colors.surface1,
          border: `${zero.borders.width.thin.borderWidth}px solid ${theme.colors.borderSubtle}`,
          borderRadius: `${theme.borderRadius.md}px ${theme.borderRadius.md}px 0 0`,
          paddingLeft: editing ? theme.spacing[1] : theme.spacing[3],
          color: theme.colors.text2,
        }}
      >
        {editing && !blank && (
          <button
            ref={setHandle}
            type="button"
            aria-label={`Drag ${label}`}
            style={{ ...iconButton, cursor: "grab" }}
          >
            <span aria-hidden="true">
              <GripVertical size={theme.spacing[4]} />
            </span>
          </button>
        )}
        <span aria-hidden="true">
          <Icon size={theme.spacing[4]} />
        </span>
        {editing ? (
          <div style={{ position: "relative", flex: 1, minWidth: 0 }}>
            <select
              aria-label={`Change ${label} widget`}
              title={label}
              data-change-widget={id}
              value={widgetKey}
              onChange={(event) => onReplace(event.target.value)}
              style={{
                width: "100%",
                minHeight: theme.touchTargets.minimum,
                appearance: "none",
                overflow: "hidden",
                textOverflow: "ellipsis",
                paddingRight: theme.spacing[5],
                color: theme.colors.text1,
                background: theme.colors.surface1,
                border: 0,
                font: "inherit",
                cursor: "pointer",
              }}
            >
              {!choices.some(({ key }) => key === widgetKey) && (
                <option value={widgetKey}>{label}</option>
              )}
              {choices.map(({ key, title }) => (
                <option key={key} value={key}>
                  {title}
                </option>
              ))}
            </select>
            <span
              aria-hidden="true"
              style={{
                position: "absolute",
                top: 0,
                right: 0,
                bottom: 0,
                display: "flex",
                alignItems: "center",
                pointerEvents: "none",
              }}
            >
              <ChevronDown size={theme.spacing[4]} />
            </span>
          </div>
        ) : (
          <strong
            style={{
              flex: 1,
              minWidth: 0,
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
              color: theme.colors.text1,
            }}
          >
            {label}
          </strong>
        )}
        {editing && !blank && (
          <button
            type="button"
            aria-label={`Move ${label}`}
            data-move-widget={id}
            disabled={!canMove}
            style={iconButton}
            onClick={onMove}
          >
            <span aria-hidden="true">
              <Move size={theme.spacing[4]} />
            </span>
          </button>
        )}
        {editing && !blank && (
          <button
            type="button"
            aria-label={`Remove ${label}`}
            style={iconButton}
            onClick={onRemove}
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
          overflow: style.position === "absolute" ? "auto" : "visible",
        }}
      >
        {children}
      </div>
      {dropSide && (
        <div
          data-testid="creator-drop-preview"
          aria-hidden="true"
          style={{
            position: "absolute",
            pointerEvents: "none",
            zIndex: zero.atoms.zIndex[10].zIndex,
            background: dropSide.allowed
              ? theme.colors.primary
              : theme.colors.danger,
            opacity: zero.atoms.opacity[20].opacity,
            ...(dropSide.direction === "horizontal"
              ? {
                  top: 0,
                  bottom: 0,
                  width: "50%",
                  [dropSide.first ? "left" : "right"]: 0,
                }
              : {
                  left: 0,
                  right: 0,
                  height: "50%",
                  [dropSide.first ? "top" : "bottom"]: 0,
                }),
          }}
        />
      )}
    </section>
  );
}
