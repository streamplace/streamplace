import { schemas } from "streamplace";

// Content warnings derived from lexicon schema
export const CONTENT_WARNINGS = (() => {
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
export const LICENSE_OPTIONS = (() => {
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

// License URL labels for C2PA manifests
export const LICENSE_URL_LABELS: Record<string, string> = {
  "http://creativecommons.org/publicdomain/zero/1.0/":
    "CC0 - Public Domain 1.0",
  "http://creativecommons.org/licenses/by/4.0/": "CC BY - Attribution 4.0",
  "http://creativecommons.org/licenses/by-sa/4.0/":
    "CC BY-SA - Attribution ShareAlike 4.0",
  "http://creativecommons.org/licenses/by-nc/4.0/":
    "CC BY-NC - Attribution NonCommercial 4.0",
  "http://creativecommons.org/licenses/by-nc-sa/4.0/":
    "CC BY-NC-SA - Attribution NonCommercial ShareAlike 4.0",
  "http://creativecommons.org/licenses/by-nd/4.0/":
    "CC BY-ND - Attribution NoDerivatives 4.0",
  "http://creativecommons.org/licenses/by-nc-nd/4.0/":
    "CC BY-NC-ND - Attribution NonCommercial NoDerivatives 4.0",
  "All rights reserved": "All Rights Reserved",
} as const;

// C2PA warning labels for content warnings
export const C2PA_WARNING_LABELS: Record<string, string> = {
  "cwarn:death": "Death",
  "cwarn:drugUse": "Drug Use",
  "cwarn:fantasyViolence": "Fantasy Violence",
  "cwarn:flashingLights": "Flashing Lights",
  "cwarn:language": "Language",
  "cwarn:nudity": "Nudity",
  "cwarn:PII": "Personally Identifiable Information",
  "cwarn:sexuality": "Sexuality",
  "cwarn:suffering": "Upsetting or Disturbing",
  "cwarn:violence": "Violence",
  // Also support lexicon constants for backward compatibility
  "place.stream.metadata.contentWarnings#death": "Death",
  "place.stream.metadata.contentWarnings#drugUse": "Drug Use",
  "place.stream.metadata.contentWarnings#fantasyViolence": "Fantasy Violence",
  "place.stream.metadata.contentWarnings#flashingLights": "Flashing Lights",
  "place.stream.metadata.contentWarnings#language": "Language",
  "place.stream.metadata.contentWarnings#nudity": "Nudity",
  "place.stream.metadata.contentWarnings#PII":
    "Personally Identifiable Information",
  "place.stream.metadata.contentWarnings#sexuality": "Sexuality",
  "place.stream.metadata.contentWarnings#suffering": "Upsetting or Disturbing",
  "place.stream.metadata.contentWarnings#violence": "Violence",
} as const;
