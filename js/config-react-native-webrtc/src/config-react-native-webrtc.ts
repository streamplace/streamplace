import RNWebRTCPlugin from "@config-plugins/react-native-webrtc";
import { ExpoConfig } from "expo/config";
import {
  ConfigPlugin,
  IOSConfig,
  withAppDelegate,
  withMainApplication,
  withXcodeProject,
} from "expo/config-plugins";
import { resolve } from "path";

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

export default function withStreamplaceReactNativeWebRTC(config: ExpoConfig) {
  config = RNWebRTCPlugin(config);
  config = withWorkingAndroidWebRTCAudio(config);
  config = withWorkingIOSWebRTCAudio(config);
  return config;
}
