import { describe, expect, it } from "vitest";
import { PDS_HOSTS } from "./pds-hosts";

describe("PDS host chooser copy", () => {
  it("describes hosts without unexplained protocol jargon", () => {
    for (const host of PDS_HOSTS) {
      expect(`${host.label} ${host.description}`).not.toMatch(/\bPDS\b/);
    }
  });
});
