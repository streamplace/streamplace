import { useLivestreamStore, usePlayerStore } from "@streamplace/components";
import { useState } from "react";
import { useCreateStreamRecord } from "../streamplace-store";

export function useLivestreamInfo() {
  const ingest = usePlayerStore((x) => x.ingestConnectionState);
  const profile = useLivestreamStore((x) => x.profile);
  const ingestStarting = usePlayerStore((x) => x.ingestStarting);
  const setIngestStarting = usePlayerStore((x) => x.setIngestStarting);

  const createStreamRecord = useCreateStreamRecord();

  const [title, setTitle] = useState<string>("");
  const [showCountdown, setShowCountdown] = useState(false);
  const [recordSubmitted, setRecordSubmitted] = useState(false);

  const handleSubmit = async () => {
    try {
      if (title !== "") {
        setRecordSubmitted(true);
        await createStreamRecord(title);
      }
    } catch (error) {
      console.error("Error creating livestream:", error);
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
      handleSubmit();
      return;
    }
    setIngestStarting(false);
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
