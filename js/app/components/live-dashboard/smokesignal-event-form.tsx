import {
  Admonition,
  Button,
  Input,
  Text,
  Textarea,
  zero,
} from "@streamplace/components";
import { useCallback, useMemo, useState } from "react";
import { Linking, Platform, View } from "react-native";

const { p, py, gap, layout, bg, borders, text, r, w } = zero;

const isWeb = Platform.OS === "web";

interface SmokesignalEventFormProps {
  defaultTitle?: string;
  streamUrl: string;
}

interface ValidationErrors {
  date?: string;
  startTime?: string;
  endTime?: string;
}

const DATE_REGEX = /^\d{4}-\d{2}-\d{2}$/;
const TIME_REGEX = /^\d{2}:\d{2}$/;

function validateDate(value: string): string | undefined {
  if (!value) return undefined;
  if (!DATE_REGEX.test(value)) {
    return "Use YYYY-MM-DD format";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "Invalid date";
  }
  return undefined;
}

function validateTime(value: string): string | undefined {
  if (!value) return undefined;
  if (!TIME_REGEX.test(value)) {
    return "Use HH:MM format";
  }
  const [hours, minutes] = value.split(":").map(Number);
  if (hours < 0 || hours > 23 || minutes < 0 || minutes > 59) {
    return "Invalid time";
  }
  return undefined;
}

export function SmokesignalEventForm({
  defaultTitle,
  streamUrl,
}: SmokesignalEventFormProps) {
  const [eventTitle, setEventTitle] = useState("");
  const [eventDescription, setEventDescription] = useState("");
  const [eventDate, setEventDate] = useState("");
  const [eventStartTime, setEventStartTime] = useState("");
  const [eventEndTime, setEventEndTime] = useState("");
  const [errors, setErrors] = useState<ValidationErrors>({});

  const timezone = useMemo(
    () => Intl.DateTimeFormat().resolvedOptions().timeZone,
    [],
  );

  const handleDateChange = useCallback((value: string) => {
    setEventDate(value);
    const error = validateDate(value);
    setErrors((prev) => ({ ...prev, date: error }));
  }, []);

  const handleStartTimeChange = useCallback(
    (value: string) => {
      setEventStartTime(value);
      const error = validateTime(value);
      setErrors((prev) => {
        const newErrors = { ...prev, startTime: error };
        // Re-validate end time relationship if both times are present and valid
        if (!error && eventEndTime && !validateTime(eventEndTime)) {
          if (value >= eventEndTime) {
            newErrors.endTime = "End time must be after start time";
          } else {
            newErrors.endTime = undefined;
          }
        }
        return newErrors;
      });
    },
    [eventEndTime],
  );

  const handleEndTimeChange = useCallback(
    (value: string) => {
      setEventEndTime(value);
      let error = validateTime(value);
      // Check if end time is after start time
      if (!error && value && eventStartTime && !validateTime(eventStartTime)) {
        if (value <= eventStartTime) {
          error = "End time must be after start time";
        }
      }
      setErrors((prev) => ({ ...prev, endTime: error }));
    },
    [eventStartTime],
  );

  const startsAt = useMemo(() => {
    if (!eventDate || !eventStartTime) return "";
    if (errors.date || errors.startTime) return "";
    const dateTime = new Date(`${eventDate}T${eventStartTime}`);
    if (Number.isNaN(dateTime.getTime())) return "";
    return dateTime.toISOString();
  }, [eventDate, eventStartTime, errors.date, errors.startTime]);

  const endsAt = useMemo(() => {
    if (!eventDate || !eventEndTime) return "";
    if (errors.date || errors.endTime) return "";
    const dateTime = new Date(`${eventDate}T${eventEndTime}`);
    if (Number.isNaN(dateTime.getTime())) return "";
    return dateTime.toISOString();
  }, [eventDate, eventEndTime, errors.date, errors.endTime]);

  const smokesignalIntentUrl = useMemo(() => {
    const eventName = eventTitle.trim() || defaultTitle?.trim() || "Livestream";
    const description =
      eventDescription.trim() ||
      (streamUrl ? `Live on Streamplace: ${streamUrl}` : "Live on Streamplace");
    const intentUrl = new URL("https://smokesignal.events/event");
    intentUrl.searchParams.set("name", eventName);
    intentUrl.searchParams.set("description", description);
    intentUrl.searchParams.set("mode", "virtual");
    if (startsAt) intentUrl.searchParams.set("starts_at", startsAt);
    if (endsAt) intentUrl.searchParams.set("ends_at", endsAt);
    if (streamUrl) {
      intentUrl.searchParams.set("link", streamUrl);
      intentUrl.searchParams.set("link_name", "Watch on Streamplace");
    }
    return intentUrl.toString();
  }, [eventTitle, eventDescription, defaultTitle, streamUrl, startsAt, endsAt]);

  const handleOpenSmokesignal = useCallback(() => {
    if (!smokesignalIntentUrl) return;
    if (isWeb && typeof window !== "undefined") {
      window.open(smokesignalIntentUrl, "_blank", "noopener,noreferrer");
      return;
    }
    Linking.openURL(smokesignalIntentUrl);
  }, [smokesignalIntentUrl]);

  const isFormValid = useMemo(() => {
    if (!eventDate || !eventStartTime) return false;
    if (errors.date || errors.startTime || errors.endTime) return false;
    if (!startsAt) return false;
    return true;
  }, [eventDate, eventStartTime, errors, startsAt]);

  return (
    <View style={[gap.all[4], w.percent[100], layout.flex.column]}>
      <Text style={[text.white, { fontSize: 18, fontWeight: "600" }]}>
        SmokeSignal event
      </Text>

      <Admonition variant="info" size="sm">
        <Text size="sm">
          SmokeSignal is a decentralized event platform. Create an event to let
          your followers know when you'll be live.
        </Text>
      </Admonition>

      <View style={[gap.all[3], w.percent[100], layout.flex.column]}>
        <View style={[gap.all[1], w.percent[100]]}>
          <Text style={[text.neutral[300], { fontSize: 12 }]}>Title</Text>
          <Input
            value={eventTitle}
            onChange={(value) => setEventTitle(value)}
            placeholder="Event title"
            variant="filled"
            inputStyle={[
              p[3],
              r.md,
              bg.neutral[800],
              text.white,
              borders.width.thin,
              borders.color.neutral[600],
              w.percent[100],
            ]}
          />
        </View>

        <View style={[gap.all[1], w.percent[100]]}>
          <Text style={[text.neutral[300], { fontSize: 12 }]}>Description</Text>
          <Textarea
            value={eventDescription}
            onChangeText={setEventDescription}
            placeholder="Optional description"
            maxLength={280}
            multiline
            style={[
              p[3],
              r.md,
              bg.neutral[800],
              text.white,
              borders.width.thin,
              borders.color.neutral[600],
              w.percent[100],
              { minHeight: 90, fontSize: 14 },
            ]}
          />
        </View>

        <View style={[gap.all[1], w.percent[100]]}>
          <Text style={[text.neutral[300], { fontSize: 12 }]}>Date</Text>
          <Input
            value={eventDate}
            onChange={handleDateChange}
            placeholder="YYYY-MM-DD"
            variant="filled"
            error={errors.date}
            inputStyle={[
              p[3],
              r.md,
              bg.neutral[800],
              text.white,
              borders.width.thin,
              borders.color.neutral[600],
              w.percent[100],
            ]}
          />
        </View>

        <View style={[gap.all[1], w.percent[100]]}>
          <Text style={[text.neutral[300], { fontSize: 12 }]}>Start time</Text>
          <Input
            value={eventStartTime}
            onChange={handleStartTimeChange}
            placeholder="HH:MM"
            variant="filled"
            error={errors.startTime}
            inputStyle={[
              p[3],
              r.md,
              bg.neutral[800],
              text.white,
              borders.width.thin,
              borders.color.neutral[600],
              w.percent[100],
            ]}
          />
        </View>

        <View style={[gap.all[1], w.percent[100]]}>
          <Text style={[text.neutral[300], { fontSize: 12 }]}>End time</Text>
          <Input
            value={eventEndTime}
            onChange={handleEndTimeChange}
            placeholder="HH:MM"
            variant="filled"
            error={errors.endTime}
            inputStyle={[
              p[3],
              r.md,
              bg.neutral[800],
              text.white,
              borders.width.thin,
              borders.color.neutral[600],
              w.percent[100],
            ]}
          />
        </View>

        <Text style={[text.neutral[400], { fontSize: 12 }]}>
          Times are in {timezone}
        </Text>
      </View>

      <Button
        onPress={handleOpenSmokesignal}
        disabled={!isFormValid}
        style={[
          bg.neutral[800],
          r.md,
          py[3],
          w.percent[100],
          layout.flex.center,
          { opacity: isFormValid ? 1 : 0.5 },
        ]}
      >
        <Text style={[text.white, { fontSize: 16, fontWeight: "600" }]}>
          Create SmokeSignal event
        </Text>
      </Button>
      <Text style={[text.neutral[400], { fontSize: 12 }]}>
        Opens SmokeSignal in a new tab.
      </Text>
    </View>
  );
}

export default SmokesignalEventForm;
