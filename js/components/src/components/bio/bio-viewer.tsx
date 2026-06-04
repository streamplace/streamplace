import { Image } from "expo-image";
import React from "react";
import {
  PlaceStreamBioBlocksBlockquote,
  PlaceStreamBioBlocksBskyPost,
  PlaceStreamBioBlocksEmbed,
  PlaceStreamBioBlocksHeader,
  PlaceStreamBioBlocksImage,
  PlaceStreamBioBlocksLink,
  PlaceStreamBioBlocksLivestream,
  PlaceStreamBioBlocksOrderedList,
  PlaceStreamBioBlocksSchedule,
  PlaceStreamBioBlocksSocialLinks,
  PlaceStreamBioBlocksText,
  PlaceStreamBioBlocksUnorderedList,
  PlaceStreamBioDefs,
  PlaceStreamBioLayoutsPanels,
  PlaceStreamBioPage,
  PlaceStreamBioRichtextFacet,
} from "streamplace";
import { zero } from "../..";
import { useTheme } from "../../lib/theme/theme";
import { Text } from "../ui/text";
import { View } from "../ui/view";
import { PanelsLayout } from "./panels-layout";

export interface BioViewerProps {
  bio: PlaceStreamBioPage.Record;
  did?: string;
}

export function BioViewer({ bio, did }: BioViewerProps) {
  const { zero: zt } = useTheme();

  return (
    <View
      style={{
        maxWidth: 1920,
        width: "100%",
        alignSelf: "center",
        padding: 16,
        justifyContent: "center",
        alignContent: "center",
      }}
    >
      <BioHeader bio={bio} />
      {bio.layout && <BioLayout layout={bio.layout} did={did} />}
    </View>
  );
}

function BioHeader({ bio }: { bio: PlaceStreamBioPage.Record }) {
  const { zero: zt } = useTheme();

  return (
    <View style={[zero.mx.auto]}>
      <View
        style={[
          zero.mb[8],
          zt.bg.muted,
          zero.r.md,
          zero.p[4],
          zero.layout.flex.row,
          { maxWidth: "1200px" },
        ]}
      >
        {bio.description && (
          <View style={[zero.flex.values[3]]}>
            <RichTextView
              plaintext={bio.description.plaintext}
              facets={bio.description.facets}
            />
          </View>
        )}
        {bio.socials && bio.socials.length > 0 && (
          <View style={[zero.flex.values[1]]}>
            <SocialRow socials={bio.socials} />
          </View>
        )}
      </View>
    </View>
  );
}

function SocialRow({ socials }: { socials: PlaceStreamBioDefs.Social[] }) {
  return (
    <View direction="row" style={{ flexWrap: "wrap", gap: 8, marginTop: 12 }}>
      {socials.map((s, i) => (
        <SocialLink key={i} social={s} />
      ))}
    </View>
  );
}

const PLATFORM_LABELS: Record<string, string> = {
  bluesky: "Bluesky",
  twitter: "Twitter",
  youtube: "YouTube",
  twitch: "Twitch",
  kick: "Kick",
  discord: "Discord",
  instagram: "Instagram",
  tiktok: "TikTok",
  github: "GitHub",
  cashapp: "Cash App",
  "ko-fi": "Ko-fi",
  patreon: "Patreon",
  website: "Web",
};

function SocialLink({ social }: { social: PlaceStreamBioDefs.Social }) {
  const { zero: zt } = useTheme();
  const label = PLATFORM_LABELS[social.platform] ?? social.platform;

  return (
    <View style={[zero.px[2], zero.py[1], zt.bg.card, zero.r.full]}>
      <Text size="sm">
        {label}
        {social.handle ? ` ${social.handle}` : ""}
      </Text>
    </View>
  );
}

function BioLayout({
  layout,
  did,
}: {
  layout: PlaceStreamBioPage.Record["layout"];
  did?: string;
}) {
  if (!layout) return null;

  const t = (layout as any)?.$type as string | undefined;

  if (t === "place.stream.bio.layouts.panels") {
    return (
      <PanelsLayout
        panels={(layout as PlaceStreamBioLayoutsPanels.Main).panels}
        renderPanel={(panel, panelIdx) =>
          panel.blocks.map((entry, blockIdx) => (
            <BlockEntryView key={blockIdx} entry={entry} did={did} />
          ))
        }
      />
    );
  }

  return <Text color="muted">Unsupported layout: {t ?? "(no type)"}</Text>;
}

function BlockEntryView({
  entry,
  did,
}: {
  entry: PlaceStreamBioLayoutsPanels.BlockEntry;
  did?: string;
}) {
  const alignment = mapAlignment(entry.alignment);

  return (
    <View style={{ marginBottom: 12, alignItems: alignment }}>
      <BlockRenderer block={entry.block} did={did} />
    </View>
  );
}

function mapAlignment(
  a?: string,
): "flex-start" | "center" | "flex-end" | undefined {
  switch (a) {
    case "left":
      return "flex-start";
    case "center":
      return "center";
    case "right":
      return "flex-end";
    default:
      return undefined;
  }
}

function BlockRenderer({
  block,
  did,
}: {
  block: PlaceStreamBioLayoutsPanels.BlockEntry["block"];
  did?: string;
}) {
  const t = (block as any)?.$type as string | undefined;

  switch (t) {
    case "place.stream.bio.blocks.text":
      return <TextBlock block={block as PlaceStreamBioBlocksText.Main} />;
    case "place.stream.bio.blocks.header":
      return <HeaderBlock block={block as PlaceStreamBioBlocksHeader.Main} />;
    case "place.stream.bio.blocks.image":
      return (
        <ImageBlock block={block as PlaceStreamBioBlocksImage.Main} did={did} />
      );
    case "place.stream.bio.blocks.orderedList":
      return (
        <OrderedListBlock
          block={block as PlaceStreamBioBlocksOrderedList.Main}
          did={did}
        />
      );
    case "place.stream.bio.blocks.unorderedList":
      return (
        <UnorderedListBlock
          block={block as PlaceStreamBioBlocksUnorderedList.Main}
          did={did}
        />
      );
    case "place.stream.bio.blocks.blockquote":
      return (
        <BlockquoteBlock block={block as PlaceStreamBioBlocksBlockquote.Main} />
      );
    case "place.stream.bio.blocks.divider":
      return <DividerBlock />;
    case "place.stream.bio.blocks.link":
      return (
        <LinkBlock block={block as PlaceStreamBioBlocksLink.Main} did={did} />
      );
    case "place.stream.bio.blocks.socialLinks":
      return (
        <SocialLinksBlock
          block={block as PlaceStreamBioBlocksSocialLinks.Main}
        />
      );
    case "place.stream.bio.blocks.livestream":
      return (
        <LivestreamBlock block={block as PlaceStreamBioBlocksLivestream.Main} />
      );
    case "place.stream.bio.blocks.schedule":
      return (
        <ScheduleBlock block={block as PlaceStreamBioBlocksSchedule.Main} />
      );
    case "place.stream.bio.blocks.bskyPost":
      return (
        <BskyPostBlock block={block as PlaceStreamBioBlocksBskyPost.Main} />
      );
    case "place.stream.bio.blocks.embed":
      return (
        <EmbedBlock block={block as PlaceStreamBioBlocksEmbed.Main} did={did} />
      );
    default:
      return (
        <Text color="muted" size="sm">
          Unknown block: {t ?? "(no type)"}
        </Text>
      );
  }
}

function TextBlock({ block }: { block: PlaceStreamBioBlocksText.Main }) {
  const { zero: zt } = useTheme();
  const size = textSizeToVariant(block.textSize);

  return (
    <View style={{ marginBottom: 8 }}>
      <RichTextView plaintext={block.plaintext} facets={block.facets} />
    </View>
  );
}

function textSizeToVariant(size?: string): "sm" | "base" | "lg" | undefined {
  switch (size) {
    case "small":
      return "sm";
    case "large":
      return "lg";
    default:
      return undefined;
  }
}

function HeaderBlock({ block }: { block: PlaceStreamBioBlocksHeader.Main }) {
  const level = block.level ?? 1;
  const headingLevel = Math.max(1, Math.min(3, level)) as 1 | 2 | 3;
  const variant = `h${headingLevel}` as "h1" | "h2" | "h3";

  return (
    <View style={{ marginBottom: 8 }}>
      <Text variant={variant} weight="bold">
        <RichTextInline plaintext={block.plaintext} facets={block.facets} />
      </Text>
    </View>
  );
}

function ImageBlock({
  block,
  did,
}: {
  block: PlaceStreamBioBlocksImage.Main;
  did?: string;
}) {
  const { zero: zt } = useTheme();
  const ar = block.aspectRatio;
  const ratio = ar && ar.height > 0 ? ar.width / ar.height : 16 / 9;
  console.log("image cid", block.image.ref.toString());
  const src = did ? blobUrl(did, block.image.ref.toString()) : undefined;

  return (
    <View
      style={{
        marginBottom: 12,
        borderRadius: 8,
        overflow: "hidden",
        backgroundColor: zt.bg.muted as string,
        aspectRatio: ratio,
        width: "100%",
      }}
      accessibilityLabel={block.alt}
    >
      {src ? (
        <Image
          source={{ uri: src }}
          accessibilityLabel={block.alt ?? ""}
          style={{ width: "100%", height: "100%" }}
          contentFit="cover"
        />
      ) : (
        <View centered style={{ flex: 1 }}>
          <Text color="muted" size="sm">
            {block.alt ?? "Image"}
          </Text>
        </View>
      )}
    </View>
  );
}

function blobUrl(did: string, cid: string): string {
  return `https://cdn.bsky.app/img/feed_fullsize/plain/${did}/${cid}@jpeg`;
}

function OrderedListBlock({
  block,
  did,
}: {
  block: PlaceStreamBioBlocksOrderedList.Main;
  did?: string;
}) {
  return (
    <View style={{ marginBottom: 12, paddingLeft: 8 }}>
      {block.children.map((item, i) => (
        <OrderedListItem key={i} item={item} index={i + 1} did={did} />
      ))}
    </View>
  );
}

function OrderedListItem({
  item,
  index,
  did,
}: {
  item: PlaceStreamBioBlocksOrderedList.ListItem;
  index: number;
  did?: string;
}) {
  return (
    <View style={{ marginBottom: 4 }}>
      <View direction="row" align="center" style={{ gap: 6 }}>
        <Text size="base" weight="medium" style={{ minWidth: 20 }}>
          {index}.
        </Text>
        <View style={{ flex: 1 }}>
          <ListItemContent block={item.content} did={did} />
          {item.children && item.children.length > 0 && (
            <View style={{ marginTop: 4, paddingLeft: 20 }}>
              {item.children.map((child, i) => (
                <OrderedListItem key={i} item={child} index={i + 1} did={did} />
              ))}
            </View>
          )}
          {item.unorderedListChildren && (
            <View style={{ marginTop: 4 }}>
              <UnorderedListBlock
                block={item.unorderedListChildren}
                did={did}
              />
            </View>
          )}
        </View>
      </View>
    </View>
  );
}

function UnorderedListBlock({
  block,
  did,
}: {
  block: PlaceStreamBioBlocksUnorderedList.Main;
  did?: string;
}) {
  return (
    <View style={{ marginBottom: 12, paddingLeft: 8 }}>
      {block.children.map((item, i) => (
        <UnorderedListItem key={i} item={item} did={did} />
      ))}
    </View>
  );
}

function UnorderedListItem({
  item,
  did,
}: {
  item: PlaceStreamBioBlocksUnorderedList.ListItem;
  did?: string;
}) {
  const isChecklist = item.checked !== undefined;

  return (
    <View style={{ marginBottom: 4 }}>
      <View direction="row" align="center" style={{ gap: 6 }}>
        <Text size="base" style={{ minWidth: 16 }}>
          {isChecklist ? (item.checked ? "☑" : "☐") : "•"}
        </Text>
        <View style={{ flex: 1 }}>
          <ListItemContent block={item.content} did={did} />
          {item.children && item.children.length > 0 && (
            <View style={{ marginTop: 4, paddingLeft: 16 }}>
              {item.children.map((child, i) => (
                <UnorderedListItem key={i} item={child} did={did} />
              ))}
            </View>
          )}
          {item.orderedListChildren && (
            <View style={{ marginTop: 4 }}>
              <OrderedListBlock block={item.orderedListChildren} did={did} />
            </View>
          )}
        </View>
      </View>
    </View>
  );
}

function ListItemContent({
  block,
  did,
}: {
  block:
    | PlaceStreamBioBlocksOrderedList.ListItem["content"]
    | PlaceStreamBioBlocksUnorderedList.ListItem["content"];
  did?: string;
}) {
  const t = (block as any)?.$type as string | undefined;

  switch (t) {
    case "place.stream.bio.blocks.text": {
      const b = block as PlaceStreamBioBlocksText.Main;
      return <RichTextView plaintext={b.plaintext} facets={b.facets} />;
    }
    case "place.stream.bio.blocks.header":
      return <HeaderBlock block={block as PlaceStreamBioBlocksHeader.Main} />;
    case "place.stream.bio.blocks.image":
      return (
        <ImageBlock block={block as PlaceStreamBioBlocksImage.Main} did={did} />
      );
    default:
      return null;
  }
}

function BlockquoteBlock({
  block,
}: {
  block: PlaceStreamBioBlocksBlockquote.Main;
}) {
  const { zero: zt } = useTheme();

  return (
    <View
      style={{
        borderLeftWidth: 3,
        borderLeftColor: (zt.border?.muted ?? "#444") as string,
        paddingLeft: 12,
        marginBottom: 12,
      }}
    >
      <Text color="muted" style={{ fontStyle: "italic" }}>
        <RichTextInline plaintext={block.plaintext} facets={block.facets} />
      </Text>
    </View>
  );
}

function DividerBlock() {
  const { zero: zt } = useTheme();

  return (
    <View
      style={{
        height: 1,
        backgroundColor: (zt.border?.muted ?? "#333") as string,
        marginVertical: 16,
        width: "100%",
      }}
    />
  );
}

function LinkBlock({
  block,
  did,
}: {
  block: PlaceStreamBioBlocksLink.Main;
  did?: string;
}) {
  const { zero: zt } = useTheme();
  const previewSrc =
    did && block.previewImage
      ? blobUrl(did, block.previewImage.ref.toString())
      : undefined;

  return (
    <View
      style={{
        borderRadius: 8,
        borderWidth: 1,
        borderColor: (zt.border?.muted ?? "#333") as string,
        overflow: "hidden",
        marginBottom: 12,
        width: "100%",
      }}
    >
      {previewSrc && (
        <Image
          source={{ uri: previewSrc }}
          accessibilityLabel={block.text ?? ""}
          style={{ width: "100%", height: 160 }}
          contentFit="cover"
        />
      )}
      <View style={{ padding: 12 }}>
        <Text weight="semibold" size="base">
          {block.text || block.url}
        </Text>
        {block.description && (
          <Text color="muted" size="sm" style={{ marginTop: 4 }}>
            {block.description}
          </Text>
        )}
        <Text color="muted" size="xs" style={{ marginTop: 4 }}>
          {block.url}
        </Text>
      </View>
    </View>
  );
}

function SocialLinksBlock({
  block,
}: {
  block: PlaceStreamBioBlocksSocialLinks.Main;
}) {
  return <SocialRow socials={block.links} />;
}

function LivestreamBlock({
  block,
}: {
  block: PlaceStreamBioBlocksLivestream.Main;
}) {
  return (
    <View
      style={{
        padding: 12,
        borderRadius: 8,
        backgroundColor: "rgba(255,0,0,0.1)",
        marginBottom: 12,
      }}
    >
      <Text color="muted" size="sm">
        Livestream: {block.livestream.uri}
      </Text>
    </View>
  );
}

function ScheduleBlock({
  block,
}: {
  block: PlaceStreamBioBlocksSchedule.Main;
}) {
  const DAY_LABELS: Record<string, string> = {
    mon: "Mon",
    tue: "Tue",
    wed: "Wed",
    thu: "Thu",
    fri: "Fri",
    sat: "Sat",
    sun: "Sun",
  };

  const groups = groupSlotsByDay(block.slots);

  return (
    <View style={{ marginBottom: 12 }}>
      <Text size="sm" color="muted" style={{ marginBottom: 8 }}>
        Schedule ({block.timezone})
      </Text>
      <View style={{ gap: 4 }}>
        {Object.entries(groups).map(([day, slots]) => (
          <View key={day} direction="row" style={{ gap: 8 }}>
            <Text size="sm" weight="semibold" style={{ minWidth: 36 }}>
              {DAY_LABELS[day] ?? day}
            </Text>
            <View style={{ flex: 1 }}>
              {slots.map((s, i) => (
                <Text key={i} size="sm">
                  {s.startTime}
                  {s.endTime ? ` - ${s.endTime}` : ""}
                  {s.title ? `  ${s.title}` : ""}
                </Text>
              ))}
            </View>
          </View>
        ))}
      </View>
    </View>
  );
}

function groupSlotsByDay(
  slots: PlaceStreamBioBlocksSchedule.Slot[],
): Record<string, PlaceStreamBioBlocksSchedule.Slot[]> {
  const groups: Record<string, PlaceStreamBioBlocksSchedule.Slot[]> = {};
  for (const s of slots) {
    if (!groups[s.dayOfWeek]) groups[s.dayOfWeek] = [];
    groups[s.dayOfWeek].push(s);
  }
  return groups;
}

function BskyPostBlock({
  block,
}: {
  block: PlaceStreamBioBlocksBskyPost.Main;
}) {
  return (
    <View
      style={{
        padding: 12,
        borderRadius: 8,
        backgroundColor: "rgba(32,139,254,0.1)",
        marginBottom: 12,
      }}
    >
      <Text color="muted" size="sm">
        Bluesky Post: {block.post.uri}
      </Text>
    </View>
  );
}

function EmbedBlock({
  block,
  did,
}: {
  block: PlaceStreamBioBlocksEmbed.Main;
  did?: string;
}) {
  const { zero: zt } = useTheme();
  const previewSrc =
    did && block.previewImage
      ? blobUrl(did, block.previewImage.ref.toString())
      : undefined;

  return (
    <View
      style={{
        borderRadius: 8,
        borderWidth: 1,
        borderColor: (zt.border?.muted ?? "#333") as string,
        overflow: "hidden",
        marginBottom: 12,
        width: "100%",
      }}
    >
      {previewSrc && (
        <Image
          source={{ uri: previewSrc }}
          accessibilityLabel={block.title ?? ""}
          style={{ width: "100%", height: 160 }}
          contentFit="cover"
        />
      )}
      <View style={{ padding: 12 }}>
        {block.title && (
          <Text weight="bold" size="base">
            {block.title}
          </Text>
        )}
        {block.description && (
          <Text color="muted" size="sm" style={{ marginTop: 4 }}>
            {block.description}
          </Text>
        )}
        <Text color="muted" size="xs" style={{ marginTop: 4 }}>
          {block.src}
        </Text>
      </View>
    </View>
  );
}

function RichTextView({
  plaintext,
  facets,
}: {
  plaintext: string;
  facets?: PlaceStreamBioRichtextFacet.Main[];
}) {
  if (!facets || facets.length === 0) {
    return (
      <Text size="base" leading="tight">
        {plaintext}
      </Text>
    );
  }
  return <RichTextPlaintext plaintext={plaintext} facets={facets} />;
}

function RichTextInline({
  plaintext,
  facets,
}: {
  plaintext: string;
  facets?: PlaceStreamBioRichtextFacet.Main[];
}) {
  if (!facets || facets.length === 0) {
    return <>{plaintext}</>;
  }
  return <RichTextPlaintext plaintext={plaintext} facets={facets} />;
}

function RichTextPlaintext({
  plaintext,
  facets,
}: {
  plaintext: string;
  facets: PlaceStreamBioRichtextFacet.Main[];
}) {
  const sorted = [...facets].sort(
    (a, b) => a.index.byteStart - b.index.byteStart,
  );
  const segments: React.ReactNode[] = [];
  let lastEnd = 0;

  for (const facet of sorted) {
    const bs = facet.index.byteStart;
    const be = facet.index.byteEnd;

    const start = byteOffsetToChar(plaintext, bs);
    const end = byteOffsetToChar(plaintext, be);

    if (lastEnd < start) {
      segments.push(
        <Text key={`t-${lastEnd}`} size="base">
          {plaintext.slice(lastEnd, start)}
        </Text>,
      );
    }

    const segmentText = plaintext.slice(start, end);
    const feature = facet.features?.[0];
    const featureType = (feature as any)?.$type as string | undefined;

    if (featureType === "app.bsky.richtext.facet#mention") {
      const did = (feature as any)?.did as string | undefined;
      segments.push(
        <Text key={`f-${bs}`} size="base" customColor="#2b8afe">
          {segmentText}
        </Text>,
      );
    } else if (featureType === "app.bsky.richtext.facet#link") {
      const uri = (feature as any)?.uri as string | undefined;
      segments.push(
        <Text key={`f-${bs}`} size="base" customColor="#2b8afe">
          {segmentText}
        </Text>,
      );
    } else {
      segments.push(
        <Text key={`f-${bs}`} size="base">
          {segmentText}
        </Text>,
      );
    }

    lastEnd = end;
  }

  if (lastEnd < plaintext.length) {
    segments.push(
      <Text key={`t-${lastEnd}`} size="base">
        {plaintext.slice(lastEnd)}
      </Text>,
    );
  }

  return <>{segments}</>;
}

function byteOffsetToChar(str: string, byteOffset: number): number {
  let bytes = 0;
  for (let i = 0; i < str.length; ) {
    if (bytes >= byteOffset) return i;
    const cp = str.codePointAt(i)!;
    if (cp < 0x80) bytes += 1;
    else if (cp < 0x800) bytes += 2;
    else if (cp < 0x10000) bytes += 3;
    else bytes += 4;
    i += cp > 0xffff ? 2 : 1;
  }
  return str.length;
}
