import AQLink from "components/aqlink";
import ErrorBox from "components/error/error";
import Loading from "components/loading/loading";
import Viewers from "components/viewers";
import {
  pollSegments,
  Repo,
  selectRecentSegments,
} from "features/streamplace/streamplaceSlice";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useEffect, useState } from "react";
import { RefreshControl } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { H6, Image, ScrollView, ScrollViewProps, View } from "tamagui";

type Segment = {
  id: string;
  repoDID: string;
  signingKeyDID: string;
  startTime: string;
  repo: Repo;
};

export default function StreamList({
  contentContainerStyle = {},
}: {
  contentContainerStyle?: Exclude<
    ScrollViewProps["contentContainerStyle"],
    string
  >;
}) {
  const { url } = useStreamplaceNode();
  const { segments, error, loading, firstRequest } =
    useAppSelector(selectRecentSegments);
  const dispatch = useAppDispatch();
  const [manualRefresh, setManualRefresh] = useState(false);
  useEffect(() => {
    dispatch(pollSegments());
  }, []);
  useEffect(() => {
    if (!loading) {
      setManualRefresh(false);
    }
  }, [loading]);
  if (error) {
    if (loading) {
      return <Loading />;
    }
    return (
      <ErrorBox
        onRetry={() => {
          dispatch(pollSegments());
        }}
      />
    );
  }
  if (firstRequest) {
    return <Loading />;
  }
  return (
    <ScrollView
      contentContainerStyle={{
        alignItems: "stretch",
        minHeight: "100%",

        ...contentContainerStyle,
      }}
      refreshControl={
        <RefreshControl
          refreshing={manualRefresh}
          onRefresh={() => {
            dispatch(pollSegments());
            setManualRefresh(true);
          }}
        />
      }
    >
      {segments.map((segment, i) => {
        const user =
          segment.repo?.handle || segment.repoDID || segment.signingKeyDID;
        return (
          <View flex={1} key={i} alignItems="stretch">
            <AQLink
              to={{
                screen: "Stream",
                params: {
                  user: user,
                },
              }}
            >
              <View
                alignItems="center"
                display="flex"
                position="relative"
                maxWidth={400}
                flexBasis="100%"
                marginHorizontal="auto"
              >
                <Image
                  f={1}
                  aspectRatio={16 / 9}
                  width="100%"
                  src={`${url}/api/playback/${user}/stream.png`}
                  resizeMode="contain"
                  objectFit="contain"
                />
                <View
                  position="absolute"
                  top={0}
                  right={0}
                  flexDirection="row"
                  justifyContent="center"
                  alignItems="center"
                  overflow="visible"
                >
                  <View
                    position="relative"
                    backgroundColor="rgb(0,0,0,0.8)"
                    borderRadius="$10"
                    marginRight="$2"
                    marginTop="$2"
                  >
                    <Viewers viewers={segment.viewers} />
                  </View>
                </View>
                <H6>
                  {segment.repo?.handle ? `@${segment.repo.handle}` : user}
                </H6>
              </View>
            </AQLink>
          </View>
        );
      })}
      <View f={1} justifyContent="center" alignItems="center">
        {segments.length === 0 && (
          <>
            <H6>No one is streaming right now 😭</H6>
          </>
        )}
      </View>
    </ScrollView>
  );
}
