import RNWebRTCPlugin from "@config-plugins/react-native-webrtc";
import type { ConfigPlugin } from "expo/config-plugins";
import {
  IOSConfig,
  withAppDelegate,
  withEntitlementsPlist,
  withMainApplication,
  withXcodeProject,
} from "expo/config-plugins";
import { copyFileSync, existsSync, mkdirSync } from "fs";
import { resolve } from "path";

interface BroadcastExtensionOptions {
  appGroupIdentifier?: string;
  extensionName?: string;
  bundleIdentifier?: string;
}

interface WebRTCConfigOptions {
  broadcastExtension?: BroadcastExtensionOptions;
}

const buildError = (message: string) => {
  if (process.env.SP_SKIP_CODEMODE_ERRORS !== "true") {
    throw new Error(`@streamplace/config-native-webrtc ${message}`);
  } else {
    console.error(
      `@streamplace/config-native-webrtc ${message}, skipping because SP_SKIP_CODEMODE_ERRORS=true`,
    );
  }
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
      .setUseStereoInput(true)
      .setUseStereoOutput(true)
      .createAudioDeviceModule()
    // enable screenshare
    options.enableMediaProjectionService = true;
`,
  },
];

export const withWorkingAndroidWebRTCAudio: ConfigPlugin = (configOuter) => {
  return withMainApplication(configOuter, (config) => {
    let stringContents: string = config.modResults.contents;

    for (const { from, to } of androidApplicationReplacements) {
      stringContents = stringContents.replace(from, to);
    }
    if (stringContents === config.modResults.contents) {
      buildError("android codemod failed to apply");
    }

    config.modResults.contents = stringContents;

    return config;
  });
};

const iosDelegateReplacements = [
  // Objective-C Version
  {
    from: "#import <React/RCTLinkingManager.h>",
    to: (config) => `
#import <React/RCTLinkingManager.h>
#import <WebRTC/WebRTC.h>
#import "CaptureController.h"
#import "CapturerEventsDelegate.h"
#import "DataChannelWrapper.h"
#import "RCTConvert+WebRTC.h"
#import "RTCMediaStreamTrack+React.h"
#import "RTCVideoViewManager.h"
#import "ScreenCaptureController.h"
#import "ScreenCapturePickerViewManager.h"
#import "ScreenCapturer.h"
#import "SerializeUtils.h"
#import "SocketConnection.h"
#import "TrackCapturerEventsEmitter.h"
#import "VideoCaptureController.h"
#import "WebRTCModule+RTCDataChannel.h"
#import "WebRTCModule+RTCMediaStream.h"
#import "WebRTCModule+RTCPeerConnection.h"
#import "WebRTCModule+VideoTrackAdapter.h"
#import "WebRTCModule.h"
#import "WebRTCModuleOptions.h"
#import "ExpoModulesCore-Swift.h"
#import "${config.name.replaceAll(" ", "")}-Swift.h"
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
  // Enable stereo audio
  options.enableStereoOutput = YES;
  ////END RTC PATCH////
    `,
  },
  // Swift Version
  {
    from: "    let delegate = ReactNativeDelegate()",
    to: () => `
    // WebRTC Configuration
    let config = RTCAudioSessionConfiguration.webRTC()

    let session = AVAudioSession.sharedInstance()
    do {
        try session.setCategory(.playAndRecord,
                              options: [.defaultToSpeaker, .allowBluetooth])
        try session.setActive(true)
    } catch {
        print("Failed to configure audio session: \(error)")
    }

    let device = AUAudioUnitRTCAudioDevice()

    let options = WebRTCModuleOptions.sharedInstance()
    options.loggingSeverity = .warning
    options.audioDevice = device
    // End WebRTC Configuration

    let delegate = ReactNativeDelegate()
    `,
  },
  {
    from: "import ReactAppDependencyProvider",
    to: () => `
import ReactAppDependencyProvider
import WebRTC
import react_native_webrtc
import AVFoundation
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

  let called = false;
  // modify the app delegate to make use of the CustomRTCAudioDevice
  config = withAppDelegate(config, (config) => {
    let stringContents: string = config.modResults.contents;

    for (const { from, to } of iosDelegateReplacements) {
      stringContents = stringContents.replace(from, to(config));
    }
    if (stringContents === config.modResults.contents) {
      buildError("ios codemod failed to change anything, aborting");
    }

    config.modResults.contents = stringContents;
    called = true;

    return config;
  });

  // add the CustomRTCAudioDevice files to the xcode project
  config = withXcodeProject(config, (config) => {
    const rtc = require.resolve("rtcaudiodevice");
    for (const file of files) {
      IOSConfig.XcodeUtils.addBuildSourceFileToGroup({
        filepath: resolve(rtc, "..", "CustomRTCAudioDevice", file),
        groupName: config.name,
        project: config.modResults,
      });
    }

    return config;
  });

  return config;
};

const withBroadcastExtension: ConfigPlugin<BroadcastExtensionOptions> = (
  config,
  {
    appGroupIdentifier = "group.com.jitsi.example-screensharing.appgroup",
    extensionName = "BroadcastExtension",
    bundleIdentifier,
  } = {},
) => {
  const targetName = extensionName;
  const bundleIdSuffix =
    extensionName.charAt(0).toLowerCase() + extensionName.slice(1);
  // Add app group entitlement to main app
  config = withEntitlementsPlist(config, (config) => {
    if (!config.modResults["com.apple.security.application-groups"]) {
      config.modResults["com.apple.security.application-groups"] = [];
    }
    const appGroups = config.modResults[
      "com.apple.security.application-groups"
    ] as string[];
    if (!appGroups.includes(appGroupIdentifier)) {
      appGroups.push(appGroupIdentifier);
    }
    return config;
  });

  // Configure Xcode project to include broadcast extension
  config = withXcodeProject(config, (config) => {
    const xcodeProject = config.modResults;
    const appName = config.name || "App";
    // Read bundle identifier from multiple sources with fallback
    let mainAppBundleId = config.ios?.bundleIdentifier || "com.example.app";
    console.log(
      `BroadcastExtension: config.ios?.bundleIdentifier = ${config.ios?.bundleIdentifier}`,
    );
    console.log(
      `BroadcastExtension: Initial mainAppBundleId = ${mainAppBundleId}`,
    );

    const mainTarget = xcodeProject.getFirstTarget();
    console.log(
      `BroadcastExtension: Found main target: ${mainTarget?.name || "undefined"}`,
    );

    // Try to get from Xcode project build settings first
    if (mainTarget && mainTarget.buildConfigurationList) {
      console.log(
        `BroadcastExtension: Main target has buildConfigurationList: ${mainTarget.buildConfigurationList}`,
      );
      const configList =
        xcodeProject.pbxXCConfigurationList[mainTarget.buildConfigurationList];
      console.log(
        `BroadcastExtension: Found config list with ${configList?.buildConfigurations?.length || 0} configurations`,
      );

      if (
        configList &&
        configList.buildConfigurations &&
        configList.buildConfigurations.length > 0
      ) {
        const buildConfig =
          xcodeProject.pbxXCBuildConfigurationSection()[
            configList.buildConfigurations[0].value
          ];
        console.log(
          `BroadcastExtension: Build config name: ${buildConfig?.name}`,
        );
        console.log(
          `BroadcastExtension: PRODUCT_BUNDLE_IDENTIFIER in build settings: ${buildConfig?.buildSettings?.PRODUCT_BUNDLE_IDENTIFIER}`,
        );

        if (
          buildConfig &&
          buildConfig.buildSettings &&
          buildConfig.buildSettings.PRODUCT_BUNDLE_IDENTIFIER
        ) {
          const originalId =
            buildConfig.buildSettings.PRODUCT_BUNDLE_IDENTIFIER;
          mainAppBundleId = originalId.replace(/"/g, "");
          console.log(
            `BroadcastExtension: Updated mainAppBundleId from "${originalId}" to "${mainAppBundleId}"`,
          );
        }
      }
    } else {
      console.log(
        `BroadcastExtension: No main target or buildConfigurationList found`,
      );
    }

    console.log(`BroadcastExtension: Main app bundle ID: ${mainAppBundleId}`);
    const extensionBundleId =
      bundleIdentifier || `${mainAppBundleId}.${bundleIdSuffix}`;
    console.log(
      `BroadcastExtension: Extension bundle ID: ${extensionBundleId}`,
    );

    try {
      // Files are copied to dist/BroadcastExtension during build, so __dirname points to dist
      const packageRoot = __dirname;

      // Determine iOS project directory
      const iosDir = resolve(config.modRequest.projectRoot, "ios");
      const extensionDir = resolve(iosDir, targetName);

      // Create BroadcastExtension directory in iOS project
      if (!existsSync(extensionDir)) {
        mkdirSync(extensionDir, { recursive: true });
      }

      // Add broadcast extension target
      const target = xcodeProject.addTarget(
        targetName,
        "app_extension",
        targetName,
        extensionBundleId,
      );

      // Create extension group in project
      const extensionGroup = xcodeProject.addPbxGroup(
        [],
        targetName,
        targetName,
        `"<group>"`,
      );

      // Add extension group to main group
      const mainGroupKey = xcodeProject.findPBXGroupKey({ name: appName });
      if (mainGroupKey) {
        xcodeProject.addToPbxGroup(extensionGroup.uuid, mainGroupKey);
      }

      // Add source files to the extension target
      const sourceFiles = [
        "SampleHandler.swift",
        "Atomic.swift",
        "DarwinNotificationCenter.swift",
        "SampleUploader.swift",
        "SocketConnection.swift",
      ];

      // Copy all source files
      sourceFiles.forEach((fileName) => {
        const sourceFilePath = resolve(
          packageRoot,
          "BroadcastExtension",
          fileName,
        );
        const destFilePath = resolve(extensionDir, fileName);

        // Check if source file exists before copying
        if (!existsSync(sourceFilePath)) {
          throw new Error(
            `BroadcastExtension source file not found: ${sourceFilePath}`,
          );
        }

        // Copy file to iOS project directory
        copyFileSync(sourceFilePath, destFilePath);

        // Add file to Xcode project
        xcodeProject.addFile(destFilePath, extensionGroup.uuid, {
          target: target.uuid,
        });
      });

      // Copy and add Info.plist for extension
      const sourcePlistPath = resolve(
        packageRoot,
        "BroadcastExtension",
        "Info.plist",
      );
      const destPlistPath = resolve(extensionDir, `${targetName}-Info.plist`);

      // Check if Info.plist exists before copying
      if (!existsSync(sourcePlistPath)) {
        throw new Error(
          `BroadcastExtension Info.plist not found: ${sourcePlistPath}`,
        );
      }

      // Copy Info.plist to iOS project directory
      copyFileSync(sourcePlistPath, destPlistPath);

      xcodeProject.addFile(destPlistPath, extensionGroup.uuid, {
        target: target.uuid,
        lastKnownFileType: "text.plist.xml",
      });

      // Copy and add entitlements file for extension
      const sourceEntitlementsPath = resolve(
        packageRoot,
        "BroadcastExtension",
        "BroadcastExtension.entitlements",
      );
      const destEntitlementsPath = resolve(
        extensionDir,
        `${targetName}.entitlements`,
      );

      // Check if entitlements file exists before copying
      if (!existsSync(sourceEntitlementsPath)) {
        throw new Error(
          `BroadcastExtension entitlements file not found: ${sourceEntitlementsPath}`,
        );
      }

      // Copy entitlements to iOS project directory
      copyFileSync(sourceEntitlementsPath, destEntitlementsPath);

      xcodeProject.addFile(destEntitlementsPath, extensionGroup.uuid, {
        target: target.uuid,
        lastKnownFileType: "text.plist.entitlements",
      });

      // Configure build settings for the extension target
      const buildConfigurations = xcodeProject.pbxXCBuildConfigurationSection();

      // Find the target's build configuration list
      const targetObject = xcodeProject.pbxTargetByName(targetName);
      console.log(`BroadcastExtension: Found target object: ${!!targetObject}`);

      if (targetObject && targetObject.buildConfigurationList) {
        const configListUuid = targetObject.buildConfigurationList;
        const configList = xcodeProject.pbxXCConfigurationList[configListUuid];
        console.log(
          `BroadcastExtension: Found config list with ${configList?.buildConfigurations?.length || 0} configurations`,
        );

        if (configList && configList.buildConfigurations) {
          configList.buildConfigurations.forEach(
            (configRef: any, index: number) => {
              const configUuid = configRef.value;
              const buildConfig = buildConfigurations[configUuid];
              console.log(
                `BroadcastExtension: Setting build config ${index + 1}, name: ${buildConfig?.name}`,
              );

              if (buildConfig && buildConfig.buildSettings) {
                // Set extension-specific build settings
                // Remove quotes from bundle identifier - Xcode adds them automatically
                buildConfig.buildSettings.PRODUCT_BUNDLE_IDENTIFIER =
                  extensionBundleId;
                buildConfig.buildSettings.INFOPLIST_FILE = `${targetName}/${targetName}-Info.plist`;
                buildConfig.buildSettings.CODE_SIGN_ENTITLEMENTS = `${targetName}/${targetName}.entitlements`;
                buildConfig.buildSettings.SWIFT_VERSION = "5.0";
                buildConfig.buildSettings.TARGETED_DEVICE_FAMILY = "1,2";
                buildConfig.buildSettings.IPHONEOS_DEPLOYMENT_TARGET = "12.0";
                buildConfig.buildSettings.PRODUCT_NAME = targetName;
                buildConfig.buildSettings.SKIP_INSTALL = "YES";
                buildConfig.buildSettings.CLANG_ENABLE_MODULES = "YES";
                buildConfig.buildSettings.DEFINES_MODULE = "YES";
                buildConfig.buildSettings.PRODUCT_MODULE_NAME = targetName;
                buildConfig.buildSettings.ALWAYS_EMBED_SWIFT_STANDARD_LIBRARIES =
                  "YES";
                buildConfig.buildSettings.APPLICATION_EXTENSION_API_ONLY =
                  "YES";
                buildConfig.buildSettings.COPY_PHASE_STRIP = "NO";
                buildConfig.buildSettings.DEBUG_INFORMATION_FORMAT = "dwarf";
                buildConfig.buildSettings.ENABLE_TESTABILITY = "YES";
                buildConfig.buildSettings.GCC_C_LANGUAGE_STANDARD = "gnu11";
                buildConfig.buildSettings.GCC_DYNAMIC_NO_PIC = "NO";
                buildConfig.buildSettings.GCC_NO_COMMON_BLOCKS = "YES";
                buildConfig.buildSettings.GCC_OPTIMIZATION_LEVEL = "0";
                buildConfig.buildSettings.GCC_PREPROCESSOR_DEFINITIONS = [
                  "DEBUG=1",
                  "$(inherited)",
                ];
                buildConfig.buildSettings.GCC_WARN_64_TO_32_BIT_CONVERSION =
                  "YES";
                buildConfig.buildSettings.GCC_WARN_ABOUT_RETURN_TYPE =
                  "YES_ERROR";
                buildConfig.buildSettings.GCC_WARN_UNDECLARED_SELECTOR = "YES";
                buildConfig.buildSettings.GCC_WARN_UNINITIALIZED_AUTOS =
                  "YES_AGGRESSIVE";
                buildConfig.buildSettings.GCC_WARN_UNUSED_FUNCTION = "YES";
                buildConfig.buildSettings.GCC_WARN_UNUSED_VARIABLE = "YES";
                buildConfig.buildSettings.MTL_ENABLE_DEBUG_INFO =
                  "INCLUDE_SOURCE";
                buildConfig.buildSettings.MTL_FAST_MATH = "YES";
                buildConfig.buildSettings.ONLY_ACTIVE_ARCH = "YES";
                buildConfig.buildSettings.SWIFT_ACTIVE_COMPILATION_CONDITIONS =
                  "DEBUG";
                buildConfig.buildSettings.SWIFT_OPTIMIZATION_LEVEL = "-Onone";

                console.log(
                  `BroadcastExtension: Set PRODUCT_BUNDLE_IDENTIFIER to: ${buildConfig.buildSettings.PRODUCT_BUNDLE_IDENTIFIER}`,
                );
              }
            },
          );
        }
      }

      // Add framework dependencies
      const frameworks = [
        "ReplayKit.framework",
        "Foundation.framework",
        "AVFoundation.framework",
      ];

      frameworks.forEach((framework) => {
        xcodeProject.addFramework(framework, {
          target: target.uuid,
          link: true,
        });
      });

      // Add the extension to the main app's dependencies (mainTarget already retrieved above)
      if (mainTarget && target) {
        xcodeProject.addTargetDependency(mainTarget.uuid, [target.uuid]);

        // Add the extension to embed app extensions build phase
        try {
          xcodeProject.addBuildPhase(
            [],
            "PBXCopyFilesBuildPhase",
            "Embed App Extensions",
            mainTarget.uuid,
            "app_extension",
          );
        } catch (error) {
          console.warn(`Could not add embed build phase: ${error}`);
        }
      }
    } catch (error) {
      console.warn(`Failed to configure broadcast extension: ${error}`);
    }

    return config;
  });

  return config;
};

const withStreamplaceReactNativeWebRTC: ConfigPlugin<WebRTCConfigOptions> = (
  config,
  options = {},
) => {
  config = RNWebRTCPlugin(config);
  config = withWorkingAndroidWebRTCAudio(config);
  config = withWorkingIOSWebRTCAudio(config);

  // Add broadcast extension if requested
  if (options.broadcastExtension) {
    config = withBroadcastExtension(config, options.broadcastExtension);
  }

  return config;
};

export default withStreamplaceReactNativeWebRTC;
