import {
  ComAtprotoModerationCreateReport,
  ComAtprotoModerationDefs,
} from "@atproto/api";
import {
  CheckCircle,
  ChevronLeft,
  ChevronRight,
  Circle,
  Loader2,
} from "lucide-react-native";
import React, { useCallback, useMemo, useState } from "react";
import { TouchableOpacity, View } from "react-native";
import { PlaceStreamChatMessage, PlaceStreamLivestream } from "streamplace";
import { useDID, zero } from "../../..";
import { useSubmitReport } from "../../../livestream-store";
import { usePDSAgent } from "../../../streamplace-store/xrpc";
import {
  Button,
  Dialog,
  DialogBody,
  DialogClose,
  DialogContent,
  DialogHeader,
  DialogOverlay,
  DialogTitle,
  Text,
  Textarea,
  useTheme,
} from "../../ui";

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
  onBlock?: () => void;
  subject: ComAtprotoModerationCreateReport.InputSchema["subject"] & {
    record?: PlaceStreamChatMessage.Record | PlaceStreamLivestream.Record;
    author?: {
      handle?: string;
      did?: string;
      [key: string]: unknown;
    };
  };
  context?: {
    text?: string;
    content?: string;
    message?: string;
    author?: {
      handle?: string;
      [key: string]: unknown;
    };
    record?: {
      text?: string;
      [key: string]: unknown;
    };
    [key: string]: unknown;
  };
  title?: string;
  description?: string;
}

type Step = 1 | 2 | 3 | 4;

export const ReportModal: React.FC<ReportModalProps> = ({
  open,
  onOpenChange,
  onSubmit,
  onBlock,
  subject,
  context,
  title = "Report",
  description = "Why are you submitting this report?",
}) => {
  const { theme } = useTheme();
  const [currentStep, setCurrentStep] = useState<Step>(1);
  const [selectedReason, setSelectedReason] = useState<string | null>(null);
  const [additionalComments, setAdditionalComments] = useState<string>("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const { theme } = useTheme();
  const [reportSubmitted, setReportSubmitted] = useState(false);

  const submitReport = useSubmitReport();
  const pdsAgent = usePDSAgent();
  const userDID = useDID();

  const isLoggedIn = pdsAgent && userDID;

  const handleCancel = useCallback(() => {
    resetForm();
    onOpenChange(false);
  }, [onOpenChange]);

  const resetForm = () => {
    setCurrentStep(1);
    setSelectedReason(null);
    setAdditionalComments("");
    setSubmitError(null);
    setIsSubmitting(false);
    setReportSubmitted(false);
  };

  const handleNext = useCallback(() => {
    if (currentStep < 5) {
      setCurrentStep((prev) => (prev + 1) as Step);
    }
  }, [currentStep]);

  const handleBack = useCallback(() => {
    if (currentStep > 1) {
      setCurrentStep((prev) => (prev - 1) as Step);
    }
  }, [currentStep]);

  const handleSubmitReport = useCallback(async () => {
    if (!selectedReason) return;

    setIsSubmitting(true);
    setSubmitError(null);

    try {
      await submitReport(
        subject,
        selectedReason,
        additionalComments.trim() || undefined,
      );
      setReportSubmitted(true);
      setCurrentStep(4);
    } catch (error) {
      console.error("Failed to submit report:", error);
      setSubmitError("Failed to submit report. " + (error as Error).message);
    } finally {
      setIsSubmitting(false);
    }
  }, [
    selectedReason,
    additionalComments,
    submitReport,
    subject,
    setCurrentStep,
    setReportSubmitted,
    setIsSubmitting,
    setSubmitError,
  ]);

  const handleBlock = useCallback(() => {
    onBlock?.();
    resetForm();
    onOpenChange(false);
  }, [onBlock, onOpenChange]);

  const handleFinish = useCallback(() => {
    resetForm();
    onOpenChange(false);
  }, [onOpenChange]);

  const getStepTitle = useMemo(() => {
    switch (currentStep) {
      case 1:
        return "Select Reason";
      case 2:
        return "Additional Details";
      case 3:
        return "Review Report";
      case 4:
        return "Report Submitted";
      default:
        return title;
    }
  }, [currentStep, title]);

  const getStepDescription = useMemo(() => {
    switch (currentStep) {
      case 1:
        return "Why are you submitting this report?";
      case 2:
        return "Provide additional context (optional)";
      case 3:
        return "Please review your report before submitting";
      case 4:
        return "Your report has been submitted successfully";
      default:
        return description;
    }
  }, [currentStep, description]);

  const canProceed = () => {
    switch (currentStep) {
      case 1:
        return selectedReason !== null;
      case 2:
        return true; // Comments are optional
      case 3:
        return true;
      default:
        return true;
    }
  };

  const VerticalStepper = useMemo(() => {
    const onlyCurrent = currentStep === 4;
    const steps = [
      {
        number: 1,
        title: "Select Reason",
        description: "Choose report category",
      },
      { number: 2, title: "Add Details", description: "Optional comments" },
      { number: 3, title: "Review", description: "Confirm your report" },
      { number: 4, title: "Complete", description: "Report submitted" },
    ];

    const getStepContent = (stepNumber: number) => {
      switch (stepNumber) {
        case 1:
          return (
            <View style={[zero.mt[3], { width: "100%" }]}>
              {REPORT_REASONS.map((reason) => (
                <TouchableOpacity
                  key={reason.value}
                  onPress={() => setSelectedReason(reason.value)}
                  style={[
                    zero.layout.flex.row,
                    zero.gap.all[2],
                    zero.py[2],
                    zero.px[2],
                    zero.my[1],
                    zero.r.md,
                    zero.layout.flex.alignCenter,
                    zero.borders.color.gray[
                      selectedReason === reason.value ? 700 : 800
                    ],
                    zero.borders.width.thin,
                    selectedReason === reason.value && {
                      backgroundColor: theme.colors.primary + "1A",
                    },
                  ]}
                >
                  <View>
                    {selectedReason === reason.value ? (
                      <CheckCircle size="18" color={theme.colors.primary} />
                    ) : (
                      <Circle size="18" color={theme.colors.text} />
                    )}
                  </View>
                  <View
                    style={[
                      zero.layout.flex.column,
                      zero.gap.all[1],
                      zero.flex[1],
                      zero.w.percent[100],
                    ]}
                  >
                    <Text
                      style={[
                        { fontWeight: "400", fontSize: 14, lineHeight: 14 },
                      ]}
                    >
                      {reason.label}
                    </Text>
                    <Text
                      style={[
                        {
                          fontSize: 12,
                          color: theme.colors.textMuted,
                          // why is the line height soo high?
                          lineHeight: 16,
                        },
                      ]}
                    >
                      {reason.description}
                    </Text>
                  </View>
                </TouchableOpacity>
              ))}
            </View>
          );

        case 2:
          return (
            <View style={[zero.mt[3], { width: "100%" }]}>
              <Text style={[zero.mb[2]]}>Additional Comments (optional)</Text>
              <Textarea
                maxLength={500}
                numberOfLines={4}
                value={additionalComments}
                onChangeText={setAdditionalComments}
                placeholder="Provide additional context for this report..."
              />
              <Text
                style={[
                  zero.mt[2],
                  { fontSize: 12, color: theme.colors.textMuted },
                ]}
              >
                {additionalComments.length}/500
              </Text>
            </View>
          );

        case 3:
          const selectedReasonData = REPORT_REASONS.find(
            (r) => r.value === selectedReason,
          );
          return (
            <View style={[zero.mt[3], { width: "100%" }]}>
              <View
                style={[
                  zero.mb[4],
                  zero.p[3],
                  zero.r.md,
                  { backgroundColor: theme.colors.background },
                ]}
              >
                <Text style={[zero.mb[2], { fontWeight: "600" }]}>Reason:</Text>
                <Text style={[zero.mb[1]]}>{selectedReasonData?.label}</Text>
                <Text style={[{ fontSize: 14, color: theme.colors.textMuted }]}>
                  {selectedReasonData?.description}
                </Text>
              </View>

              {additionalComments.trim() && (
                <View
                  style={[
                    zero.mb[4],
                    zero.p[3],
                    zero.r.md,
                    { backgroundColor: theme.colors.background },
                  ]}
                >
                  <Text style={[zero.mb[2], { fontWeight: "600" }]}>
                    Additional Comments:
                  </Text>
                  <Text style={[{ color: theme.colors.text }]}>
                    {additionalComments.trim()}
                  </Text>
                </View>
              )}

              {submitError && (
                <Text
                  style={[
                    zero.mt[2],
                    { color: theme.colors.destructive, fontSize: 14 },
                  ]}
                >
                  {submitError}
                </Text>
              )}
            </View>
          );

        case 4:
          return (
            <View
              style={[
                zero.mt[3],

                zero.layout.flex.alignCenter,
                { width: "100%" },
              ]}
            >
              <CheckCircle
                size={48}
                color={theme.colors.success}
                style={[zero.mb[4]]}
              />
              <Text style={[zero.mb[2], { fontSize: 18, fontWeight: "600" }]}>
                Report Submitted
              </Text>
              <Text
                style={[
                  zero.mb[4],
                  {
                    fontSize: 14,
                    color: theme.colors.textMuted,
                    textAlign: "center",
                  },
                ]}
              >
                Thank you for helping keep our community safe. We'll review your
                report and take appropriate action.
              </Text>
              <Text style={[zero.mb[2], { textAlign: "center" }]}>
                Would you like to block this user as well?
              </Text>
              <Text
                style={[
                  {
                    fontSize: 14,
                    color: theme.colors.textMuted,
                    textAlign: "center",
                  },
                ]}
              >
                Blocking a user will prevent them from interacting with you
                globally. Blocks are public.
              </Text>
            </View>
          );

        default:
          return null;
      }
    };

    if (onlyCurrent) {
      const currentStepData = steps.find((s) => s.number === currentStep);
      if (!currentStepData) return <></>;

      return (
        <View style={[zero.mb[4]]}>
          <View style={[zero.layout.flex.row, zero.mb[3]]}>
            <View
              style={[
                {
                  width: 24,
                  height: 24,
                  borderRadius: 12,
                  alignItems: "center",
                  justifyContent: "center",
                  backgroundColor: theme.colors.success,
                  marginRight: 12,
                },
              ]}
            >
              <CheckCircle size={14} color={theme.colors.primaryForeground} />
            </View>
            <View style={[zero.mb[3]]}>
              <Text
                style={[
                  { color: theme.colors.text, fontWeight: "600", fontSize: 14 },
                ]}
              >
                {currentStepData.title}
              </Text>
              <Text
                style={[
                  {
                    color: theme.colors.textMuted,
                    marginTop: 4,
                    fontSize: 12,
                  },
                ]}
              >
                {currentStepData.description}
              </Text>
            </View>
          </View>
          {getStepContent(currentStep)}
        </View>
      );
    }

    return (
      <View style={[zero.mb[4], zero.px[4]]}>
        {steps.map((step, index) => (
          <View
            key={step.number}
            style={[zero.mb[step.number === currentStep ? 0 : 3]]}
          >
            <View style={[zero.layout.flex.row]}>
              <View
                style={[
                  {
                    width: 24,
                    height: 24,
                    borderRadius: 12,
                    alignItems: "center",
                    justifyContent: "center",
                    backgroundColor:
                      step.number < currentStep
                        ? theme.colors.success
                        : step.number === currentStep
                          ? theme.colors.primary
                          : theme.colors.muted,
                    marginRight: 12,
                  },
                ]}
              >
                {step.number < currentStep ? (
                  <CheckCircle
                    size={14}
                    color={theme.colors.successForeground}
                  />
                ) : (
                  <Text
                    style={[
                      {
                        fontSize: 12,
                        fontWeight: "600",
                        color:
                          step.number === currentStep
                            ? theme.colors.primaryForeground
                            : theme.colors.textMuted,
                      },
                    ]}
                  >
                    {step.number}
                  </Text>
                )}
              </View>

              {/* Step Content */}
              <View style={[zero.flex[1]]}>
                <Text
                  style={[
                    {
                      fontWeight: step.number === currentStep ? "600" : "400",
                      fontSize: 14,
                      color:
                        step.number <= currentStep
                          ? theme.colors.text
                          : theme.colors.textMuted,
                    },
                  ]}
                >
                  {step.title}
                </Text>
                <Text
                  style={[
                    {
                      fontSize: 12,
                      color:
                        step.number <= currentStep
                          ? theme.colors.textMuted
                          : theme.colors.textDisabled,
                    },
                  ]}
                >
                  {step.description}
                </Text>
              </View>
            </View>

            {/* Vertical Line */}
            {index < steps.length - 1 && (
              <View
                style={[
                  {
                    position: "absolute",
                    left: 11,
                    top: 24,
                    width: 2,
                    backgroundColor:
                      step.number < currentStep
                        ? theme.colors.success
                        : theme.colors.muted,
                    zIndex: -1,
                  },
                ]}
              />
            )}

            {/* Expanded Step Content */}
            {step.number === currentStep && (
              <View style={[zero.mb[4]]}>{getStepContent(step.number)}</View>
            )}
          </View>
        ))}
      </View>
    );
  }, [
    currentStep,
    selectedReason,
    additionalComments,
    setSelectedReason,
    setAdditionalComments,
    submitError,
    isSubmitting,
    subject,
  ]);

  const renderFooterButtons = useMemo(() => {
    switch (currentStep) {
      case 1:
        return (
          <>
            <Button
              style={[zero.flex.grow[1]]}
              variant="secondary"
              onPress={handleCancel}
            >
              <Text>Cancel</Text>
            </Button>
            <Button
              style={[zero.flex.grow[1]]}
              variant="primary"
              onPress={handleNext}
              disabled={!canProceed()}
              rightIcon={<ChevronRight size={16} color={theme.colors.text} />}
            >
              <Text>Next</Text>
            </Button>
          </>
        );

      case 2:
        return (
          <>
            <Button
              style={[zero.flex.grow[1]]}
              variant="secondary"
              onPress={handleBack}
            >
              <Text>Back</Text>
            </Button>
            <Button
              style={[zero.flex.grow[1]]}
              variant="primary"
              onPress={handleNext}
              disabled={!canProceed()}
              rightIcon={<ChevronRight size={16} style={[{ marginLeft: 4 }]} />}
            >
              <Text>Next</Text>
            </Button>
          </>
        );

      case 3:
        return (
          <>
            <Button
              style={[zero.flex.grow[1]]}
              variant="secondary"
              onPress={handleBack}
              disabled={isSubmitting}
            >
              <ChevronLeft size={16} style={[{ marginRight: 4 }]} />
              <Text>Back</Text>
            </Button>
            <Button
              style={[zero.flex.grow[1]]}
              variant="primary"
              onPress={handleSubmitReport}
              disabled={isSubmitting || !subject}
              leftIcon={
                isSubmitting ? <Loader2 style={[{ marginRight: 8 }]} /> : null
              }
            >
              {isSubmitting ? (
                <Text>Submitting...</Text>
              ) : (
                <Text>Submit Report</Text>
              )}
            </Button>
          </>
        );

      case 4:
        return (
          <>
            <Button
              style={[zero.flex.grow[1]]}
              variant="destructive"
              onPress={handleBlock}
            >
              <Text>Block User</Text>
            </Button>
            <Button
              style={[zero.flex.grow[1]]}
              variant="primary"
              onPress={handleFinish}
            >
              <Text>Done</Text>
            </Button>
          </>
        );

      default:
        return null;
    }
  }, [
    currentStep,
    selectedReason,
    canProceed,
    handleNext,
    handleCancel,
    handleBack,
    handleSubmitReport,
    isSubmitting,
    handleFinish,
    handleBlock,
  ]);

  let aturi = (subject as { uri?: string })?.uri;

  // Lexicon NSID (second to last part )
  let lexid = aturi?.split("/")[3] || null;

  // take place.stream.lex.chat -> Lex chat
  let lexidParts = lexid?.split(".");
  let lexSubject = lexidParts?.[3];
  let lexSubType = lexidParts?.[2];
  console.log(subject);
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      dismissible={currentStep === 1 && !isSubmitting}
      onClose={() => onOpenChange(false)}
      variant="default"
      size="md"
      position="center"
    >
      <DialogOverlay
        dismissible={currentStep === 1 && !isSubmitting}
        onDismiss={() => onOpenChange(false)}
      >
        <DialogContent
          size="full"
          position="top"
          style={{
            width: "100%",
            maxWidth: "100%",
            backgroundColor: "transparent",
            margin: 0,
            borderRadius: 0,
          }}
        >
          <DialogHeader withBorder={false}>
            <DialogTitle>Report</DialogTitle>
            {!isSubmitting && (
              <DialogClose onClose={() => onOpenChange(false)} />
            )}
          </DialogHeader>

          {!isLoggedIn ? (
            <View
              style={[
                zero.p[4],
                zero.borderRadius[8],
                zero.flex.grow[1],
                zero.layout.flex.center,
              ]}
            >
              <Text center size="2xl">
                Sorry, but you need to be logged in to submit a report.
              </Text>
            </View>
          ) : !subject ? (
            <DialogBody scrollable>
              <View
                style={[
                  zero.mb[4],
                  zero.p[3],
                  zero.borderRadius[8],
                  { backgroundColor: theme.colors.background },
                ]}
              >
                <Text
                  style={[
                    {
                      fontSize: 14,
                      color: theme.colors.textMuted,
                      textAlign: "center",
                    },
                  ]}
                >
                  No content selected for reporting
                </Text>
              </View>
              {/* Step content */}
              {isLoggedIn && VerticalStepper}
            </DialogBody>
          ) : (
            <DialogBody scrollable>
              <View
                style={[
                  zero.p[2],
                  zero.mb[4],
                  zero.r.md,
                  { backgroundColor: theme.colors.background },
                ]}
              >
                <Text style={[{ fontSize: 14, fontWeight: "500" }]}>
                  {(lexSubType?.charAt(0).toUpperCase() ?? "") +
                    (lexSubType?.slice(1) ?? "")}{" "}
                  {lexSubject}
                  {subject.author?.handle && (
                    <Text
                      style={[
                        {
                          fontSize: 13,
                          color: theme.colors.textMuted,
                          fontWeight: "400",
                        },
                      ]}
                    ></Text>
                  )}
                </Text>

                {/* Show record content */}
                {subject.record && (
                  <Text
                    style={[
                      zero.mt[2],
                      zero.p[2],
                      zero.r.sm,
                      {
                        fontSize: 13,
                        color: theme.colors.text,
                        backgroundColor: theme.colors.muted,
                      },
                    ]}
                    numberOfLines={3}
                  >
                    {subject.record.$type === "place.stream.chat.message" &&
                      subject.record.text &&
                      subject.author && (
                        <Text
                          style={{
                            fontSize: 13,
                            color: theme.colors.text,
                            backgroundColor: theme.colors.muted,
                            fontWeight: "500",
                          }}
                        >
                          {subject.author.handle}:{" "}
                        </Text>
                      )}

                    {subject.record.$type === "place.stream.chat.message" &&
                      subject.record.text &&
                      subject.record.text}
                    {subject.record.$type === "place.stream.livestream" &&
                      subject.record.title &&
                      `${subject.record.title}`}
                  </Text>
                )}
              </View>
              {/* Step content */}
              {isLoggedIn && VerticalStepper}
            </DialogBody>
          )}

          {/* Footer with buttons */}
          {isLoggedIn ? (
            <View
              style={[
                zero.layout.flex.row,
                zero.gap.all[4],
                zero.px[4],
                zero.pt[4],
              ]}
            >
              {renderFooterButtons}
            </View>
          ) : (
            <View
              style={[
                zero.layout.flex.row,
                zero.gap.all[4],
                zero.px[4],
                zero.pt[4],
              ]}
            >
              <Button
                onPress={() => onOpenChange(false)}
                style={[zero.flex.grow[1]]}
              >
                Close
              </Button>
            </View>
          )}
        </DialogContent>
      </DialogOverlay>
    </Dialog>
  );
};

export default ReportModal;
