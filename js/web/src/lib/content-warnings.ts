// Content-warnings schema constants for the web app.

export interface ContentWarning {
  value: string;
  label: string;
  description: string;
}

export const CONTENT_WARNINGS: ContentWarning[] = [
  {
    value: "place.stream.metadata.contentWarnings#death",
    label: "Death",
    description: "Depicts or discusses death.",
  },
  {
    value: "place.stream.metadata.contentWarnings#drugUse",
    label: "Drug Use",
    description: "Depicts or discusses drug use.",
  },
  {
    value: "place.stream.metadata.contentWarnings#fantasyViolence",
    label: "Fantasy Violence",
    description: "Depicts fantasy violence.",
  },
  {
    value: "place.stream.metadata.contentWarnings#flashingLights",
    label: "Flashing Lights",
    description: "Contains flashing lights that may trigger seizures.",
  },
  {
    value: "place.stream.metadata.contentWarnings#language",
    label: "Strong Language",
    description: "Contains strong or pervasive language.",
  },
  {
    value: "place.stream.metadata.contentWarnings#nudity",
    label: "Nudity",
    description:
      "Discusses nudity. Depictions of nudity are never allowed, except, in a video game, as required to progress, with this content warning.",
  },
  {
    value: "place.stream.metadata.contentWarnings#PII",
    label: "Personally Identifiable Information",
    description: "May disclose personally identifiable information.",
  },
  {
    value: "place.stream.metadata.contentWarnings#sexuality",
    label: "Sexuality",
    description: "Discusses sexual content.",
  },
  {
    value: "place.stream.metadata.contentWarnings#suffering",
    label: "Upsetting or Disturbing",
    description: "Discusses upsetting or disturbing content.",
  },
  {
    value: "place.stream.metadata.contentWarnings#violence",
    label: "Violence",
    description: "Discusses violence.",
  },
];
