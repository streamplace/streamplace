export const delay = (ms: number) => new Promise((r) => setTimeout(r, ms));

export type PlayerReport = {
  whatHappened: { [k: string]: number };
  avSync?: {
    min: number;
    max: number;
    avg: number;
  };
};
