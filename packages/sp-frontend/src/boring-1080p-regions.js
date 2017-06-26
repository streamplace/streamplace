/**
 * Streamplace will eventually support tons of badass templates that do everything! This isn't
 * that. This is just dividing up a 1080p window into the people currently in a channel so we can
 * get started.
 */

export const arrangements = {
  "0": [],

  "1": [
    {
      x: 0,
      y: 0,
      width: 1920,
      height: 1080
    }
  ],

  "2": [
    {
      x: 0,
      y: 0,
      width: 1920 / 2,
      height: 1080
    },
    {
      x: 1920 / 2,
      y: 0,
      width: 1920 / 2,
      height: 1080
    }
  ],

  "3": [
    {
      x: 0,
      y: 0,
      width: 1920 / 3,
      height: 1080
    },
    {
      x: 1920 / 3,
      y: 0,
      width: 1920 / 3,
      height: 1080
    },
    {
      x: 1920 / 3 * 2,
      y: 0,
      width: 1920 / 3,
      height: 1080
    }
  ],

  "4": [
    {
      x: 0,
      y: 0,
      width: 1920 / 2,
      height: 1080 / 2
    },
    {
      x: 1920 / 2,
      y: 0,
      width: 1920 / 2,
      height: 1080 / 2
    },
    {
      x: 0,
      y: 1080 / 2,
      width: 1920 / 2,
      height: 1080 / 2
    },
    {
      x: 1920 / 2,
      y: 1080 / 2,
      width: 1920 / 2,
      height: 1080 / 2
    }
  ]
};

export function normalizeRegions(regions) {
  const sizes = arrangements[`${regions.length}`];
  regions.forEach((region, i) => {
    const size = sizes[i];
    Object.keys(size).forEach(key => {
      region[key] = size[key];
    });
  });
  return regions;
}
