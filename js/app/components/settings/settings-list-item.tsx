import { Switch, Text, zero } from "@streamplace/components";
import { fontFamilies } from "@streamplace/components/src/lib/theme/tokens";
import { Edit3, Trash2 } from "lucide-react-native";
import { ReactNode } from "react";
import { Pressable, View } from "react-native";

const { bg, text, p, mb, w, h, r, layout, borders, flex, gap, pt } = zero;

interface SettingsListItemProps {
  title: string;
  subtitle?: string;
  url?: string;
  active: boolean;
  isDeleting?: boolean;
  badges?: Array<{ text: string; color: string; bgColor: string }>;
  metadata?: Array<{ label: string; value: string | string[] }>;
  footer?: {
    left?: string;
    right?: ReactNode;
  };
  onEdit: () => void;
  onDelete: () => void;
  onToggle?: (active: boolean) => void;
  isToggling?: boolean;
}

export function SettingsListItem({
  title,
  subtitle,
  url,
  active,
  isDeleting = false,
  badges = [],
  metadata = [],
  footer,
  onEdit,
  onDelete,
  onToggle,
  isToggling = false,
}: SettingsListItemProps) {
  const { theme } = zero.useTheme();

  return (
    <View
      style={[
        flex.shrink[1],
        borders.width.thin,
        { borderColor: theme.colors.borderStrong },
        bg.neutral[800],
        r.xl,
        p[4],
        mb[3],
        layout.flex.column,
        gap.all[3],
        { opacity: isDeleting ? 0.5 : active ? 1 : 0.7 },
      ]}
    >
      {/* Header */}
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
        ]}
      >
        <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
          <View
            style={[
              w[3],
              h[3],
              r.full,
              {
                backgroundColor: active
                  ? theme.colors.success
                  : theme.colors.text4,
              },
            ]}
          />
          <Text style={[{ fontSize: 16, fontWeight: "600" }]}>{title}</Text>
          {badges.map((badge, index) => (
            <View
              key={index}
              style={[{ backgroundColor: badge.bgColor }, p[1], r.full]}
            >
              <Text style={[{ color: badge.color, fontSize: 12 }]}>
                {badge.text}
              </Text>
            </View>
          ))}
        </View>

        <View style={[layout.flex.row, gap.all[2]]}>
          {onToggle && (
            <Switch
              value={active}
              onValueChange={onToggle}
              disabled={isDeleting || isToggling}
            />
          )}
          <Pressable
            style={[
              p[2],
              r.md,
              layout.flex.center,
              {
                minWidth: 32,
                minHeight: 32,
                backgroundColor: theme.colors.surface2,
              },
            ]}
            onPress={onEdit}
            disabled={isDeleting}
          >
            <Edit3 size={16} color={theme.colors.text2} />
          </Pressable>

          <Pressable
            style={[
              bg.red[800],
              p[2],
              r.md,
              layout.flex.center,
              { minWidth: 32, minHeight: 32 },
            ]}
            onPress={onDelete}
            disabled={isDeleting}
          >
            <Trash2 size={16} />
          </Pressable>
        </View>
      </View>

      {/* Subtitle/Description */}
      {subtitle && (
        <Text style={[{ color: theme.colors.text2 }, { fontSize: 14 }]}>
          {subtitle}
        </Text>
      )}

      {/* URL */}
      {url && (
        <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
          <Text style={[{ color: theme.colors.text2 }, { fontSize: 12 }]}>
            URL:
          </Text>
          <Text
            style={[
              { color: theme.colors.text3 },
              { fontSize: 12, fontFamily: fontFamilies.monoRegular },
            ]}
            numberOfLines={1}
          >
            {url.length > 50
              ? url.slice(0, 45) + "..." + url.slice(url.length - 5)
              : url}
          </Text>
        </View>
      )}

      {/* Metadata */}
      {metadata.map((meta, index) => (
        <View
          key={index}
          style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
        >
          <Text style={[{ color: theme.colors.text2 }, { fontSize: 12 }]}>
            {meta.label}:
          </Text>
          <View style={[layout.flex.row, gap.all[1], { flexWrap: "wrap" }]}>
            {Array.isArray(meta.value) ? (
              meta.value.map((item: any, idx: number) => (
                <View
                  key={idx}
                  style={[
                    { backgroundColor: theme.colors.primary },
                    p[1],
                    r.full,
                  ]}
                >
                  <Text style={[text.blue[300], { fontSize: 11 }]}>{item}</Text>
                </View>
              ))
            ) : (
              <Text style={[{ color: theme.colors.text3 }, { fontSize: 12 }]}>
                {meta.value}
              </Text>
            )}
          </View>
        </View>
      ))}

      {/* Footer */}
      {footer && (
        <View
          style={[
            layout.flex.row,
            layout.flex.spaceBetween,
            pt[2],
            borders.top.width.thin,
            borders.top.color.gray[100],
          ]}
        >
          {footer.left && (
            <Text style={[{ color: theme.colors.text3 }, { fontSize: 11 }]}>
              {footer.left}
            </Text>
          )}
          {footer.right && (
            <View style={[layout.flex.row, gap.all[4]]}>{footer.right}</View>
          )}
        </View>
      )}
    </View>
  );
}
