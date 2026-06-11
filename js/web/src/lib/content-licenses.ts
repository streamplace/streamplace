// Local copy of the license constants. Mirrors `LICENSE_OPTIONS` from
// @streamplace/components/lib/metadata-constants.

export interface LicenseOption {
  value: string;
  label: string;
  description: string;
}

export const LICENSE_OPTIONS: LicenseOption[] = [
  {
    value: "place.stream.metadata.contentRights#all-rights-reserved",
    label: "All Rights Reserved",
    description: "All rights reserved by the creator.",
  },
  {
    value: "place.stream.metadata.contentRights#cc0_1__0",
    label: "CC0 (Public Domain) 1.0",
    description: "No rights reserved. Anyone may use this for any purpose.",
  },
  {
    value: "place.stream.metadata.contentRights#cc-by_4__0",
    label: "CC BY 4.0",
    description:
      "Anyone may use, remix, and build upon this work, even commercially, as long as they credit the creator.",
  },
  {
    value: "place.stream.metadata.contentRights#cc-by-sa_4__0",
    label: "CC BY-SA 4.0",
    description:
      "Anyone may use, remix, and build upon this work, even commercially, as long as they credit the creator and license derivatives under the same terms.",
  },
  {
    value: "place.stream.metadata.contentRights#cc-by-nc_4__0",
    label: "CC BY-NC 4.0",
    description:
      "Anyone may use, remix, and build upon this work non-commercially, as long as they credit the creator.",
  },
  {
    value: "place.stream.metadata.contentRights#cc-by-nc-sa_4__0",
    label: "CC BY-NC-SA 4.0",
    description:
      "Anyone may use, remix, and build upon this work non-commercially, as long as they credit the creator and license derivatives under the same terms.",
  },
  {
    value: "place.stream.metadata.contentRights#cc-by-nd_4__0",
    label: "CC BY-ND 4.0",
    description:
      "Anyone may use this work, even commercially, as long as they credit the creator and don't modify it.",
  },
  {
    value: "place.stream.metadata.contentRights#cc-by-nc-nd_4__0",
    label: "CC BY-NC-ND 4.0",
    description:
      "Anyone may use this work non-commercially, as long as they credit the creator and don't modify it.",
  },
];

/** A magic value used in the UI to indicate the user wants to enter a custom license URL/text. */
export const CUSTOM_LICENSE = "__custom__";
