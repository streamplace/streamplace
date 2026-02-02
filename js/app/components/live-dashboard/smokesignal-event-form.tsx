import DateTimePicker from "@react-native-community/datetimepicker";
import {
  Admonition,
  Button,
  Input,
  Text,
  Textarea,
  useTheme,
  zero,
} from "@streamplace/components";
import { useCallback, useMemo, useState } from "react";
import { Linking, Platform, Pressable, View } from "react-native";

const { p, pb, py, gap, layout, r, w } = zero;

const isWeb = Platform.OS === "web";

interface SmokesignalEventFormProps {
  defaultTitle?: string;
  streamUrl: string;
}

// Web-specific input component using HTML5 native pickers
function WebDateTimeInput({
  type,
  value,
  onChange,
  label,
}: {
  type: "date" | "time";
  value: string;
  onChange: (value: string) => void;
  label: string;
}) {
  const { theme } = useTheme();

  return (
    <View style={[gap.all[1], w.percent[100]]}>
      <Text style={{ fontSize: 12, color: theme.colors.textMuted }}>{label}</Text>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        style={{
          padding: 12,
          borderRadius: 6,
          backgroundColor: theme.colors.muted,
          color: theme.colors.foreground,
          border: `1px solid ${theme.colors.border}`,
          width: "100%",
          fontSize: 14,
          outline: "none",
          boxSizing: "border-box" as const,
          colorScheme: "dark",
        }}
      />
    </View>
  );
}

// Native input component using @react-native-community/datetimepicker
function NativeDateTimeInput({
  mode,
  value,
  onChange,
  label,
}: {
  mode: "date" | "time";
  value: Date | undefined;
  onChange: (date: Date | undefined) => void;
  label: string;
}) {
  const { theme, zero: z } = useTheme();
  const [show, setShow] = useState(false);

  const displayValue = value
    ? mode === "date"
      ? value.toLocaleDateString()
      : value.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    : mode === "date"
      ? "Select date"
      : "Select time";

  return (
    <View style={[gap.all[1], w.percent[100]]}>
      <Text style={{ fontSize: 12, color: theme.colors.textMuted }}>{label}</Text>
      <Pressable
        onPress={() => setShow(true)}
        style={[p[3], r.md, z.bg.muted, { borderWidth: 1, borderColor: theme.colors.border }, w.percent[100]]}
      >
        <Text style={{ fontSize: 14, color: value ? theme.colors.foreground : theme.colors.textMuted }}>
          {displayValue}
        </Text>
      </Pressable>
      {show && (
        <DateTimePicker
          value={value || new Date()}
          mode={mode}
          display="default"
          onChange={(event, selectedDate) => {
            setShow(false);
            if (event.type === "set" && selectedDate) {
              onChange(selectedDate);
            }
          }}
        />
      )}
    </View>
  );
}

export function SmokesignalEventForm({
  defaultTitle = "",
  streamUrl,
}: SmokesignalEventFormProps) {
  const { theme, zero: z } = useTheme();

  // Controlled-by-props-until-touched: Show prop values until user edits, then their input.
  // Avoids useEffect syncing that would overwrite user input when props arrive async.
  const [eventTitle, setEventTitle] = useState("");
  const [titleTouched, setTitleTouched] = useState(false);
  const [eventDescription, setEventDescription] = useState("");
  const [descTouched, setDescTouched] = useState(false);

  const defaultDescription = streamUrl ? `Live on Streamplace: ${streamUrl}` : "";

  const handleTitleChange = useCallback((value: string) => {
    setTitleTouched(true);
    setEventTitle(value);
  }, []);

  const handleDescriptionChange = useCallback((value: string) => {
    setDescTouched(true);
    setEventDescription(value);
  }, []);

  const effectiveTitle = titleTouched ? eventTitle : defaultTitle;
  const effectiveDescription = descTouched ? eventDescription : defaultDescription;

  // Web uses string values (YYYY-MM-DD, HH:MM format)
  const [webDate, setWebDate] = useState("");
  const [webStartTime, setWebStartTime] = useState("");
  const [webEndTime, setWebEndTime] = useState("");

  // Native uses Date objects
  const [nativeDate, setNativeDate] = useState<Date | undefined>(undefined);
  const [nativeStartTime, setNativeStartTime] = useState<Date | undefined>(
    undefined,
  );
  const [nativeEndTime, setNativeEndTime] = useState<Date | undefined>(
    undefined,
  );

  // Compute startsAt ISO string
  const startsAt = useMemo(() => {
    if (isWeb) {
      if (!webDate || !webStartTime) return "";
      const dateTime = new Date(`${webDate}T${webStartTime}`);
      if (Number.isNaN(dateTime.getTime())) return "";
      return dateTime.toISOString();
    } else {
      if (!nativeDate || !nativeStartTime) return "";
      const combined = new Date(nativeDate);
      combined.setHours(
        nativeStartTime.getHours(),
        nativeStartTime.getMinutes(),
        0,
        0,
      );
      return combined.toISOString();
    }
  }, [webDate, webStartTime, nativeDate, nativeStartTime]);

  // Compute endsAt ISO string
  const endsAt = useMemo(() => {
    if (isWeb) {
      if (!webDate || !webEndTime) return "";
      const dateTime = new Date(`${webDate}T${webEndTime}`);
      if (Number.isNaN(dateTime.getTime())) return "";
      return dateTime.toISOString();
    } else {
      if (!nativeDate || !nativeEndTime) return "";
      const combined = new Date(nativeDate);
      combined.setHours(
        nativeEndTime.getHours(),
        nativeEndTime.getMinutes(),
        0,
        0,
      );
      return combined.toISOString();
    }
  }, [webDate, webEndTime, nativeDate, nativeEndTime]);

  // Check if end time is valid (after start time)
  const endTimeError = useMemo(() => {
    if (!startsAt || !endsAt) return undefined;
    const start = new Date(startsAt);
    const end = new Date(endsAt);
    if (end <= start) {
      return "End time must be after start time";
    }
    return undefined;
  }, [startsAt, endsAt]);

  const smokesignalIntentUrl = useMemo(() => {
    const eventName = effectiveTitle.trim() || "Livestream";
    const description = effectiveDescription.trim() || "Live on Streamplace";
    const intentUrl = new URL("https://smokesignal.events/event");
    intentUrl.searchParams.set("name", eventName);
    intentUrl.searchParams.set("description", description);
    intentUrl.searchParams.set("mode", "virtual");
    if (startsAt) intentUrl.searchParams.set("starts_at", startsAt);
    if (endsAt && !endTimeError) intentUrl.searchParams.set("ends_at", endsAt);
    if (streamUrl) {
      intentUrl.searchParams.set("link", streamUrl);
      intentUrl.searchParams.set("link_name", "Watch on Streamplace");
    }
    return intentUrl.toString();
  }, [effectiveTitle, effectiveDescription, streamUrl, startsAt, endsAt, endTimeError]);

  const handleOpenSmokesignal = useCallback(() => {
    if (!smokesignalIntentUrl) return;
    if (isWeb && typeof window !== "undefined") {
      window.open(smokesignalIntentUrl, "_blank", "noopener,noreferrer");
      return;
    }
    Linking.openURL(smokesignalIntentUrl);
  }, [smokesignalIntentUrl]);

  const isFormValid = useMemo(() => {
    if (!startsAt) return false;
    if (endTimeError) return false;
    return true;
  }, [startsAt, endTimeError]);

  return (
    <View style={[gap.all[4], w.percent[100], layout.flex.column]}>
      <Text style={{ fontSize: 18, fontWeight: "600", color: theme.colors.foreground }}>
        Smoke Signal Event
      </Text>

      <Admonition variant="info" size="sm">
        <Text size="sm">
          Smoke Signal is a decentralized event platform. Create an event to let
          your followers know when you'll be live.
        </Text>
      </Admonition>

      <View style={[gap.all[3], w.percent[100], layout.flex.column]}>
        <Input
          label="Title"
          value={effectiveTitle}
          onChange={handleTitleChange}
          placeholder="Event title"
          variant="filled"
        />

        <View style={[gap.all[1], w.percent[100]]}>
          <Text style={{ fontSize: 12, color: theme.colors.textMuted }}>Description</Text>
          <Textarea
            value={effectiveDescription}
            onChangeText={handleDescriptionChange}
            placeholder="Optional description"
            placeholderTextColor={theme.colors.textMuted}
            maxLength={280}
            multiline
            style={[
              p[3],
              r.md,
              z.bg.muted,
              { borderWidth: 1, borderColor: theme.colors.border, color: theme.colors.foreground },
              w.percent[100],
              { minHeight: 90, fontSize: 14 },
            ]}
          />
        </View>

        {isWeb ? (
          <>
            <WebDateTimeInput type="date" value={webDate} onChange={setWebDate} label="Date" />
            <WebDateTimeInput type="time" value={webStartTime} onChange={setWebStartTime} label="Start time" />
            <View style={[gap.all[1], w.percent[100]]}>
              <WebDateTimeInput type="time" value={webEndTime} onChange={setWebEndTime} label="End time (optional)" />
              {endTimeError && (
                <Text style={{ fontSize: 12, color: theme.colors.destructive }}>
                  {endTimeError}
                </Text>
              )}
            </View>
          </>
        ) : (
          <>
            <NativeDateTimeInput mode="date" value={nativeDate} onChange={setNativeDate} label="Date" />
            <NativeDateTimeInput mode="time" value={nativeStartTime} onChange={setNativeStartTime} label="Start time" />
            <View style={[gap.all[1], w.percent[100]]}>
              <NativeDateTimeInput mode="time" value={nativeEndTime} onChange={setNativeEndTime} label="End time (optional)" />
              {endTimeError && (
                <Text style={{ fontSize: 12, color: theme.colors.destructive }}>
                  {endTimeError}
                </Text>
              )}
            </View>
          </>
        )}
      </View>

      <Button
        onPress={handleOpenSmokesignal}
        disabled={!isFormValid}
        style={[
          z.bg.muted,
          r.md,
          py[3],
          w.percent[100],
          layout.flex.center,
          { opacity: isFormValid ? 1 : 0.5 },
        ]}
      >
        <Text style={{ fontSize: 16, fontWeight: "600", color: theme.colors.foreground }}>
          Create Smoke Signal Event
        </Text>
      </Button>
      <Text style={[pb[4], { fontSize: 12, color: theme.colors.textMuted }]}>
        Opens Smoke Signal in a new tab.
      </Text>
    </View>
  );
}

export default SmokesignalEventForm;
