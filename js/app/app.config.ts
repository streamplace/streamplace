import {
  ConfigPlugin,
  withEntitlementsPlist,
  withXcodeProject,
} from "expo/config-plugins";
import streamplaceReactNativeWebRTC from "../config-react-native-webrtc";
export const withNotificationsIOS: ConfigPlugin = (config) => {
  config = withEntitlementsPlist(config, (config) => {
    config.modResults["aps-environment"] = "production";
    return config;
  });
  return config;
};

export const withoutNotificationsIOS: ConfigPlugin = (config) => {
  config = withEntitlementsPlist(config, (config) => {
    delete config.modResults["aps-environment"];
    return config;
  });
  return config;
};

const withConsistentVersionNumber = (
  config,
  { version }: { version: string },
) => {
  // if (!config.ios) {
  //   config.ios = {};
  // }
  // if (!config.ios.infoPlist) {
  //   config.ios.infoPlist = {};
  // }
  config = withXcodeProject(config, (config) => {
    for (let [k, v] of Object.entries(
      config.modResults.hash.project.objects.XCBuildConfiguration,
    )) {
      const obj = v as any;
      if (!obj.buildSettings) {
        continue;
      }
      if (typeof obj.buildSettings.MARKETING_VERSION !== "undefined") {
        obj.buildSettings.MARKETING_VERSION = version;
      }
      if (typeof obj.buildSettings.CURRENT_PROJECT_VERSION !== "undefined") {
        obj.buildSettings.CURRENT_PROJECT_VERSION = version;
      }
    }
    return config;
  });
  return config;
};

// turn a semver string into a always-increasing integer for google
export const versionCode = (verStr: string) => {
  const [major, minor, patch] = verStr.split(".").map((x) => parseInt(x));
  return major * 1000 * 1000 + minor * 1000 + patch;
};

export default function () {
  const isProd =
    process.env["SP_PRODUCTION_RELEASE"] === "true" || !!process.env.CI;
  const pkg = require("./package.json");
  const name = isProd ? "Streamplace" : "Devplace";
  const bundle = isProd ? "tv.aquareum" : "tv.aquareum.dev";
  const scheme = process.env["SP_APP_SCHEME"] ?? bundle;
  return {
    expo: {
      name: name,
      slug: name,
      version: pkg.version,
      // Only rev this to the current version when native dependencies change!
      runtimeVersion: pkg.runtimeVersion,
      orientation: "default",
      icon: "./assets/images/icon.png",
      scheme: scheme,
      userInterfaceStyle: "automatic",
      splash: {
        image: "./assets/images/splash.png",
        resizeMode: "contain",
        backgroundColor: "#ffffff",
      },
      assetBundlePatterns: ["**/*"],
      ios: {
        supportsTablet: true,
        bundleIdentifier: bundle,
        infoPlist: {
          UIBackgroundModes: ["fetch", "remote-notification"],
          LSMinimumSystemVersion: "12.0",
        },
        ...(isProd
          ? {
              googleServicesFile: "./GoogleService-Info.plist",
              entitlements: {
                "aps-environment": "production",
              },
            }
          : {}),
      },
      android: {
        adaptiveIcon: {
          foregroundImage: "./assets/images/adaptive-icon.png",
          backgroundColor: "#ffffff",
        },
        package: bundle,
        versionCode: versionCode(pkg.version),
        ...(isProd
          ? {
              googleServicesFile: "./google-services.json",
              permissions: [
                "android.permission.SCHEDULE_EXACT_ALARM",
                "android.permission.POST_NOTIFICATIONS",
              ],
            }
          : {}),
      },
      web: {
        bundler: "metro",
        output: "single",
        favicon: "./assets/images/favicon.png",
      },
      plugins: [
        streamplaceReactNativeWebRTC,
        ["expo-sqlite", { useSQLCipher: true }],
        "expo-file-system",
        [
          "expo-font",
          {
            fonts: [
              "assets/fonts/FiraCode-Bold.ttf",
              "assets/fonts/FiraCode-Light.ttf",
              "assets/fonts/FiraCode-Medium.ttf",
              "assets/fonts/FiraCode-Regular.ttf",
              "assets/fonts/FiraCode-Retina.ttf",
              "assets/fonts/FiraSans-Black.ttf",
              "assets/fonts/FiraSans-BlackItalic.ttf",
              "assets/fonts/FiraSans-Bold.ttf",
              "assets/fonts/FiraSans-BoldItalic.ttf",
              "assets/fonts/FiraSans-ExtraBold.ttf",
              "assets/fonts/FiraSans-ExtraBoldItalic.ttf",
              "assets/fonts/FiraSans-ExtraLight.ttf",
              "assets/fonts/FiraSans-ExtraLightItalic.ttf",
              "assets/fonts/FiraSans-Italic.ttf",
              "assets/fonts/FiraSans-Light.ttf",
              "assets/fonts/FiraSans-LightItalic.ttf",
              "assets/fonts/FiraSans-Medium.ttf",
              "assets/fonts/FiraSans-MediumItalic.ttf",
              "assets/fonts/FiraSans-Regular.ttf",
              "assets/fonts/FiraSans-SemiBold.ttf",
              "assets/fonts/FiraSans-SemiBoldItalic.ttf",
              "assets/fonts/FiraSans-Thin.ttf",
              "assets/fonts/FiraSans-ThinItalic.ttf",
              "assets/fonts/SpaceMono-Regular.ttf",
            ],
          },
        ],
        [
          "expo-build-properties",
          {
            ios: {
              useFrameworks: "static",
            },
            // uncomment to test OTA updates to http://localhost:8080
            // android: {
            //   usesCleartextTraffic: true,
            // },
          },
        ],
        [
          "expo-asset",
          {
            assets: ["assets"],
          },
        ],
        [withConsistentVersionNumber, { version: pkg.version }],
        ...(isProd
          ? [
              "@react-native-firebase/app",
              "@react-native-firebase/messaging",
              [withNotificationsIOS, {}],
            ]
          : ["expo-dev-launcher", withoutNotificationsIOS]),
      ],
      experiments: {
        typedRoutes: true,
      },
      updates: isProd
        ? {
            url: `https://stream.place/api/manifest`,
            enabled: true,
            checkAutomatically: "ON_LOAD",
            fallbackToCacheTimeout: 30000,
            codeSigningCertificate: "./code-signing/certs/certificate.pem",
            codeSigningMetadata: {
              keyid: "main",
              alg: "rsa-v1_5-sha256",
            },
          }
        : {},
    },
  };
}
