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
import { ScrollView, TouchableOpacity, View } from "react-native";
import { zero } from "../../..";
import { useSubmitReport } from "../../../livestream-store";
import {
  Button,
  DialogFooter,
  ResponsiveDialog,
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
  subject: ComAtprotoModerationCreateReport.InputSchema["subject"];
  context?: {
    text?: string;
    content?: string;
    message?: string;
    [key: string]: any;
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
  const [currentStep, setCurrentStep] = useState<Step>(1);
  const [selectedReason, setSelectedReason] = useState<string | null>(null);
  const [additionalComments, setAdditionalComments] = useState<string>("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const { theme } = useTheme();
  const [reportSubmitted, setReportSubmitted] = useState(false);

  const submitReport = useSubmitReport();

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
      submitReport(
        subject,
        selectedReason,
        additionalComments.trim() || undefined,
      );
      setReportSubmitted(true);
      setCurrentStep(4);
    } catch (error) {
      console.error("Failed to submit report:", error);
      setSubmitError("Failed to submit report. Please try again.");
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
            <View style={[zero.mt[3], { width: "100%", maxWidth: 300 }]}>
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
                      backgroundColor: "rgba(0, 122, 255, 0.1)",
                    },
                  ]}
                >
                  <View>
                    {selectedReason === reason.value ? (
                      <CheckCircle size="18" />
                    ) : (
                      <Circle size="18" />
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
                          color: "rgba(255,255,255,0.7)",
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
            <View
              style={[
                zero.mt[3],
                zero.ml[9],
                zero.pr[4],
                { width: "100%", maxWidth: 300 },
              ]}
            >
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
                  { fontSize: 12, color: "rgba(255,255,255,0.5)" },
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
            <View
              style={[
                zero.mt[3],
                zero.ml[9],
                zero.pr[4],
                { width: "100%", maxWidth: 300 },
              ]}
            >
              <View
                style={[
                  zero.mb[4],
                  zero.p[3],
                  zero.borderRadius[8],
                  { backgroundColor: "rgba(255,255,255,0.05)" },
                ]}
              >
                <Text style={[zero.mb[2], { fontWeight: "600" }]}>Reason:</Text>
                <Text style={[zero.mb[1]]}>{selectedReasonData?.label}</Text>
                <Text
                  style={[{ fontSize: 14, color: "rgba(255,255,255,0.7)" }]}
                >
                  {selectedReasonData?.description}
                </Text>
              </View>

              {additionalComments.trim() && (
                <View
                  style={[
                    zero.mb[4],
                    zero.p[3],
                    zero.borderRadius[8],
                    { backgroundColor: "rgba(255,255,255,0.05)" },
                  ]}
                >
                  <Text style={[zero.mb[2], { fontWeight: "600" }]}>
                    Additional Comments:
                  </Text>
                  <Text style={[{ color: "rgba(255,255,255,0.9)" }]}>
                    {additionalComments.trim()}
                  </Text>
                </View>
              )}

              {submitError && (
                <Text style={[zero.mt[2], { color: "red", fontSize: 14 }]}>
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
                { width: "100%", maxWidth: 300 },
              ]}
            >
              <CheckCircle size={48} color="#00C851" style={[zero.mb[4]]} />
              <Text style={[zero.mb[2], { fontSize: 18, fontWeight: "600" }]}>
                Report Submitted
              </Text>
              <Text
                style={[
                  zero.mb[4],
                  {
                    fontSize: 14,
                    color: "rgba(255,255,255,0.7)",
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
                    color: "rgba(255,255,255,0.6)",
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
        <View style={[zero.mb[4], { minWidth: 320 }]}>
          <View style={[zero.layout.flex.row, zero.mb[3]]}>
            <View
              style={[
                {
                  width: 24,
                  height: 24,
                  borderRadius: 12,
                  alignItems: "center",
                  justifyContent: "center",
                  backgroundColor: "#00C851",
                  marginRight: 12,
                },
              ]}
            >
              <CheckCircle size={14} color="white" />
            </View>
            <View style={[zero.mb[3]]}>
              <Text
                style={[{ color: "white", fontWeight: "600", fontSize: 14 }]}
              >
                {currentStepData.title}
              </Text>
              <Text
                style={[
                  {
                    color: "rgba(255,255,255,0.7)",
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
      <View style={[zero.mb[4], zero.px[4], { minWidth: 320 }]}>
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
                        ? "#00C851"
                        : step.number === currentStep
                          ? "#007AFF"
                          : "rgba(255,255,255,0.3)",
                    marginRight: 12,
                  },
                ]}
              >
                {step.number < currentStep ? (
                  <CheckCircle size={14} color="white" />
                ) : (
                  <Text
                    style={[
                      {
                        fontSize: 12,
                        fontWeight: "600",
                        color:
                          step.number === currentStep
                            ? "white"
                            : "rgba(255,255,255,0.6)",
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
                          ? "white"
                          : "rgba(255,255,255,0.6)",
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
                          ? "rgba(255,255,255,0.7)"
                          : "rgba(255,255,255,0.4)",
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
                        ? "#00C851"
                        : "rgba(255,255,255,0.2)",
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
            <Button variant="secondary" onPress={handleCancel}>
              <Text>Cancel</Text>
            </Button>
            <Button
              variant="primary"
              onPress={handleNext}
              disabled={!canProceed()}
            >
              <Text>Next</Text>
              <ChevronRight size={16} style={[{ marginLeft: 4 }]} />
            </Button>
          </>
        );

      case 2:
        return (
          <>
            <Button variant="secondary" onPress={handleBack}>
              <ChevronLeft size={16} style={[{ marginRight: 4 }]} />
              <Text>Back</Text>
            </Button>
            <Button variant="primary" onPress={handleNext}>
              <Text>Next</Text>
              <ChevronRight size={16} style={[{ marginLeft: 4 }]} />
            </Button>
          </>
        );

      case 3:
        return (
          <>
            <Button
              variant="secondary"
              onPress={handleBack}
              disabled={isSubmitting}
            >
              <ChevronLeft size={16} style={[{ marginRight: 4 }]} />
              <Text>Back</Text>
            </Button>
            <Button
              variant="primary"
              onPress={handleSubmitReport}
              disabled={isSubmitting || !subject}
            >
              {isSubmitting ? (
                <>
                  <Loader2 style={[{ marginRight: 8 }]} />
                  <Text>Submitting...</Text>
                </>
              ) : (
                <Text>Submit Report</Text>
              )}
            </Button>
          </>
        );

      case 4:
        return (
          <>
            <Button variant="destructive" onPress={handleBlock}>
              <Text>Block User</Text>
            </Button>
            <Button variant="primary" onPress={handleFinish}>
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

  let aturi = (subject as any)?.uri;

  // Lexicon NSID (second to last part )
  let lexid = aturi?.split("/")[3] || null;

  // take place.stream.lex.chat -> Lex chat
  let lexidParts = lexid?.split(".");
  let lexSubject = lexidParts?.[3];
  let lexSubType = lexidParts?.[2];

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={getStepTitle}
      description={getStepDescription}
      showCloseButton={!isSubmitting}
      variant="default"
      size="md"
      dismissible={currentStep === 1 && !isSubmitting}
      style={{ minWidth: 360, maxWidth: 400 }}
      position="center"
    >
      <ModalContent style={[zero.pb[2]]}>
        <ScrollView
          style={{ flex: 1 }}
          contentContainerStyle={{ flexGrow: 1 }}
          showsVerticalScrollIndicator={false}
        >
          {!subject ? (
            <View
              style={[
                zero.mx[4],
                zero.mb[4],
                zero.p[3],
                zero.borderRadius[8],
                { backgroundColor: "rgba(255,255,255,0.05)" },
              ]}
            >
              <Text
                style={[
                  {
                    fontSize: 14,
                    color: "rgba(255,255,255,0.7)",
                    textAlign: "center",
                  },
                ]}
              >
                No content selected for reporting
              </Text>
            </View>
          ) : (
            <View
              style={[
                zero.p[2],
                zero.mb[4],
                zero.r.md,
                { backgroundColor: "rgba(255,255,255,0.05)" },
              ]}
            >
              <Text style={[{ fontSize: 14, fontWeight: "500" }]}>
                {lexSubType?.charAt(0).toUpperCase() + lexSubType?.slice(1)}{" "}
                {lexSubject}
              </Text>

              {/* Show chat message content */}
              {subject &&
                (subject as any).context?.record?.text &&
                (subject as any).context?.author.handle && (
                  <Text
                    style={[
                      zero.mt[2],
                      zero.p[2],
                      zero.r.sm,
                      {
                        fontSize: 13,
                        color: "rgba(255,255,255,0.8)",
                        backgroundColor: "rgba(255,255,255,0.03)",
                        fontStyle: "italic",
                      },
                    ]}
                    numberOfLines={3}
                  >
                    {(subject as any).context?.author.handle}: "
                    {(subject as any).context?.record?.text}"
                  </Text>
                )}
            </View>
          )}
          {VerticalStepper}
        </ScrollView>
      </ModalContent>
      <DialogFooter>{renderFooterButtons}</DialogFooter>
    </Dialog>
  );
};

export default ReportModal;
