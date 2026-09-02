import { getStreamplaceUrl } from "./streamplace-url";

export function getApiBase(): string {
  return `${getStreamplaceUrl()}/api`;
}
