import {
  useContentMetadata,
  useContentMetadataError,
  useGetContentMetadata,
  useIsCreating,
  useIsUpdating,
  useSaveContentMetadata,
} from "@streamplace/components";
import { ChevronDown, Info } from "@tamagui/lucide-icons";
import { useToastController } from "@tamagui/toast";
import {
  createStreamKeyRecord,
  selectStoredKey,
} from "features/bluesky/blueskySlice";
import React, { useEffect, useState } from "react";
import { useWindowDimensions } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { schemas } from "streamplace";
import {
  Adapt,
  Button,
  Checkbox,
  Input,
  Label,
  Paragraph,
  Select,
  Sheet,
  TextArea,
  View,
  XStack,
  YStack,
  isWeb,
} from "tamagui";

// Type definitions for metadata
export interface DistributionPolicy {
  deleteAfter?: string;
}

export interface Rights {
  creator?: string;
  copyrightNotice?: string;
  copyrightYear?: number;
  license?: string;
  creditLine?: string;
}

export interface ContentMetadata {
  contentWarnings: { warnings: string[] };
  distributionPolicy: DistributionPolicy;
  contentRights: Rights;
}

interface ContentMetadataFormProps {
  onMetadataChange: (metadata: ContentMetadata) => void;
  initialMetadata?: Partial<ContentMetadata>;
  showUpdateButton?: boolean;
}

// Content warnings derived from lexicon schema
const CONTENT_WARNINGS = (() => {
  // Find the content warnings schema
  const contentWarningsSchema = schemas.find(
    (schema) => schema.id === "place.stream.metadata.contentWarnings",
  );
  if (!contentWarningsSchema?.defs) {
    throw new Error(
      "Could not find place.stream.metadata.contentWarnings schema",
    );
  }

  const contentWarningConstants = [
    { constant: "place.stream.metadata.contentWarnings#death", label: "Death" },
    {
      constant: "place.stream.metadata.contentWarnings#drugUse",
      label: "Drug Use",
    },
    {
      constant: "place.stream.metadata.contentWarnings#fantasyViolence",
      label: "Fantasy Violence",
    },
    {
      constant: "place.stream.metadata.contentWarnings#flashingLights",
      label: "Flashing Lights",
    },
    {
      constant: "place.stream.metadata.contentWarnings#language",
      label: "Language",
    },
    {
      constant: "place.stream.metadata.contentWarnings#nudity",
      label: "Nudity",
    },
    {
      constant: "place.stream.metadata.contentWarnings#PII",
      label: "Personally Identifiable Information",
    },
    {
      constant: "place.stream.metadata.contentWarnings#sexuality",
      label: "Sexuality",
    },
    {
      constant: "place.stream.metadata.contentWarnings#suffering",
      label: "Upsetting or Disturbing",
    },
    {
      constant: "place.stream.metadata.contentWarnings#violence",
      label: "Violence",
    },
  ];

  return contentWarningConstants.map(({ constant, label }) => {
    // Extract the key from the constant by splitting on '#'
    const key = constant.split("#")[1];
    const def = contentWarningsSchema.defs[key];
    const description = def?.description || `Description for ${label}`;
    return {
      value: constant,
      label: label,
      description: description,
    };
  });
})();

// License options derived from lexicon schema
const LICENSE_OPTIONS = (() => {
  // Find the content rights schema
  const contentRightsSchema = schemas.find(
    (schema) => schema.id === "place.stream.metadata.contentRights",
  );
  if (!contentRightsSchema?.defs) {
    throw new Error(
      "Could not find place.stream.metadata.contentRights schema",
    );
  }

  const licenseConstants = [
    {
      constant: "place.stream.metadata.contentRights#all-rights-reserved",
      label: "All Rights Reserved",
    },
    {
      constant: "place.stream.metadata.contentRights#cc0_1__0",
      label: "CC0 (Public Domain) 1.0",
    },
    {
      constant: "place.stream.metadata.contentRights#cc-by_4__0",
      label: "CC BY 4.0",
    },
    {
      constant: "place.stream.metadata.contentRights#cc-by-sa_4__0",
      label: "CC BY-SA 4.0",
    },
    {
      constant: "place.stream.metadata.contentRights#cc-by-nc_4__0",
      label: "CC BY-NC 4.0",
    },
    {
      constant: "place.stream.metadata.contentRights#cc-by-nc-sa_4__0",
      label: "CC BY-NC-SA 4.0",
    },
    {
      constant: "place.stream.metadata.contentRights#cc-by-nd_4__0",
      label: "CC BY-ND 4.0",
    },
    {
      constant: "place.stream.metadata.contentRights#cc-by-nc-nd_4__0",
      label: "CC BY-NC-ND 4.0",
    },
  ];

  const options = licenseConstants.map(({ constant, label }) => {
    // Extract the key from the constant by splitting on '#'
    const key = constant.split("#")[1];
    const def = contentRightsSchema.defs[key];
    const description = def?.description || `Description for ${label}`;
    return {
      value: constant,
      label: label,
      description: description,
    };
  });

  // Add custom license option
  options.push({
    value: "custom",
    label: "Custom License",
    description:
      "Custom license. Define your own terms for how others can use, adapt, or share your content.",
  });

  return options;
})();

// Helper to generate custom date options starting from today
const generateDateOptions = () => {
  const months = Array.from({ length: 12 }, (_, i) =>
    new Date(2000, i, 1).toLocaleString("en-US", { month: "long" }),
  );
  const days = Array.from({ length: 31 }, (_, i) => i + 1);
  const hours = Array.from({ length: 12 }, (_, i) => i + 1);
  const minutes = Array.from({ length: 60 }, (_, i) =>
    String(i).padStart(2, "0"),
  );

  return { months, days, hours, minutes };
};

export default function ContentMetadataForm({
  onMetadataChange,
  initialMetadata,
  showUpdateButton = false,
}: ContentMetadataFormProps) {
  const { width } = useWindowDimensions();
  const isWide = width > 1020;
  const useTwoColumns = isWide;
  const dispatch = useAppDispatch();
  const toast = useToastController();
  const isUpdating = useIsUpdating();
  const isCreating = useIsCreating();
  const error = useContentMetadataError();
  const lastCreatedRecord = useContentMetadata();
  const storedKey = useAppSelector(selectStoredKey);

  const saveContentMetadata = useSaveContentMetadata();
  const getContentMetadata = useGetContentMetadata();

  const isLoading = isUpdating || isCreating;
  // Check if we have metadata based on the fetched record
  const hasMetadata = Boolean(lastCreatedRecord?.record);
  const hasStreamKey = Boolean(storedKey);

  const [contentWarnings, setContentWarnings] = useState<string[]>(
    initialMetadata?.contentWarnings?.warnings || [],
  );
  const [distributionPolicy, setDistributionPolicy] =
    useState<DistributionPolicy>(initialMetadata?.distributionPolicy || {});
  const [contentRights, setContentRights] = useState<Rights>(
    initialMetadata?.contentRights || {},
  );
  const [selectedLicense, setSelectedLicense] = useState(
    initialMetadata?.contentRights?.license || "",
  );

  // Date picker state - only set deleteAfter when user actually selects
  const [customDay, setCustomDay] = useState("");
  const [customMonth, setCustomMonth] = useState("");
  const [customYear, setCustomYear] = useState("");
  const [customHour, setCustomHour] = useState("");
  const [customMinute, setCustomMinute] = useState("");
  const [customPeriod, setCustomPeriod] = useState<"AM" | "PM">("AM");

  // Track if we've already populated the form to prevent overriding user input
  const [hasPopulated, setHasPopulated] = useState(false);

  const { months, days, hours, minutes } = generateDateOptions();

  // Fetch existing metadata on component mount
  useEffect(() => {
    // Always check if metadata record exists when component mounts
    getContentMetadata({ rkey: "self" }).catch((error) => {
      console.warn("Failed to fetch content metadata:", error);
    });
  }, [getContentMetadata]);

  // Pre-populate form fields when existing metadata is fetched
  useEffect(() => {
    if (lastCreatedRecord?.record && !hasPopulated) {
      const record = lastCreatedRecord.record;

      // Update content warnings
      if (record.contentWarnings && record.contentWarnings.warnings) {
        setContentWarnings(
          Array.isArray(record.contentWarnings.warnings)
            ? record.contentWarnings.warnings
            : [],
        );
      }

      // Update distribution policy
      if (record.distributionPolicy) {
        setDistributionPolicy({
          deleteAfter: record.distributionPolicy.deleteAfter,
        });

        // Parse delete after date if it exists
        if (record.distributionPolicy.deleteAfter) {
          const expiryDate = new Date(record.distributionPolicy.deleteAfter);
          if (!isNaN(expiryDate.getTime())) {
            setCustomMonth(months[expiryDate.getMonth()]);
            setCustomDay(String(expiryDate.getDate()));
            setCustomYear(String(expiryDate.getFullYear()));

            let hours = expiryDate.getHours();
            const period = hours >= 12 ? "PM" : "AM";
            if (hours > 12) hours -= 12;
            if (hours === 0) hours = 12;

            setCustomHour(String(hours));
            setCustomMinute(String(expiryDate.getMinutes()).padStart(2, "0"));
            setCustomPeriod(period);
          }
        }
      }

      // Update content rights
      if (record.contentRights) {
        const rights = {
          creator: record.contentRights.creator || "",
          copyrightNotice: record.contentRights.copyrightNotice || "",
          copyrightYear: record.contentRights.copyrightYear || undefined,
          license: record.contentRights.license || "",
          creditLine: record.contentRights.creditLine || "",
        };
        setContentRights(rights);
        setSelectedLicense(rights.license);
      }

      // Update the parent component with the loaded metadata
      const metadata: ContentMetadata = {
        contentWarnings: {
          warnings: Array.isArray(record.contentWarnings?.warnings)
            ? record.contentWarnings.warnings
            : [],
        },
        distributionPolicy: {
          deleteAfter: record.distributionPolicy?.deleteAfter,
        },
        contentRights: record.contentRights || {},
      };
      onMetadataChange(metadata);

      // Mark as populated to prevent future overwrites
      setHasPopulated(true);
    }
  }, [lastCreatedRecord, onMetadataChange, months, hasPopulated]);

  const handleContentWarningChange = (warning: string, checked: boolean) => {
    const newWarnings = checked
      ? [...contentWarnings, warning]
      : contentWarnings.filter((w) => w !== warning);
    setContentWarnings(newWarnings);
    updateMetadata({ contentWarnings: { warnings: newWarnings } });
  };

  const handleDistributionPolicyChange = (
    updates: Partial<DistributionPolicy>,
  ) => {
    const newPolicy = { ...distributionPolicy, ...updates };
    setDistributionPolicy(newPolicy);
    updateMetadata({ distributionPolicy: newPolicy });
  };

  const handleContentRightsChange = (updates: Partial<Rights>) => {
    const newRights = { ...contentRights, ...updates };
    setContentRights(newRights);
    updateMetadata({ contentRights: newRights });
  };

  const updateMetadata = (updates: Partial<ContentMetadata>) => {
    const metadata: ContentMetadata = {
      contentWarnings: updates.contentWarnings || { warnings: contentWarnings },
      distributionPolicy: updates.distributionPolicy || distributionPolicy,
      contentRights: updates.contentRights || contentRights,
    };
    onMetadataChange(metadata);
  };

  // Update deleteAfter only when user has selected all date/time fields
  React.useEffect(() => {
    // Only set deleteAfter if user has selected all required fields
    if (customMonth && customDay && customYear && customHour && customMinute) {
      const monthIndex = months.indexOf(customMonth);
      // Handle 12-hour format correctly
      let hour24 = parseInt(customHour);
      if (customPeriod === "PM" && hour24 !== 12) {
        hour24 += 12;
      } else if (customPeriod === "AM" && hour24 === 12) {
        hour24 = 0;
      }

      const date = new Date(
        parseInt(customYear),
        monthIndex,
        parseInt(customDay),
        hour24,
        parseInt(customMinute),
      );
      const isoString = date.toISOString();
      handleDistributionPolicyChange({
        deleteAfter: isoString,
      });
    } else {
      // Clear deleteAfter if fields are incomplete
      handleDistributionPolicyChange({
        deleteAfter: undefined,
      });
    }
  }, [
    customDay,
    customMonth,
    customYear,
    customHour,
    customMinute,
    customPeriod,
  ]);

  const handleSaveMetadata = async () => {
    try {
      // Validate date if expiry is set
      if (
        customMonth &&
        customDay &&
        customYear &&
        customHour &&
        customMinute
      ) {
        const monthIndex = months.indexOf(customMonth);
        const year = parseInt(customYear);
        const day = parseInt(customDay);
        const daysInMonth = new Date(year, monthIndex + 1, 0).getDate();

        if (day > daysInMonth) {
          throw new Error(`Invalid date: ${customMonth} ${day}, ${customYear}`);
        }
      }

      // First ensure we have a stream key
      if (!hasStreamKey) {
        await dispatch(createStreamKeyRecord({ store: true })).unwrap();
      }

      const contentRightsData = {
        ...(contentRights.creator && { creator: contentRights.creator }),
        ...(contentRights.copyrightNotice && {
          copyrightNotice: contentRights.copyrightNotice,
        }),
        ...(contentRights.copyrightYear && {
          copyrightYear: contentRights.copyrightYear,
        }),
        ...(contentRights.license && { license: contentRights.license }),
        ...(contentRights.creditLine && {
          creditLine: contentRights.creditLine,
        }),
      };

      const distributionPolicyData = {
        ...(distributionPolicy.deleteAfter && {
          deleteAfter: distributionPolicy.deleteAfter,
        }),
      };

      const metadataPayload = {
        ...(contentWarnings.length > 0 && { contentWarnings }),
        ...(Object.keys(distributionPolicyData).length > 0 && {
          distributionPolicy: distributionPolicyData,
        }),
        ...(Object.keys(contentRightsData).length > 0 && {
          contentRights: contentRightsData,
        }),
      };

      // Use saveContentMetadata which handles both create and update with putRecord
      await saveContentMetadata({
        contentWarnings,
        distributionPolicy,
        contentRights,
        _isUpdate: hasMetadata,
      });
      toast.show("Success", {
        message: "Content metadata saved successfully",
      });
    } catch (error) {
      toast.show("Error", {
        message: `Failed to save metadata: ${error.message || error}`,
      });
    }
  };

  return (
    <View style={{ flex: 1 }}>
      {/* Two-column layout */}
      <View
        flexDirection={useTwoColumns ? "row" : "column"}
        gap="$1"
        paddingHorizontal={isWide ? "$1" : "$1"}
        paddingVertical="$0.5"
        justifyContent="center"
        alignItems={useTwoColumns ? "flex-start" : "center"}
      >
        {/* Left column: Content Warnings and Distribution Policy */}
        <View
          f={1}
          minWidth={0}
          gap="$1.5"
          alignItems="center"
          justifyContent="flex-start"
          w={useTwoColumns ? 400 : "100%"}
          style={{
            marginTop: 2,
            ...(useTwoColumns ? {} : { marginLeft: 5 }),
          }}
        >
          {/* Content Warnings Section */}
          <YStack gap="$0.5" w="100%">
            <XStack alignItems="baseline" gap="$1">
              <Paragraph fontWeight="600" fontSize="$2.5">
                Content Warnings
              </Paragraph>
              <Paragraph color="$gray11" fontSize="$1">
                (select any that apply)
              </Paragraph>
            </XStack>

            {/* Ultra-compact grid layout for content warnings */}
            <View flexDirection="row" flexWrap="wrap" gap={2} w="100%">
              {CONTENT_WARNINGS.map((warning) => {
                const isChecked = contentWarnings.includes(warning.value);
                return (
                  <View key={warning.value} width="49.5%" minHeight={20}>
                    <XStack
                      alignItems="center"
                      gap="$1"
                      paddingVertical={2}
                      paddingHorizontal={4}
                      backgroundColor={isChecked ? "$gray3" : "transparent"}
                      borderRadius="$1"
                      cursor="pointer"
                      hoverStyle={{ backgroundColor: "$gray3" }}
                      pressStyle={{ backgroundColor: "$gray4" }}
                      onPress={() => {
                        handleContentWarningChange(warning.value, !isChecked);
                      }}
                    >
                      <Checkbox
                        id={`warning-${warning.value}`}
                        checked={isChecked}
                        size="$2"
                        borderWidth={1.5}
                        borderColor={isChecked ? "$blue10" : "$gray8"}
                        backgroundColor={isChecked ? "$blue3" : "transparent"}
                        pointerEvents="none"
                        disabled
                      >
                        <Checkbox.Indicator>
                          <View
                            backgroundColor="$blue10"
                            width={6}
                            height={6}
                            borderRadius="$0.5"
                          />
                        </Checkbox.Indicator>
                      </Checkbox>
                      <Label
                        fontSize="$1"
                        cursor="pointer"
                        flex={1}
                        userSelect="none"
                        pointerEvents="none"
                      >
                        {warning.label}
                      </Label>
                    </XStack>
                  </View>
                );
              })}
            </View>

            {/* Content Warning Descriptions */}
            {contentWarnings.length > 0 && (
              <YStack gap="$0.5" mt="$1">
                {contentWarnings.map((warningValue) => {
                  const warning = CONTENT_WARNINGS.find(
                    (w) => w.value === warningValue,
                  );
                  return warning?.description ? (
                    <XStack
                      key={warningValue}
                      gap="$1"
                      p="$1"
                      backgroundColor="$orange2"
                      borderRadius="$1"
                      alignItems="flex-start"
                    >
                      <Info size={12} color="$orange11" mt={2} />
                      <YStack flex={1}>
                        <Paragraph
                          fontSize="$1"
                          fontWeight="500"
                          color="$orange12"
                        >
                          {warning.label}
                        </Paragraph>
                        <Paragraph fontSize="$1" color="$orange11">
                          {warning.description}
                        </Paragraph>
                      </YStack>
                    </XStack>
                  ) : null;
                })}
              </YStack>
            )}
          </YStack>

          {/* Distribution Policy Section */}
          <YStack gap="$0.5" w="100%">
            <XStack alignItems="baseline" gap="$1">
              <Paragraph fontWeight="600" fontSize="$2.5">
                Distribution
              </Paragraph>
              <Paragraph color="$gray11" fontSize="$1">
                (broadcast & archive)
              </Paragraph>
            </XStack>

            <YStack gap="$1">
              <Label fontSize="$1.5" fontWeight="500">
                Broadcast Expiry (Optional)
              </Label>
              <Paragraph fontSize="$1" color="$gray11">
                Leave empty for no restrictions, or set a specific end date and
                time
              </Paragraph>

              <YStack
                gap="$1"
                mt="$1"
                p="$1"
                backgroundColor="$gray2"
                borderRadius="$2"
              >
                <Label fontSize="$1" fontWeight="500">
                  Select End Date & Time
                </Label>

                {/* Date Selection */}
                <XStack gap="$1" alignItems="center">
                  <Select
                    value={customMonth}
                    onValueChange={setCustomMonth}
                    size="$1"
                  >
                    <Select.Trigger width={90} size="$2">
                      <Select.Value placeholder="Month" fontSize="$1" />
                    </Select.Trigger>
                    <Select.Content zIndex={200000}>
                      <Select.Viewport>
                        <Select.Group>
                          {months.map((month) => (
                            <Select.Item
                              key={month}
                              value={month}
                              index={months.indexOf(month)}
                            >
                              <Select.ItemText fontSize="$1">
                                {month}
                              </Select.ItemText>
                            </Select.Item>
                          ))}
                        </Select.Group>
                      </Select.Viewport>
                    </Select.Content>
                  </Select>

                  <Select
                    value={customDay}
                    onValueChange={setCustomDay}
                    size="$1"
                  >
                    <Select.Trigger width={50} size="$2">
                      <Select.Value placeholder="Day" fontSize="$1" />
                    </Select.Trigger>
                    <Select.Content zIndex={200000}>
                      <Select.Viewport>
                        <Select.Group>
                          {days.map((day) => (
                            <Select.Item
                              key={day}
                              value={String(day)}
                              index={day - 1}
                            >
                              <Select.ItemText fontSize="$1">
                                {day}
                              </Select.ItemText>
                            </Select.Item>
                          ))}
                        </Select.Group>
                      </Select.Viewport>
                    </Select.Content>
                  </Select>

                  <Input
                    placeholder={String(new Date().getFullYear())}
                    value={customYear}
                    onChangeText={setCustomYear}
                    size="$2"
                    fontSize="$1"
                    width={65}
                    maxLength={4}
                    keyboardType="numeric"
                    borderWidth={1}
                    borderColor="$gray8"
                    backgroundColor="$gray2"
                    focusStyle={{
                      borderColor: "$blue10",
                      backgroundColor: "$gray3",
                    }}
                  />
                </XStack>

                {/* Time Selection */}
                <XStack gap="$1" alignItems="center">
                  <Select
                    value={customHour}
                    onValueChange={setCustomHour}
                    size="$1"
                  >
                    <Select.Trigger width={50} size="$2">
                      <Select.Value placeholder="HH" fontSize="$1" />
                    </Select.Trigger>
                    <Select.Content zIndex={200000}>
                      <Select.Viewport>
                        <Select.Group>
                          {hours.map((hour) => (
                            <Select.Item
                              key={hour}
                              value={String(hour)}
                              index={hour - 1}
                            >
                              <Select.ItemText fontSize="$1">
                                {hour}
                              </Select.ItemText>
                            </Select.Item>
                          ))}
                        </Select.Group>
                      </Select.Viewport>
                    </Select.Content>
                  </Select>

                  <Paragraph fontSize="$1">:</Paragraph>

                  <Select
                    value={customMinute}
                    onValueChange={setCustomMinute}
                    size="$1"
                  >
                    <Select.Trigger width={50} size="$2">
                      <Select.Value placeholder="MM" fontSize="$1" />
                    </Select.Trigger>
                    <Select.Content zIndex={200000}>
                      <Select.Viewport>
                        <Select.Group>
                          {minutes.map((minute) => (
                            <Select.Item
                              key={minute}
                              value={minute}
                              index={minutes.indexOf(minute)}
                            >
                              <Select.ItemText fontSize="$1">
                                {minute}
                              </Select.ItemText>
                            </Select.Item>
                          ))}
                        </Select.Group>
                      </Select.Viewport>
                    </Select.Content>
                  </Select>

                  <XStack gap="$0.5">
                    {["AM", "PM"].map((period) => {
                      const isSelected = customPeriod === period;
                      return (
                        <XStack
                          key={period}
                          alignItems="center"
                          gap="$0.5"
                          paddingHorizontal={6}
                          paddingVertical={2}
                          backgroundColor={isSelected ? "$blue10" : "$gray3"}
                          borderRadius="$1"
                          cursor="pointer"
                          hoverStyle={{
                            backgroundColor: isSelected ? "$blue11" : "$gray4",
                          }}
                          pressStyle={{ opacity: 0.8 }}
                          onPress={() => setCustomPeriod(period as "AM" | "PM")}
                        >
                          <Paragraph
                            fontSize="$1"
                            fontWeight={isSelected ? "600" : "400"}
                            color={isSelected ? "white" : "$gray11"}
                            cursor="pointer"
                          >
                            {period}
                          </Paragraph>
                        </XStack>
                      );
                    })}
                  </XStack>
                </XStack>

                {distributionPolicy.deleteAfter && (
                  <Paragraph fontSize="$1" color="$gray11" mt="$0.5">
                    Until:{" "}
                    {new Date(distributionPolicy.deleteAfter).toLocaleString()}
                  </Paragraph>
                )}
              </YStack>
            </YStack>
          </YStack>
        </View>

        {/* Right column: Content Rights */}
        <View
          f={1}
          minWidth={0}
          gap="$1.5"
          alignItems="center"
          justifyContent="flex-start"
          w={useTwoColumns ? 400 : "100%"}
          style={{
            marginTop: 2,
            ...(useTwoColumns ? {} : { marginLeft: 5 }),
          }}
        >
          <YStack gap="$1.5" w="100%">
            {/* Content Rights Section */}
            <YStack gap="$0.5" w="100%">
              <XStack alignItems="baseline" gap="$1">
                <Paragraph fontWeight="600" fontSize="$2.5">
                  Content Rights
                </Paragraph>
                <Paragraph color="$gray11" fontSize="$1">
                  (copyright & attribution)
                </Paragraph>
              </XStack>

              <YStack gap="$1.5">
                <YStack gap="$0.5">
                  <Label fontSize="$1.5" fontWeight="500">
                    License
                  </Label>
                  <Select
                    value={selectedLicense || undefined}
                    onValueChange={(value) => {
                      setSelectedLicense(value);
                      if (value !== "custom") {
                        handleContentRightsChange({ license: value });
                      }
                    }}
                  >
                    <Select.Trigger
                      width="100%"
                      iconAfter={ChevronDown}
                      size="$2"
                    >
                      <Select.Value placeholder="Select a license" />
                    </Select.Trigger>

                    <Adapt when="sm" platform="touch">
                      <Sheet
                        modal
                        dismissOnSnapToBottom
                        animationConfig={{
                          type: "spring",
                          damping: 20,
                          mass: 1.2,
                          stiffness: 250,
                        }}
                      >
                        <Sheet.Frame>
                          <Sheet.ScrollView>
                            <Adapt.Contents />
                          </Sheet.ScrollView>
                        </Sheet.Frame>
                        <Sheet.Overlay />
                      </Sheet>
                    </Adapt>

                    <Select.Content zIndex={200000}>
                      <Select.ScrollUpButton
                        alignItems="center"
                        justifyContent="center"
                        position="relative"
                        width="100%"
                        height="$3"
                      >
                        <YStack zIndex={10}>
                          <ChevronDown size={20} />
                        </YStack>
                      </Select.ScrollUpButton>

                      <Select.Viewport minWidth={200}>
                        <Select.Group>
                          <Select.Label>Choose License</Select.Label>
                          {LICENSE_OPTIONS.map((license) => (
                            <Select.Item
                              key={license.value}
                              index={LICENSE_OPTIONS.indexOf(license)}
                              value={license.value}
                            >
                              <Select.ItemText>{license.label}</Select.ItemText>
                              <Select.ItemIndicator marginLeft="auto">
                                <View
                                  width={5}
                                  height={5}
                                  borderRadius="$10"
                                  backgroundColor="$blue10"
                                />
                              </Select.ItemIndicator>
                            </Select.Item>
                          ))}
                        </Select.Group>
                      </Select.Viewport>

                      <Select.ScrollDownButton
                        alignItems="center"
                        justifyContent="center"
                        position="relative"
                        width="100%"
                        height="$3"
                      >
                        <YStack zIndex={10}>
                          <ChevronDown size={20} />
                        </YStack>
                      </Select.ScrollDownButton>
                    </Select.Content>
                  </Select>

                  {/* License Description */}
                  {(() => {
                    const selectedOption = LICENSE_OPTIONS.find(
                      (option) => option.value === selectedLicense,
                    );
                    return selectedOption?.description ? (
                      <XStack
                        gap="$1"
                        p="$1"
                        backgroundColor="$gray2"
                        borderRadius="$1"
                        alignItems="flex-start"
                      >
                        <Info size={12} color="$gray11" mt={2} />
                        <Paragraph fontSize="$1" color="$gray11" flex={1}>
                          {selectedOption.description}
                        </Paragraph>
                      </XStack>
                    ) : null;
                  })()}
                </YStack>

                {selectedLicense === "custom" && (
                  <YStack gap="$0.5">
                    <Label fontSize="$1" fontWeight="500">
                      Custom License
                    </Label>
                    <TextArea
                      placeholder="Enter custom license terms"
                      value={contentRights.license || ""}
                      onChangeText={(text) =>
                        handleContentRightsChange({
                          license: text,
                        })
                      }
                      size="$2"
                      minHeight={35}
                      fontSize="$1"
                      maxLength={200}
                      borderWidth={1}
                      borderColor="$gray8"
                      backgroundColor="$gray2"
                      focusStyle={{
                        borderColor: "$blue10",
                        backgroundColor: "$gray3",
                      }}
                    />
                  </YStack>
                )}

                <XStack gap="$1.5">
                  <YStack gap="$0.5" f={1}>
                    <Label fontSize="$1" fontWeight="500">
                      Year
                    </Label>
                    <Input
                      placeholder="2025"
                      value={contentRights.copyrightYear?.toString() || ""}
                      onChangeText={(text) => {
                        if (text === "") {
                          handleContentRightsChange({
                            copyrightYear: undefined,
                          });
                        } else {
                          const year = parseInt(text, 10);
                          if (!isNaN(year)) {
                            handleContentRightsChange({ copyrightYear: year });
                          }
                        }
                      }}
                      size="$2"
                      fontSize="$1"
                      maxLength={4}
                      keyboardType="numeric"
                      borderWidth={1}
                      borderColor="$gray8"
                      backgroundColor="$gray2"
                      focusStyle={{
                        borderColor: "$blue10",
                        backgroundColor: "$gray3",
                      }}
                    />
                  </YStack>

                  {/* TODO: Add creator - currently not supported by the backend     
                  <YStack gap="$0.5" f={2}>
                    <Label fontSize="$1" fontWeight="500">
                      Creator
                    </Label>
                    <Input
                      placeholder="Your Name / Handle"
                      value={contentRights.creator || ""}
                      onChangeText={(text) =>
                        handleContentRightsChange({ creator: text })
                      }
                      size="$2"
                      fontSize="$1"
                      maxLength={100}
                      borderWidth={1}
                      borderColor="$gray8"
                      backgroundColor="$gray2"
                      focusStyle={{
                        borderColor: "$blue10",
                        backgroundColor: "$gray3",
                      }}
                    />
                  </YStack>
                  */}
                </XStack>

                <YStack gap="$0.5">
                  <Label fontSize="$1" fontWeight="500">
                    Copyright Notice (Optional)
                  </Label>
                  <TextArea
                    placeholder="Additional copyright info"
                    value={contentRights.copyrightNotice || ""}
                    onChangeText={(text) =>
                      handleContentRightsChange({ copyrightNotice: text })
                    }
                    size="$2"
                    fontSize="$1"
                    minHeight={30}
                    maxLength={200}
                    borderWidth={1}
                    borderColor="$gray8"
                    backgroundColor="$gray2"
                    focusStyle={{
                      borderColor: "$blue10",
                      backgroundColor: "$gray3",
                    }}
                  />
                </YStack>

                <YStack gap="$0.5">
                  <Label fontSize="$1" fontWeight="500">
                    Credit Line (Optional)
                  </Label>
                  <TextArea
                    placeholder="How you'd like to be credited"
                    value={contentRights.creditLine || ""}
                    onChangeText={(text) =>
                      handleContentRightsChange({ creditLine: text })
                    }
                    size="$2"
                    fontSize="$1"
                    minHeight={30}
                    maxLength={200}
                    borderWidth={1}
                    borderColor="$gray8"
                    backgroundColor="$gray2"
                    focusStyle={{
                      borderColor: "$blue10",
                      backgroundColor: "$gray3",
                    }}
                  />
                </YStack>
              </YStack>
            </YStack>
          </YStack>
        </View>
      </View>

      {/* Save/Update Button */}
      {showUpdateButton && (
        <View p="$2" alignItems="center">
          <Button
            disabled={isLoading}
            opacity={isLoading ? 0.5 : 1}
            size="$4"
            w="100%"
            maxWidth={400}
            onPress={handleSaveMetadata}
          >
            {isLoading
              ? isCreating
                ? "Creating..."
                : "Updating..."
              : hasMetadata
                ? "Update Content Metadata"
                : "Create Content Metadata"}
          </Button>
          {error && (
            <Paragraph color="$red10" fontSize="$1" mt="$1">
              {error}
            </Paragraph>
          )}
        </View>
      )}

      {/* Footer Information */}
      <View p="$0.5" maxWidth={900} alignSelf="center">
        <XStack
          gap="$1"
          backgroundColor="$gray2"
          p="$1"
          borderRadius="$1"
          alignItems="center"
        >
          <Paragraph fontSize="$1" color="$gray11" flex={1}>
            Metadata is cryptographically signed for authenticity.
            <Paragraph
              color="$blue10"
              fontSize="$1"
              textDecorationLine="underline"
              onPress={() => {
                if (isWeb) {
                  window.open("https://contentcredentials.org", "_blank");
                }
              }}
            >
              {" "}
              Learn more
            </Paragraph>
          </Paragraph>
        </XStack>
      </View>
    </View>
  );
}
