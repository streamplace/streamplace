import {
  Button,
  storage,
  Text,
  tokens,
  useTheme,
  zero,
} from "@streamplace/components";
import { ChevronDown } from "lucide-react-native";
import { useEffect, useRef, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import {
  Linking,
  Modal,
  Platform,
  ScrollView,
  useWindowDimensions,
  View,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";

const STREAMER_AGREEMENT_KEY = "streamer_agreement_accepted_2";

interface StreamerAgreementProps {
  onAccepted: () => void;
  onDeclined?: () => void;
  nodeUrl?: URL;
}

export default function StreamerAgreement({
  onAccepted,
  onDeclined,
  nodeUrl,
}: StreamerAgreementProps) {
  const dims = useWindowDimensions();
  const [visible, setVisible] = useState(false);
  const [loading, setLoading] = useState(true);
  const [scrolledToBottom, setScrolledToBottom] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const bottomMarkerRef = useRef<View>(null);
  const scrollViewRef = useRef<ScrollView>(null);
  const { theme, zero: z } = useTheme();
  const { t } = useTranslation("common");

  useEffect(() => {
    checkAgreement();
  }, []);

  const checkAgreement = async () => {
    try {
      const accepted = await storage.getItem(
        STREAMER_AGREEMENT_KEY + "-" + nodeUrl?.hostname,
      );
      if (!accepted) {
        setVisible(true);
      } else {
        onAccepted();
      }
    } catch (error) {
      console.error("Failed to check streamer agreement:", error);
      setVisible(true);
    } finally {
      setLoading(false);
    }
  };

  const handleAccept = async () => {
    if (!scrolledToBottom && !confirming) {
      setConfirming(true);
      return;
    }

    try {
      await storage.setItem(
        STREAMER_AGREEMENT_KEY + "-" + nodeUrl?.hostname,
        "true",
      );
      setVisible(false);
      onAccepted();
    } catch (error) {
      console.error("Failed to save streamer agreement:", error);
    }
  };

  const handleDecline = () => {
    setVisible(false);
    onDeclined?.();
  };

  if (loading) {
    return null;
  }

  if (!visible) {
    return null;
  }

  return (
    <Modal
      visible={visible}
      transparent={true}
      animationType="fade"
      onRequestClose={() => {}}
      style={{ cursor: "pointer" }}
    >
      <View
        style={[
          zero.layout.flex[1],
          zero.layout.flex.center,
          zero.layout.flex.alignCenter,
          zero.layout.flex.justifyCenter,
          {
            backgroundColor: "rgba(0, 0, 0, 0.9)",
            position: "absolute",
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            width: "100%",
            height: "100%",
          },
        ]}
      >
        <SafeAreaView
          style={[
            z.bg.card,
            zero.r.xl,
            zero.p[6],
            Platform.OS == "web" && {
              width: 600,
              maxWidth: "95%",
              maxHeight: "85%",
              cursor: "auto",
            },
          ]}
        >
          <View style={[zero.layout.flex[1]]}>
            <Text size="2xl" leading="snug" style={[zero.mb[4]]}>
              {t("streamer-agreement-title")}
            </Text>

            <ScrollView
              ref={scrollViewRef}
              onScroll={(e) => {
                const { layoutMeasurement, contentOffset, contentSize } =
                  e.nativeEvent;
                const isScrolledToBottom =
                  layoutMeasurement.height + contentOffset.y >=
                  contentSize.height - 10;
                if (isScrolledToBottom) {
                  setScrolledToBottom(true);
                }
              }}
              onContentSizeChange={(width, height) => {
                if (scrollViewRef.current) {
                  (scrollViewRef.current as any).measure(
                    (x: number, y: number, w: number, h: number) => {
                      if (h >= height - 10) {
                        setScrolledToBottom(true);
                      }
                    },
                  );
                }
              }}
              scrollEventThrottle={100}
              style={[zero.flex[1], zero.mb[2], { maxHeight: "60vh" }]}
              showsHorizontalScrollIndicator={true}
            >
              <View style={{ maxWidth: dims.width * 0.8, width: "100%" }}>
                <Text size="base" leading="relaxed" style={[zero.mb[3]]}>
                  {t("streamer-agreement-intro")}
                </Text>

                <View style={[zero.mb[4], zero.gap.all[1]]}>
                  <View style={[zero.layout.flex.row]}>
                    <Text size="base" leading="relaxed" style={[zero.mr[2]]}>
                      1.
                    </Text>
                    <Text
                      size="base"
                      leading="relaxed"
                      style={[zero.layout.flex[1]]}
                    >
                      {t("streamer-agreement-rule-1")}
                    </Text>
                  </View>
                  <View style={[zero.layout.flex.row]}>
                    <Text size="base" leading="relaxed" style={[zero.mr[2]]}>
                      2.
                    </Text>
                    <Text
                      size="base"
                      leading="relaxed"
                      style={[zero.layout.flex[1]]}
                    >
                      {t("streamer-agreement-rule-2")}
                    </Text>
                  </View>
                  <View style={[zero.layout.flex.row]}>
                    <Text size="base" leading="relaxed" style={[zero.mr[2]]}>
                      3.
                    </Text>
                    <Text
                      size="base"
                      leading="relaxed"
                      style={[zero.layout.flex[1]]}
                    >
                      {t("streamer-agreement-rule-3")}
                    </Text>
                  </View>
                  <View style={[zero.layout.flex.row]}>
                    <Text size="base" leading="relaxed" style={[zero.mr[2]]}>
                      4.
                    </Text>
                    <Text
                      size="base"
                      leading="relaxed"
                      style={[zero.layout.flex[1]]}
                    >
                      <Trans
                        i18nKey="streamer-agreement-rule-4"
                        default="Not stream content that is illegal, harmful, or violates our Terms of Service. <1>This may include graphic and certain sexual content.</1>"
                        components={{
                          1: (
                            <Text
                              size="base"
                              style={[
                                {
                                  fontWeight: 600,
                                  backgroundColor: "#ffea0066",
                                  marginHorizontal: -4,
                                  paddingHorizontal: 4,
                                },
                              ]}
                            />
                          ),
                        }}
                      />
                    </Text>
                  </View>
                  <View style={[zero.layout.flex.row]}>
                    <Text size="base" leading="relaxed" style={[zero.mr[2]]}>
                      5.
                    </Text>
                    <Text
                      size="base"
                      leading="relaxed"
                      style={[zero.layout.flex[1]]}
                    >
                      <Trans
                        i18nKey="streamer-agreement-rule-5"
                        default="Not violate our policies. Doing so may result in the <1>removal of features available to you (including your ability to stream), account suspension, and in some cases, account termination.</1>"
                        components={{
                          1: (
                            <Text
                              size="base"
                              style={[
                                {
                                  fontWeight: 600,
                                  backgroundColor: "#ffea0066",
                                  marginHorizontal: -4,
                                  paddingHorizontal: 4,
                                },
                              ]}
                            />
                          ),
                        }}
                      />
                    </Text>
                  </View>
                </View>

                <Text size="base" leading="relaxed" style={[zero.mb[3]]}>
                  <Trans
                    i18nKey="streamer-agreement-footer"
                    default="For full details, please review our <1>Terms of Service</1> and <2>Community Guidelines</2>."
                    components={{
                      1: <Text size="base" weight="semibold" />,
                      2: (
                        <Text
                          size="base"
                          weight="semibold"
                          style={{ color: tokens.colors.blue[400] }}
                          onPress={() =>
                            Linking.openURL(
                              "https://blog.stream.place/3mcqwibo4ks2w",
                            )
                          }
                        />
                      ),
                    }}
                  />
                </Text>

                <View ref={bottomMarkerRef} style={{ height: 1, width: 1 }} />
              </View>
            </ScrollView>

            {!scrolledToBottom && (
              <View
                style={[
                  {
                    position: "absolute",
                    bottom: 150,
                    left: 0,
                    right: 0,
                    alignItems: "center",
                    pointerEvents: "none",
                  },
                ]}
              >
                <View
                  style={[
                    z.bg.muted,
                    zero.r.full,
                    zero.px[2],
                    zero.py[2],
                    zero.borders.width.thin,
                    {
                      borderColor: theme.colors.mutedForeground,
                    },
                  ]}
                >
                  <ChevronDown color={theme.colors.mutedForeground} />
                </View>
              </View>
            )}

            <Text
              size="xs"
              leading="relaxed"
              style={[z.text.mutedForeground, zero.mb[4]]}
            >
              {t("streamer-agreement-disclaimer")}
            </Text>

            <View
              style={[
                zero.layout.flex.row,
                zero.layout.flex.justify.end,
                zero.gap.all[2],
              ]}
            >
              <Button
                variant="outline"
                size="lg"
                width={"38%" as any}
                style={[zero.px[0]]}
                onPress={handleDecline}
              >
                {t("streamer-agreement-decline", "Go Back")}
              </Button>
              <Button
                variant="primary"
                size="lg"
                width={"62%" as any}
                style={[zero.px[0]]}
                onPress={handleAccept}
                disabled={!scrolledToBottom && !confirming}
              >
                {confirming
                  ? t("are-you-sure")
                  : t("streamer-agreement-accept")}
              </Button>
            </View>
          </View>
        </SafeAreaView>
      </View>
    </Modal>
  );
}
