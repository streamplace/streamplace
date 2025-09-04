/** @type {Detox.DetoxConfig} */

const { execSync } = require("child_process");
const { resolve } = require("path");

let versionStr;
try {
  versionStr = execSync("go run ../../pkg/config/git/git.go -v", {
    encoding: "utf8",
    cwd: __dirname,
  }).trim();
  console.log(`Version string: ${versionStr}`);
} catch (e) {
  console.error(`Could not get version string: ${e}`);
  process.exit(1);
}

module.exports = {
  testRunner: {
    args: {
      $0: "jest",
      config: "e2e/jest.config.ts",
    },
    jest: {
      setupTimeout: 120000,
    },
  },
  apps: {
    "ios.debug": {
      type: "ios.app",
      binaryPath: "ios/build/Build/Products/Debug-iphonesimulator/YOUR_APP.app",
      build:
        "xcodebuild -workspace ios/YOUR_APP.xcworkspace -scheme YOUR_APP -configuration Debug -sdk iphonesimulator -derivedDataPath ios/build",
    },
    "ios.release": {
      type: "ios.app",
      binaryPath:
        "ios/build/Build/Products/Release-iphonesimulator/YOUR_APP.app",
      build:
        "xcodebuild -workspace ios/YOUR_APP.xcworkspace -scheme YOUR_APP -configuration Release -sdk iphonesimulator -derivedDataPath ios/build",
    },
    "android.debug": {
      type: "android.apk",
      binaryPath: resolve(
        "..",
        "..",
        "bin",
        `streamplace-${versionStr}-android-debug.apk`,
      ),
      build: "cd ../.. && make android-debug",
      // reversePorts: [8081],
    },
    "android.release": {
      type: "android.apk",
      binaryPath: resolve(
        "..",
        "..",
        "bin",
        `streamplace-${versionStr}-android-release.apk`,
      ),
      testBinaryPath: resolve(
        "..",
        "..",
        "bin",
        `streamplace-${versionStr}-android-release-androidTest.apk`,
      ),
      build: "cd ../.. && make android-release",
    },
  },
  devices: {
    simulator: {
      type: "ios.simulator",
      device: {
        type: "iPhone 15",
      },
    },
    attached: {
      type: "android.attached",
      device: {
        adbName: ".*",
      },
    },
    emulator: {
      type: "android.emulator",
      device: {
        avdName: "Pixel_3a_API_30_x86",
      },
    },
  },
  configurations: {
    "ios.sim.debug": {
      device: "simulator",
      app: "ios.debug",
    },
    "ios.sim.release": {
      device: "simulator",
      app: "ios.release",
    },
    "android.att.debug": {
      device: "attached",
      app: "android.debug",
    },
    "android.att.release": {
      device: "attached",
      app: "android.release",
    },
    "android.emu.debug": {
      device: "emulator",
      app: "android.debug",
    },
    "android.emu.release": {
      device: "emulator",
      app: "android.release",
    },
  },
};
