import {
  Button,
  Dialog,
  DialogFooter,
  ModalContent,
  Text,
  Textarea,
  zero,
} from "@streamplace/components";
import { CheckCircle, Circle } from "@tamagui/lucide-icons";
import React, { useState } from "react";
import { TouchableOpacity, View } from "react-native";

const REPORT_REASONS = [
  "Terrorism",
  "Nudity or Sexually Explicit",
  "Hateful Conduct",
  "Bullying or Harassment",
  "Violence or Gore",
  "Self-Harm",
  "Spam, Scams, Bots, or Tampering",
  "Miscategorized Content",
  "Missing or Incorrect Content Classification Label",
  "Search",
];

interface ReportModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (reason: string) => void;
  title?: string;
  description?: string;
}

export const ReportModal: React.FC<ReportModalProps> = ({
  open,
  onOpenChange,
  onSubmit,
  title = "Report",
  description = "Why are you submitting this report?",
}) => {
  const [selectedReason, setSelectedReason] = useState<string | null>(null);

  const handleCancel = () => {
    setSelectedReason(null);
    onOpenChange(false);
  };

  const handleSubmit = () => {
    if (selectedReason) {
      onSubmit(selectedReason);
      setSelectedReason(null);
      onOpenChange(false);
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
            key={reason}
            onPress={() => setSelectedReason(reason)}
            style={[
              zero.layout.flex.row,
              zero.gap.all[2],
              zero.py[1],
              zero.px[2],
              zero.borderRadius[8],
              zero.layout.flex.alignCenter,
            ]}
          >
            <View>
              {selectedReason === reason ? <CheckCircle /> : <Circle />}
            </View>
            <Text>{reason}</Text>
          </TouchableOpacity>
        ))}

        <View style={[zero.pb[4], zero.mt[4], zero.px[2]]}>
          <Text style={[zero.mb[2]]}>Additional Comments (optional)</Text>
          <Textarea maxLength={500} numberOfLines={2} />
        </View>
      </ModalContent>
      <DialogFooter>
        <Button variant="secondary" onPress={handleCancel}>
          <Text>Cancel</Text>
        </Button>
        <Button
          variant="primary"
          onPress={handleSubmit}
          disabled={!selectedReason}
        >
          Submit
        </Button>
      </DialogFooter>
    </Dialog>
  );
};

export default ReportModal;
