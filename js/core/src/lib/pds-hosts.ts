// PDS (Personal Data Server) host list shown during signup. The
// streamplace user needs a PDS before they can sign in — this is the
// curated list we present to new users, plus an option to enter a
// custom host.
//
// `handlePolicyDocs` is a special case: hosts that have a handle
// policy (e.g. Blacksky's "you must follow this account") require the
// user to check a box before continuing. We pre-shuffle the list once
// at module load so the order feels fresh on each visit, but always
// pin a non-policy host to the top so the first option is always
// ready to go without an extra checkbox.
export interface PdsHost {
  value: string;
  label: string;
  description: string;
  terms: string;
  privacy: string;
  handlePolicyDocs?: string;
}

export const PDS_HOSTS: PdsHost[] = [
  {
    value: "https://selfhosted.social",
    label: "selfhosted.social",
    description: "A popular community-run PDS",
    terms: "https://selfhosted.social/legal#terms",
    privacy: "https://selfhosted.social/legal",
  },
  {
    // Redirects to https://bsky.social for sign in.
    value: "https://witchesbutter.us-west.host.bsky.network",
    label: "Bluesky",
    description: "The main Bluesky PDS instance",
    terms: "https://bsky.social/about/support/tos",
    privacy: "https://bsky.social/about/support/privacy-policy",
  },
  {
    value: "https://blacksky.app",
    label: "Blacksky PDS",
    description: "A PDS service by Blacksky Algorithms",
    terms: "https://blackskyweb.xyz/about/support/tos",
    privacy: "https://blackskyweb.xyz/about/support/privacy-policy/",
    handlePolicyDocs:
      "https://docs.blacksky.community/migrating-to-blacksky-pds-complete-guide#who-can-use-blacksky-services",
  },
  {
    value: "https://pds.tophhie.cloud",
    label: "Tophhie Cloud",
    description: "A PDS service by Tophhie",
    terms: "https://blog.tophhie.cloud/atproto-tos/",
    privacy: "https://blog.tophhie.cloud/atproto-privacy-policy/",
  },
];

const shuffleArray = <T>(array: T[]): T[] => {
  const arr = [...array];
  for (let i = arr.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [arr[i], arr[j]] = [arr[j], arr[i]];
  }
  return arr;
};

// First item is always a non-policy host; the rest are shuffled
// (policies and non-policies mixed). Evaluated once at module load.
export const SHUFFLED_PDS_HOSTS: PdsHost[] = (() => {
  const withPolicies = PDS_HOSTS.filter((h) => h.handlePolicyDocs);
  const [first, ...withoutPolicies] = PDS_HOSTS.filter(
    (h) => !h.handlePolicyDocs,
  );
  return [first, ...shuffleArray(withPolicies.concat(withoutPolicies))];
})();
