import { useState } from "react";
import { useLivestreamStore } from "../livestream-store";
import { usePlayerStore } from "../player-store";
import { useCreateStreamRecord } from "../streamplace-store";

export function useLivestreamInfo(url?: string) {
  const ingest = usePlayerStore((x) => x.ingestConnectionState);
  const profile = useLivestreamStore((x) => x.profile);
  const ingestStarting = usePlayerStore((x) => x.ingestStarting);
  const setIngestStarting = usePlayerStore((x) => x.setIngestStarting);
  const setIngestLive = usePlayerStore((x) => x.setIngestLive);

  const createStreamRecord = useCreateStreamRecord();

  const [title, setTitle] = useState<string>("");
  const [showCountdown, setShowCountdown] = useState(false);
  const [recordSubmitted, setRecordSubmitted] = useState(false);

  const handleSubmit = async () => {
    try {
      if (title !== "") {
        setRecordSubmitted(true);
        // Create the livestream record with title and custom url if available
        await createStreamRecord({
          title,
          canonicalUrl: url || undefined,
        });
      }
    } catch (error) {
      console.error("Error creating livestream:", error);
      throw new Error("Failed to create livestream record");
    } finally {
      setRecordSubmitted(false);
    }
  };

  const toggleGoLive = (
    keyboardHeight?: number,
    closeKeyboard?: () => void,
  ) => {
    if (!ingestStarting) {
      // Optionally close keyboard if provided
      if (closeKeyboard) closeKeyboard();
      setShowCountdown(true);
      setIngestStarting(true);
      setIngestLive(true);
      // wait ~3 seconds before announcing
      setTimeout(() => {
        handleSubmit();
      }, 3000);
    } else {
      setIngestStarting(false);
      setIngestLive(false);
    }
  };

  return {
    ingest,
    profile,
    title,
    setTitle,
    showCountdown,
    setShowCountdown,
    recordSubmitted,
    setRecordSubmitted,
    ingestStarting,
    setIngestStarting,
    handleSubmit,
    toggleGoLive,
  };
}
