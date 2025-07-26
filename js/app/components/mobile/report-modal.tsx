import { ComAtprotoModerationDefs } from "@atproto/api";
import {
  Button,
  Dialog,
  DialogFooter,
  ModalContent,
  Text,
  Textarea,
  useReportChatMessage,
  zero,
} from "@streamplace/components";
import { CheckCircle, Circle, Loader2 } from "@tamagui/lucide-icons";
import React, { useState } from "react";
import { TouchableOpacity, View } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";

// AT Protocol moderation reason types with proper labels
const REPORT_REASONS = [
  {
    value: ComAtprotoModerationDefs.REASONSPAM,
    label: "Spam",
    description: "Excessive unwanted promotion, replies, mentions",
  },
  {
    value: ComAtprotoModerationDefs.REASONVIOLATION,
    label: "Rule Violation",
    description: "Direct, blatant violation of laws or terms of service",
  },
  {
    value: ComAtprotoModerationDefs.REASONMISLEADING,
    label: "Misleading Content",
    description: "Misleading identity, affiliation, or content",
  },
  {
    value: ComAtprotoModerationDefs.REASONSEXUAL,
    label: "Sexual Content",
    description: "Unwanted or mislabeled sexual content",
  },
  {
    value: ComAtprotoModerationDefs.REASONRUDE,
    label: "Harassment",
    description: "Rude, harassing, explicit, or otherwise unwelcoming behavior",
  },
  {
    value: ComAtprotoModerationDefs.REASONOTHER,
    label: "Other",
    description: "Reports not falling under another report category",
  },
];

interface ReportModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit?: (reason: string, additionalComments?: string) => void;
  message?: ChatMessageViewHydrated;
  title?: string;
  description?: string;
}

export const ReportModal: React.FC<ReportModalProps> = ({
  open,
  onOpenChange,
  onSubmit,
  message,
  title = "Report",
  description = "Why are you submitting this report?",
}) => {
  const [selectedReason, setSelectedReason] = useState<string | null>(null);
  const [additionalComments, setAdditionalComments] = useState<string>("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const reportChatMessage = useReportChatMessage();

  const handleCancel = () => {
    setSelectedReason(null);
    setAdditionalComments("");
    setSubmitError(null);
    onOpenChange(false);
  };

  const handleSubmit = async () => {
    if (!selectedReason) return;

    setIsSubmitting(true);
    setSubmitError(null);

    try {
      if (message) {
        // Use the integrated chat message reporting
        await reportChatMessage(
          message,
          selectedReason,
          additionalComments.trim() || undefined,
        );
      } else if (onSubmit) {
        // Fallback to custom onSubmit prop
        onSubmit(selectedReason, additionalComments.trim() || undefined);
      }

      // Reset form and close modal on success
      setSelectedReason(null);
      setAdditionalComments("");
      onOpenChange(false);
    } catch (error) {
      console.error("Failed to submit report:", error);
      setSubmitError("Failed to submit report. Please try again.");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      description={description}
      showCloseButton
      variant="default"
      size="md"
      dismissible={false}
      position="center"
    >
      <ModalContent style={[zero.pb[2]]}>
        {REPORT_REASONS.map((reason) => (
          <TouchableOpacity
            key={reason.value}
            onPress={() => setSelectedReason(reason.value)}
            style={[
              zero.layout.flex.row,
              zero.gap.all[2],
              zero.py[3],
              zero.px[3],
              zero.borderRadius[8],
              zero.layout.flex.alignCenter,
              selectedReason === reason.value && {
                backgroundColor: "rgba(0, 122, 255, 0.1)",
              },
            ]}
          >
            <View>
              {selectedReason === reason.value ? <CheckCircle /> : <Circle />}
            </View>
            <View
              style={[zero.layout.flex.column, zero.gap.all[1], zero.flex[1]]}
            >
              <Text style={[{ fontWeight: "600" }]}>{reason.label}</Text>
              <Text style={[{ fontSize: 14, color: "rgba(255,255,255,0.7)" }]}>
                {reason.description}
              </Text>
            </View>
          </TouchableOpacity>
        ))}

        <View style={[zero.pb[4], zero.mt[4], zero.px[2]]}>
          <Text style={[zero.mb[2]]}>Additional Comments (optional)</Text>
          <Textarea
            maxLength={500}
            numberOfLines={3}
            value={additionalComments}
            onChangeText={setAdditionalComments}
            placeholder="Provide additional context for this report..."
          />
          {submitError && (
            <Text style={[zero.mt[2], { color: "red", fontSize: 14 }]}>
              {submitError}
            </Text>
          )}
        </View>
      </ModalContent>
      <DialogFooter>
        <Button
          variant="secondary"
          onPress={handleCancel}
          disabled={isSubmitting}
        >
          <Text>Cancel</Text>
        </Button>
        <Button
          variant="primary"
          onPress={handleSubmit}
          disabled={!selectedReason || isSubmitting}
        >
          {isSubmitting ? (
            <>
              <Loader2 style={[{ marginRight: 8 }]} />
              <Text>Submitting...</Text>
            </>
          ) : (
            <Text>Submit</Text>
          )}
        </Button>
      </DialogFooter>
    </Dialog>
  );
};

export default ReportModal;
