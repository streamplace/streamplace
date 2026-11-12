// Keep these defaults in step with pkg/constants/constants.go.
export const DEFAULT_ALLOW_EVERYONE_TO_DISTRIBUTE = true;
export const DEFAULT_ALLOW_GEN_AI_TRAINING = false;
export const DEFAULT_DELETE_AFTER: number | undefined = undefined;
export const ALL_BROADCASTERS = "*";
export const INDEFINITE_ARCHIVAL = -1;

export function allowsEveryoneToDistribute(broadcasters?: string[]): boolean {
  return broadcasters?.length
    ? broadcasters.includes(ALL_BROADCASTERS)
    : DEFAULT_ALLOW_EVERYONE_TO_DISTRIBUTE;
}
