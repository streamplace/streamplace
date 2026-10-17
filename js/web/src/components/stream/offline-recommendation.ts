export type OfflineRecommendation = {
  did: string;
  source: string;
};

export type ResolvedOfflineRecommendation = OfflineRecommendation & {
  handle: string;
};

export function validateOfflineRecommendation(
  recommendation: OfflineRecommendation | null,
  profile: { did: string; handle: string } | null | undefined,
  offlineDid: string | undefined,
): ResolvedOfflineRecommendation | null {
  if (!recommendation || !profile?.handle) return null;
  if (recommendation.did !== profile.did) return null;
  if (recommendation.did === offlineDid) return null;
  return { ...recommendation, handle: profile.handle };
}
