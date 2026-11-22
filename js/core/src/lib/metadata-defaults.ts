// Keep these defaults in step with pkg/constants/constants.go.
export const DEFAULT_CONTENT_METADATA = {
  contentWarnings: [] as readonly string[],
  contentRights: {},
  distributionPolicy: {
    allowEveryoneToDistribute: true,
    allowGenAiTraining: false,
    deleteAfter: undefined as number | undefined,
  },
} as const;
export const ALL_BROADCASTERS = "*";
export const INDEFINITE_ARCHIVAL = -1;

export function allowsEveryoneToDistribute(broadcasters?: string[]): boolean {
  return broadcasters?.length
    ? broadcasters.includes(ALL_BROADCASTERS)
    : DEFAULT_CONTENT_METADATA.distributionPolicy.allowEveryoneToDistribute;
}
