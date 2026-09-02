import { useState } from "react";
import { sonare } from "sonare";

function generateSonare(length: number, suffixLength: number) {
  let words = Array.from({ length }, () =>
    sonare({ minLength: 4, maxLength: 8 }),
  );
  let word = words.join("-");

  if (suffixLength) {
    const suffix = Math.random()
      .toString(36)
      .slice(2, 2 + suffixLength);
    word += `-${suffix}`;
  }

  return word;
}

// Sonare-based random ID generator. It generates a string of N words
// separated by dashes, optionally with a random suffix of specified length.
// Note that THIS IS NOT A CRYPTOGRAPHICALLY SECURE ID GENERATOR.
export function useSonare(
  { length, suffixLength } = { length: 2, suffixLength: 4 },
) {
  const [sona, setSona] = useState(() => {
    return generateSonare(length, suffixLength);
  });

  const regenerate = () => {
    setSona(generateSonare(length, suffixLength));
  };

  return [sona, regenerate] as const;
}
