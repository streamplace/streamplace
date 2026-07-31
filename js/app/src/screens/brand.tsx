import { Button, Text, useTheme } from "@streamplace/components";
import {
  colors,
  fontFamilies,
  spacing,
  statusColors,
  surfaces,
} from "@streamplace/components/src/lib/theme/tokens";
import {
  LogoLockup,
  LogoMark,
  LogoTile,
  MARK,
  MARK_WITH_HOLE,
  Wordmark,
} from "components/brand/logo";
import Container from "components/container";
import { Download } from "lucide-react-native";
import { type ReactNode, useState } from "react";
import {
  Linking,
  Pressable,
  ScrollView,
  View,
  useWindowDimensions,
} from "react-native";
import Svg, { Circle, G, Line, Path, Polygon, Rect } from "react-native-svg";

const BRAND_INK = surfaces.dark[0];
const BRAND_PAPER = colors.white;

// The readings the mark carries, ordered most obvious to most earned.
const MARK_READINGS: { title: string; body: string }[] = [
  {
    title: "It's an S.",
    body: "The solid ink is the letter — Streamplace, held in a single figure.",
  },
  {
    title: "It's two plays.",
    body: "There are two because a stream has two ends — one points out to broadcast, its mirror points back to watch. Play and rewind, live and replay: both sides of the same stream.",
  },
  {
    title: "It's a place.",
    body: "On a nautical chart, a triangle is a beacon — a fixed point that stays put while the water moves, something you can steer by. One mark is only a point; two, lined up, give you a bearing — the way out and the way home. Water is what moves; a place is what holds still within it. That's the place in Streamplace — your own fixed point, yours to keep and not ours to move, where people find you even as the stream keeps going.",
  },
];

const polyStr = (pts: [number, number][]) =>
  pts.map((p) => p.join(",")).join(" ");

function Construction({ size = 320 }: { size?: number }) {
  const { theme } = useTheme();
  const guide = theme.colors.text3;
  const faint = theme.colors.borderSubtle;
  const grid = Array.from({ length: MARK.grid + 1 }, (_, i) => i);

  return (
    <Svg width={size} height={size} viewBox="-2 -2 28 28">
      <Rect x={0} y={0} width={24} height={24} fill="transparent" />
      {grid.map((i) => (
        <G key={i}>
          <Line
            x1={i}
            y1={0}
            x2={i}
            y2={24}
            stroke={faint}
            strokeWidth={i % 6 === 0 ? 0.06 : 0.035}
          />
          <Line
            x1={0}
            y1={i}
            x2={24}
            y2={i}
            stroke={faint}
            strokeWidth={i % 6 === 0 ? 0.06 : 0.035}
          />
        </G>
      ))}
      {/* the rounded-square field */}
      <Path
        d={MARK.tilePath}
        fill="none"
        stroke={guide}
        strokeDasharray="0.5 0.5"
        strokeWidth={0.12}
      />
      {/* the two play triangles, each the other's 180-degree rotation */}
      <Polygon
        points={polyStr(MARK.upperPlay)}
        fill="none"
        stroke={guide}
        strokeDasharray="0.5 0.5"
        strokeWidth={0.12}
      />
      <Polygon
        points={polyStr(MARK.lowerPlay)}
        fill="none"
        stroke={guide}
        strokeDasharray="0.5 0.5"
        strokeWidth={0.12}
      />
      {/* the spine: the diagonal the two voids leave behind */}
      <Line
        x1={4}
        y1={20}
        x2={20}
        y2={4}
        stroke={guide}
        strokeDasharray="0.5 0.5"
        strokeWidth={0.12}
      />
      {/* the mark */}
      <Path d={MARK_WITH_HOLE} fill={theme.colors.text1} fillRule="evenodd" />
      {/* the center of rotation */}
      <Circle cx={MARK.center[0]} cy={MARK.center[1]} r={0.3} fill={guide} />
    </Svg>
  );
}

function Section({
  title,
  kicker,
  children,
}: {
  title: string;
  kicker?: string;
  children: ReactNode;
}) {
  const { theme } = useTheme();
  return (
    <View style={{ gap: spacing[4], marginBottom: spacing[12] }}>
      <View style={{ gap: spacing[2], maxWidth: 720 }}>
        {kicker ? (
          <Text
            size="xs"
            uppercase
            weight="semibold"
            style={{
              color: theme.colors.primary,
              fontFamily: fontFamilies.monoMedium,
            }}
          >
            {kicker}
          </Text>
        ) : null}
        <Text size="2xl" weight="semibold">
          {title}
        </Text>
      </View>
      {children}
    </View>
  );
}

function Panel({
  children,
  background,
  bordered = true,
  height = 176,
  grow = true,
}: {
  children: ReactNode;
  background?: string;
  bordered?: boolean;
  height?: number;
  grow?: boolean;
}) {
  const { theme } = useTheme();
  return (
    <View
      style={{
        flexGrow: grow ? 1 : 0,
        flexBasis: 220,
        minWidth: 180,
        height,
        borderRadius: theme.borderRadius.md,
        backgroundColor: background ?? theme.colors.surface1,
        borderColor: theme.colors.borderSubtle,
        borderWidth: bordered ? 1 : 0,
        alignItems: "center",
        justifyContent: "center",
        overflow: "hidden",
      }}
    >
      {children}
    </View>
  );
}

function Spec({ label, value }: { label: string; value: string }) {
  const { theme } = useTheme();
  return (
    <View
      style={{
        borderTopWidth: 1,
        borderTopColor: theme.colors.borderSubtle,
        paddingTop: spacing[3],
        gap: spacing[1],
      }}
    >
      <Text size="xs" color="muted">
        {label}
      </Text>
      <Text size="sm" weight="semibold">
        {value}
      </Text>
    </View>
  );
}

// A single button variant with its name + when-to-use note.
function ButtonSpec({
  label,
  note,
  children,
}: {
  label: string;
  note: string;
  children: ReactNode;
}) {
  const { theme } = useTheme();
  return (
    <View
      style={{
        gap: spacing[3],
        padding: spacing[4],
        borderRadius: 12,
        borderWidth: 1,
        borderColor: theme.colors.borderSubtle,
        backgroundColor: theme.colors.surface1,
        minWidth: 190,
      }}
    >
      <View style={{ alignItems: "flex-start" }}>{children}</View>
      <View style={{ gap: spacing[1] }}>
        <Text size="sm" weight="semibold">
          {label}
        </Text>
        <Text size="xs" color="muted">
          {note}
        </Text>
      </View>
    </View>
  );
}

function Swatch({
  name,
  value,
  note,
}: {
  name: string;
  value: string;
  note: string;
}) {
  const { theme } = useTheme();
  return (
    <View
      style={{ flexDirection: "row", alignItems: "center", gap: spacing[3] }}
    >
      <View
        style={{
          width: 56,
          height: 40,
          borderRadius: theme.borderRadius.sm,
          backgroundColor: value,
          borderColor: theme.colors.borderSubtle,
          borderWidth: 1,
        }}
      />
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text size="sm" weight="semibold">
          {name}
        </Text>
        <Text size="xs" color="muted">
          {note}
        </Text>
      </View>
      <Text
        size="xs"
        style={{
          color: theme.colors.text3,
          fontFamily: fontFamilies.monoRegular,
        }}
      >
        {value.toUpperCase()}
      </Text>
    </View>
  );
}

function Rule({ ok, children }: { ok: boolean; children: ReactNode }) {
  const { theme } = useTheme();
  return (
    <View style={{ flexDirection: "row", gap: spacing[3], maxWidth: 720 }}>
      <Text
        size="sm"
        weight="semibold"
        style={{
          width: 52,
          color: ok ? theme.colors.success : theme.colors.danger,
        }}
      >
        {ok ? "Do" : "Don't"}
      </Text>
      <Text size="sm" color="muted" style={{ flex: 1 }}>
        {children}
      </Text>
    </View>
  );
}

const BRAND_ASSETS: { file: string; label: string }[] = [
  { file: "streamplace-mark.svg", label: "Mark" },
  { file: "streamplace-tile.svg", label: "App tile" },
  { file: "streamplace-wordmark.svg", label: "Wordmark" },
  { file: "streamplace-lockup.svg", label: "Lockup" },
];

// A downloadable brand asset: a hover-lit row that saves the SVG. On web it
// renders as an <a href download>, so left-click downloads and right-click
// offers "Save link as". A no-op affordance on native.
function AssetDownload({ file, label }: { file: string; label: string }) {
  const { theme } = useTheme();
  const [hovered, setHovered] = useState(false);
  const path = `/brand/${file}`;
  return (
    <Pressable
      {...({ href: path, hrefAttrs: { download: file } } as any)}
      onHoverIn={() => setHovered(true)}
      onHoverOut={() => setHovered(false)}
      style={{
        flexDirection: "row",
        alignItems: "center",
        gap: spacing[3],
        borderWidth: 1,
        borderColor: hovered
          ? theme.colors.borderStrong
          : theme.colors.borderSubtle,
        borderRadius: theme.borderRadius.md,
        paddingHorizontal: spacing[4],
        paddingVertical: spacing[3],
        backgroundColor: hovered
          ? theme.colors.surface2
          : theme.colors.surface1,
      }}
    >
      <View style={{ flex: 1, gap: 2 }}>
        <Text size="sm" weight="medium">
          {label}
        </Text>
        <Text
          size="xs"
          style={{
            color: theme.colors.text3,
            fontFamily: fontFamilies.monoRegular,
          }}
        >
          {path}
        </Text>
      </View>
      <Download size={18} color={theme.colors.text2} />
    </Pressable>
  );
}

export default function BrandScreen() {
  const { theme } = useTheme();
  const { width } = useWindowDimensions();
  const compact = width < 760;

  return (
    <ScrollView style={{ backgroundColor: theme.colors.surface0 }}>
      <Container style={{ maxWidth: 1120, paddingVertical: spacing[10] }}>
        <View
          style={{
            minHeight: compact ? 420 : 520,
            justifyContent: "center",
            gap: spacing[8],
          }}
        >
          <View style={{ gap: spacing[5], maxWidth: 760 }}>
            <LogoLockup size={compact ? 34 : 44} />
            <Text
              size={compact ? "lg" : "xl"}
              weight="semibold"
              style={{ lineHeight: compact ? 24 : 28, maxWidth: 620 }}
            >
              Brand guidelines for Streamplace, the video layer for everything.
            </Text>
          </View>
          <View
            style={{ flexDirection: "row", flexWrap: "wrap", gap: spacing[4] }}
          >
            <Panel
              height={compact ? 220 : 280}
              background={theme.colors.surface1}
            >
              <LogoMark size={compact ? 128 : 176} />
            </Panel>
            <Panel
              height={compact ? 220 : 280}
              background={BRAND_INK}
              bordered={false}
            >
              <LogoLockup
                size={compact ? 26 : 34}
                color={BRAND_PAPER}
                markColor={BRAND_PAPER}
              />
            </Panel>
          </View>
        </View>

        <Section title="Reading the Mark" kicker="Letter, play, place">
          <Text size="base" color="muted" style={{ maxWidth: 620 }}>
            The Streamplace mark is one solid figure with two triangular play
            voids, cut in rotational symmetry — one turned out, its mirror
            turned back. It reads three ways, most obvious to most earned:
          </Text>
          <View style={{ gap: spacing[5], maxWidth: 680 }}>
            {MARK_READINGS.map((reading, i) => (
              <View
                key={reading.title}
                style={{ flexDirection: "row", gap: spacing[3] }}
              >
                <Text
                  size="sm"
                  style={{
                    width: 22,
                    lineHeight: 20,
                    color: theme.colors.text3,
                    fontFamily: fontFamilies.monoMedium,
                  }}
                >
                  {String(i + 1).padStart(2, "0")}
                </Text>
                <View style={{ flex: 1, gap: spacing[1] }}>
                  <Text weight="semibold">{reading.title}</Text>
                  <Text color="muted">{reading.body}</Text>
                </View>
              </View>
            ))}
          </View>
        </Section>

        <Section title="Core System" kicker="Icon, wordmark, lockup">
          <View
            style={{ flexDirection: "row", flexWrap: "wrap", gap: spacing[4] }}
          >
            <Panel>
              <LogoMark size={92} />
            </Panel>
            <Panel>
              <Wordmark size={32} />
            </Panel>
            <Panel>
              <LogoLockup size={32} />
            </Panel>
            <Panel>
              <LogoTile size={96} />
            </Panel>
          </View>
          <Text size="sm" color="muted" style={{ maxWidth: 760 }}>
            The icon carries the idea; the wordmark carries the address. The
            lockup is the default public signature, while the tile is reserved
            for app icons, avatars, and surfaces that require a contained
            square.
          </Text>
        </Section>

        <Section title="Construction Grid" kicker="24-unit geometry">
          <View
            style={{ flexDirection: "row", flexWrap: "wrap", gap: spacing[5] }}
          >
            <Panel height={360}>
              <Construction size={compact ? 260 : 320} />
            </Panel>
            <View style={{ flex: 1, minWidth: 260, gap: spacing[4] }}>
              <Text size="sm" color="muted">
                Start with an 18-unit rounded square on a 24-unit field. Cut a
                play triangle that breaches the right edge, then cut its exact
                180-degree rotation breaching the left. The ink left between
                them is the S; the diagonal spine falls through the center of
                rotation.
              </Text>
              <View style={{ gap: spacing[4] }}>
                <Spec label="Outer field" value="24 x 24 units" />
                <Spec label="Body" value="18u square, 3.5u corners" />
                <Spec label="Voids" value="Two plays, 180 deg symmetry" />
                <Spec label="Breach" value="Opposite edges, ~1u opening" />
                <Spec
                  label="Clear space"
                  value="Half the mark height on every side"
                />
              </View>
            </View>
          </View>
        </Section>

        <Section title="Lockup Spacing" kicker="Signature rules">
          <Panel height={260} grow={false}>
            <View
              style={{
                borderWidth: 1,
                borderStyle: "dashed",
                borderColor: theme.colors.borderStrong,
                padding: compact ? spacing[5] : spacing[8],
                borderRadius: theme.borderRadius.sm,
              }}
            >
              <LogoLockup size={compact ? 28 : 38} />
            </View>
          </Panel>
          <Text size="sm" color="muted" style={{ maxWidth: 760 }}>
            Set the mark at 1.3x the wordmark size. The gap is one quarter of
            the mark height. Keep at least half a mark of clear space around the
            whole lockup. Minimum sizes: 16px for the mark, 20px wordmark size
            for the full lockup.
          </Text>
        </Section>

        <Section title="Typography" kicker="Geist, one family">
          <View
            style={{ flexDirection: "row", flexWrap: "wrap", gap: spacing[4] }}
          >
            <Panel height={compact ? 200 : 240} grow={false}>
              <View style={{ alignItems: "center", gap: spacing[2] }}>
                <Text
                  style={{
                    fontFamily: fontFamilies.semiBold,
                    fontSize: compact ? 92 : 124,
                    lineHeight: compact ? 100 : 132,
                    color: theme.colors.text1,
                  }}
                >
                  Aa
                </Text>
                <Text
                  size="xs"
                  uppercase
                  style={{
                    color: theme.colors.text3,
                    fontFamily: fontFamilies.monoMedium,
                  }}
                >
                  Geist
                </Text>
              </View>
            </Panel>
            <View
              style={{
                flex: 1,
                minWidth: 280,
                gap: spacing[4],
                justifyContent: "center",
              }}
            >
              {[
                {
                  fam: fontFamilies.regular,
                  label: "Regular · 400 — body copy",
                },
                {
                  fam: fontFamilies.medium,
                  label: "Medium · 500 — UI labels, buttons, nav",
                },
                {
                  fam: fontFamilies.semiBold,
                  label: "SemiBold · 600 — headings, the wordmark",
                },
              ].map((w) => (
                <View key={w.label} style={{ gap: spacing[1] }}>
                  <Text
                    size="xs"
                    style={{
                      color: theme.colors.text3,
                      fontFamily: fontFamilies.monoRegular,
                    }}
                  >
                    {w.label}
                  </Text>
                  <Text
                    style={{
                      fontFamily: w.fam,
                      fontSize: compact ? 22 : 26,
                      color: theme.colors.text1,
                    }}
                  >
                    The video layer for everything
                  </Text>
                </View>
              ))}
            </View>
          </View>

          <Panel height={compact ? 116 : 104} grow>
            <View
              style={{
                alignItems: "center",
                gap: spacing[2],
                paddingHorizontal: spacing[4],
              }}
            >
              <Text
                style={{
                  fontFamily: fontFamilies.monoMedium,
                  fontSize: compact ? 14 : 17,
                  color: theme.colors.text1,
                  textAlign: "center",
                }}
              >
                rtmp://stream.place/live · sk_live_9f2a…c7
              </Text>
              <Text
                size="xs"
                uppercase
                style={{
                  color: theme.colors.text3,
                  fontFamily: fontFamilies.monoMedium,
                }}
              >
                Geist Mono
              </Text>
            </View>
          </Panel>

          <Text size="sm" color="muted" style={{ maxWidth: 760 }}>
            One typeface carries the whole system. Geist — a clean, contemporary
            grotesque — in just three weights: Regular for text, Medium for
            interface, SemiBold for headings and the wordmark. Its monospace
            companion, Geist Mono, is reserved for the technical register:
            stream keys, code, and the small labels above each section here.
          </Text>
          <Text size="sm" color="muted" style={{ maxWidth: 760 }}>
            Geist is free and open source under the SIL Open Font License,
            designed by{" "}
            <Text
              size="sm"
              onPress={() => Linking.openURL("https://basement.studio")}
              style={{ color: theme.colors.primary }}
            >
              basement.studio
            </Text>
            .
          </Text>
        </Section>

        <Section title="Color" kicker="Mono brand, one accent">
          <Text size="sm" color="muted" style={{ maxWidth: 760 }}>
            The brand is monochrome — ink and paper carry the identity. Indigo
            is a secondary accent reserved for interface state (links, focus,
            active nav, selected items) — not button fills, and never the mark.
            Red signals state, not decoration: a filled red dot is on-air, and
            that hero use stays reserved; destructive and error states borrow
            red as ink (outline, text, icon) so a glance still separates "live"
            from "careful." Red never becomes a brand or accent color.
          </Text>
          <View style={{ gap: spacing[4], maxWidth: 760 }}>
            <Swatch
              name="Ink"
              value={BRAND_INK}
              note="Brand on light — the mark, type, surfaces"
            />
            <Swatch
              name="Paper"
              value={BRAND_PAPER}
              note="Brand on dark — the reverse mark and type"
            />
            <Swatch
              name="Indigo"
              value={colors.primary[500]}
              note="Secondary accent — interface only, never the mark"
            />
            <Swatch
              name="Live red"
              value={statusColors.live}
              note="On-air as a filled bug; danger & error as red ink — state, never decoration"
            />
          </View>
        </Section>

        <Section title="Monochrome Variants" kicker="One solid color">
          <View
            style={{ flexDirection: "row", flexWrap: "wrap", gap: spacing[4] }}
          >
            <Panel>
              <LogoMark size={72} />
            </Panel>
            <Panel background={theme.colors.primary} bordered={false}>
              <LogoMark size={72} color={BRAND_PAPER} />
            </Panel>
            <Panel background={BRAND_PAPER} bordered={false}>
              <LogoMark size={72} color={BRAND_INK} />
            </Panel>
            <Panel background={surfaces.dark[3]}>
              <LogoLockup
                size={24}
                color={BRAND_PAPER}
                markColor={BRAND_PAPER}
              />
            </Panel>
          </View>
        </Section>

        <Section title="Buttons" kicker="Contrast, not color">
          <Text size="sm" color="muted" style={{ maxWidth: 760 }}>
            Emphasis comes from contrast, not hue — the way ink and paper carry
            the mark. The single most important action per view is Paper
            (near-white on dark, ink on light): the highest-contrast element on
            screen. Everything else steps down in weight. Indigo stays off
            button fills so it keeps its meaning as a state color; live red is
            never a button.
          </Text>
          <View
            style={{
              flexDirection: "row",
              flexWrap: "wrap",
              gap: spacing[4],
              alignItems: "flex-start",
            }}
          >
            <ButtonSpec
              label="Primary · Paper"
              note="The one hero action. One per view."
            >
              <Button width="min">Publish</Button>
            </ButtonSpec>
            <ButtonSpec label="Secondary · Tonal" note="Supporting actions.">
              <Button variant="secondary" width="min">
                Save draft
              </Button>
            </ButtonSpec>
            <ButtonSpec label="Ghost" note="Low-stakes or repeated.">
              <Button variant="ghost" width="min">
                Cancel
              </Button>
            </ButtonSpec>
            <ButtonSpec label="Danger · Red ink" note="Outline, never a fill.">
              <Button variant="danger" width="min">
                Delete
              </Button>
            </ButtonSpec>
            <ButtonSpec
              label="Accent · Indigo"
              note="Reserved. Rare, opt-in brand moment."
            >
              <Button variant="accent" width="min">
                Get started
              </Button>
            </ButtonSpec>
          </View>
          <View style={{ gap: spacing[3] }}>
            <Rule ok>
              Exactly one Paper (primary) button per view — it marks the single
              most important action.
            </Rule>
            <Rule ok>
              Step down to tonal, then ghost, for supporting actions. Hierarchy
              reads by contrast alone.
            </Rule>
            <Rule ok>
              Reserve the indigo accent variant for a deliberate brand moment,
              not routine primaries.
            </Rule>
            <Rule ok={false}>
              Don't fill a button with solid red — a red fill reads as on-air.
              Destructive actions use red ink (an outline), never a fill.
            </Rule>
            <Rule ok={false}>
              Don't place two Paper buttons side by side — nothing signals which
              one is primary.
            </Rule>
          </View>
        </Section>

        <Section title="Downloads" kicker="SVG brand assets">
          <View style={{ gap: spacing[3], maxWidth: 760 }}>
            {BRAND_ASSETS.map((a) => (
              <AssetDownload key={a.file} file={a.file} label={a.label} />
            ))}
          </View>
        </Section>

        <Section title="Usage" kicker="Keep it simple">
          <View style={{ gap: spacing[3] }}>
            <Rule ok>
              Use the lockup as the default brand signature on web pages and
              documents.
            </Rule>
            <Rule ok>
              Use the standalone mark for favicons, compact navigation, avatars,
              and badges.
            </Rule>
            <Rule ok>
              Keep the mark monochrome: ink or paper. Indigo is an interface
              accent, never a mark color.
            </Rule>
            <Rule ok={false}>
              Don't outline, shadow, gradient-fill, skew, or redraw the mark.
            </Rule>
            <Rule ok={false}>
              Don't fill the play voids or color them apart. The S is one solid
              color; the plays are negative space.
            </Rule>
            <Rule ok={false}>
              Don't rotate or flip the mark. It reads as an S upright — turned,
              it becomes a Z.
            </Rule>
          </View>
        </Section>
      </Container>
    </ScrollView>
  );
}
