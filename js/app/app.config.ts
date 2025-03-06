import {
  ConfigPlugin,
  IOSConfig,
  withAppDelegate,
  withEntitlementsPlist,
  withMainApplication,
  withXcodeProject,
} from "expo/config-plugins";
import path from "path";
export const withNotificationsIOS: ConfigPlugin = (config) => {
  config = withEntitlementsPlist(config, (config) => {
    config.modResults["aps-environment"] = "production";
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

// https://github.com/react-native-webrtc/react-native-webrtc/blob/19ca31d4b77d149a659ee037fae54861a2d90a73/Documentation/AndroidInstallation.md#set-audio-category-output-to-media
// look, i'm as upset about this as you are
const androidApplicationReplacements = [
  {
    from: "class MainApplication : Application(), ReactApplication {",
    to: `
import com.oney.WebRTCModule.WebRTCModuleOptions
import android.media.AudioAttributes
import org.webrtc.audio.JavaAudioDeviceModule

class MainApplication : Application(), ReactApplication {`,
  },
  {
    from: "override fun onCreate() {",
    to: `
  override fun onCreate() {
    // append this before WebRTCModule initializes
    val options = WebRTCModuleOptions.getInstance()
    val audioAttributes = AudioAttributes.Builder()
      .setUsage(AudioAttributes.USAGE_MEDIA)
      .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
      .build()
    options.audioDeviceModule = JavaAudioDeviceModule.builder(this)
      .setAudioAttributes(audioAttributes)
      .createAudioDeviceModule()
`,
  },
];

export const withWorkingAndroidWebRTCAudio: ConfigPlugin = (configOuter) => {
  return withMainApplication(configOuter, (config) => {
    let stringContents: string = config.modResults.contents;

    for (const { from, to } of androidApplicationReplacements) {
      stringContents = stringContents.replace(from, to);
    }

    config.modResults.contents = stringContents;

    return config;
  });
};

const iosDelegateReplacements = [
  {
    from: "#import <React/RCTLinkingManager.h>",
    to: (config) => `
#import <React/RCTLinkingManager.h>
#import <WebRTC/WebRTC.h>
#import <react-native-webrtc-umbrella.h>
#import "ExpoModulesCore-Swift.h"
#import "${config.name}-Swift.h"
`,
  },
  {
    from: "  self.initialProps = @{};",
    to: () => `
  self.initialProps = @{};
  ////RTC PATCH////
  RTCAudioSessionConfiguration* config = [RTCAudioSessionConfiguration webRTCConfiguration];

  AVAudioSession * session = [AVAudioSession sharedInstance];
  // Set audio to use phone speaker instead of headset speaker
  [session setCategory:AVAudioSessionCategoryPlayAndRecord
           withOptions:AVAudioSessionCategoryOptionDefaultToSpeaker | AVAudioSessionCategoryOptionAllowBluetooth
                 error:nil];
  [session setActive:YES error:nil];

  id<RTCAudioDevice> device;
  device = [[AUAudioUnitRTCAudioDevice alloc] init];

  WebRTCModuleOptions *options = [WebRTCModuleOptions sharedInstance];
  options.loggingSeverity = RTCLoggingSeverityWarning;
  options.audioDevice = device;
  ////END RTC PATCH////
    `,
  },
];

const withWorkingIOSWebRTCAudio: ConfigPlugin = (config) => {
  const files = [
    "AUAudioUnitRTCAudioDevice.swift",
    "AudioSessionHandler.swift",
    "SimpleAudioConverter.swift",
    "Utils.swift",
  ];

  // modify the app delegate to make use of the CustomRTCAudioDevice
  config = withAppDelegate(config, (config) => {
    let stringContents: string = config.modResults.contents;

    for (const { from, to } of iosDelegateReplacements) {
      stringContents = stringContents.replace(from, to(config));
    }

    config.modResults.contents = stringContents;

    return config;
  });

  // add the CustomRTCAudioDevice files to the xcode project
  config = withXcodeProject(config, (config) => {
    const rtc = require.resolve("rtcaudiodevice");
    for (const file of files) {
      IOSConfig.XcodeUtils.addBuildSourceFileToGroup({
        filepath: path.resolve(rtc, "..", "CustomRTCAudioDevice", file),
        groupName: config.name,
        project: config.modResults,
      });
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
    process.env["AQ_PRODUCTION_RELEASE"] === "true" || !!process.env.CI;
  const pkg = require("./package.json");
  const name = isProd ? "Streamplace" : "Devplace";
  const bundle = isProd ? "tv.aquareum" : "tv.aquareum.dev";
  const scheme = process.env["AQ_APP_SCHEME"] ?? bundle;
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
        withWorkingIOSWebRTCAudio,
        withWorkingAndroidWebRTCAudio,
        "@config-plugins/react-native-webrtc",
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
          : ["expo-dev-launcher"]),
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
