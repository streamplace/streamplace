import {
  Dialog,
  DialogFooter,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
} from "@streamplace/components";
import React, { useState } from "react";
import { Button } from "react-native";

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
      dismissible
      variant="default"
      size="md"
      position="center"
    >
      <DropdownMenuRadioGroup
        value={selectedReason || undefined}
        onValueChange={setSelectedReason}
      >
        {REPORT_REASONS.map((reason) => (
          <DropdownMenuRadioItem key={reason} value={reason}>
            {reason}
          </DropdownMenuRadioItem>
        ))}
      </DropdownMenuRadioGroup>
      <DialogFooter>
        <Button title="Cancel" onPress={handleCancel} color="#888" />
        <Button
          title="Submit"
          onPress={handleSubmit}
          disabled={!selectedReason}
        />
      </DialogFooter>
    </Dialog>
  );
};

export default ReportModal;
