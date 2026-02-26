import { useState } from "react";
import { useLivestreamStore } from "../livestream-store";
import { usePlayerStore } from "../player-store";
import { useCreateStreamRecord, useEndLivestream } from "../streamplace-store";

export function useLivestreamInfo(url?: string) {
  const ingest = usePlayerStore((x) => x.ingestConnectionState);
  const profile = useLivestreamStore((x) => x.profile);
  const endLivestream = useEndLivestream();
  const setLocalLivestreamURI = useLivestreamStore(
    (x) => x.setLocalLivestreamURI,
  );
  const createStreamRecord = useCreateStreamRecord();

  const [title, setTitle] = useState<string>("");
  const [showCountdown, setShowCountdown] = useState(false);
  const [recordSubmitted, setRecordSubmitted] = useState(false);

  const handleSubmit = async () => {
    try {
      if (title !== "") {
        setRecordSubmitted(true);
        // Create the livestream record with title and custom url if available
        const { uri } = await createStreamRecord({
          title,
          canonicalUrl: url || undefined,
        });
        setLocalLivestreamURI(uri);
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
    // Optionally close keyboard if provided
    if (closeKeyboard) closeKeyboard();
    setShowCountdown(true);
    // wait ~3 seconds before announcing
    setTimeout(() => {
      handleSubmit();
    }, 3000);
  };

  // Stop the current broadcast
  const toggleStopStream = () => {
    console.log("Stopping stream...");
    endLivestream();
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
    handleSubmit,
    toggleGoLive,
    toggleStopStream,
  };
}
