import { MakerDMG } from "@electron-forge/maker-dmg";
import { MakerSquirrel } from "@electron-forge/maker-squirrel";
import { MakerZIP } from "@electron-forge/maker-zip";
import type {
  ForgeConfig,
  ForgePackagerOptions,
} from "@electron-forge/shared-types";
// import { MakerDeb } from "@electron-forge/maker-deb";
// import { MakerRpm } from "@electron-forge/maker-rpm";
import { AutoUnpackNativesPlugin } from "@electron-forge/plugin-auto-unpack-natives";
import { FusesPlugin } from "@electron-forge/plugin-fuses";
import { WebpackPlugin } from "@electron-forge/plugin-webpack";
import { PublisherS3 } from "@electron-forge/publisher-s3";
import { FuseV1Options, FuseVersion } from "@electron/fuses";
import { MakerAppImage } from "@reforged/maker-appimage";
import child_process from "child_process";
import fs from "fs";
import { mainConfig } from "./webpack.main.config";
import { rendererConfig } from "./webpack.renderer.config";

export default async function () {
  // go get the version from the actual script
  let version = child_process
    .execSync("go run ../../pkg/config/git/git.go -v")
    .toString()
    .slice(1);

  let versionCode = child_process
    .execSync("go run ../../pkg/config/git/git.go --js")
    .toString();

  fs.writeFileSync("./src/version.ts", versionCode, "utf8");

  // https://github.com/Squirrel/Squirrel.Windows/issues/1394#issuecomment-2356692821
  // This makes the Desktop app have confusing numbers, but actual relases just use X.Y.Z
  // so it doesn't really matter. Those are just for testing.
  if (version.includes("-")) {
    version = version.replace("-", "-z");
  }

  const packagerConfig: ForgePackagerOptions = {
    asar: true,
    name: "Aquareum",
    appVersion: version,
    buildVersion: version,
    icon: "./assets/images/aquareum-logo",
    extraResource: ["./assets/images/aquareum-logo.png"],
  };

  const config: ForgeConfig = {
    packagerConfig: packagerConfig,
    hooks: {
      prePackage: async (config, plat, arch) => {
        let platform = plat;
        let architecture = arch;
        if (platform === "win32") {
          platform = "windows";
        }
        if (architecture === "x64") {
          architecture = "amd64";
        }
        let binary = `../../build-${platform}-${architecture}/aquareum`;
        if (platform === "windows") {
          binary += ".exe";
        }
        const exists = fs.existsSync(binary);
        if (!exists) {
          throw new Error(
            `could not find ${binary} while building electron bundle. do you need to run make ${platform}-${architecture}?`,
          );
        }

        if (!config.packagerConfig) {
          throw new Error("config.packageConfig undefined");
        }
        if (!config.packagerConfig.extraResource) {
          config.packagerConfig.extraResource = [];
        }
        config.packagerConfig.extraResource = [
          ...config.packagerConfig.extraResource,
          binary,
        ];
      },
      readPackageJson: async (forgeConfig, packageJson) => {
        packageJson.version = version;
        return packageJson;
      },
    },
    rebuildConfig: {},
    makers: [
      new MakerSquirrel({
        iconUrl:
          "https://git.aquareum.tv/-/project/1/uploads/2e5899ffd2b4799ce661cf9b8675e610/aquareum-logo-256.ico",
        setupIcon: "./assets/images/aquareum-logo.ico",
      }),
      new MakerDMG(
        {
          icon: "./assets/images/aquareum-logo.icns",
          appPath: "./Aquareum.app",
        },
        ["darwin"],
      ),
      new MakerZIP({}, ["darwin"]),
      // new MakerRpm({}),
      new MakerAppImage({
        options: {
          bin: "Aquareum",
          icon: "./assets/images/aquareum-logo.png",
        },
      }),
    ],
    plugins: [
      new AutoUnpackNativesPlugin({}),
      new WebpackPlugin({
        mainConfig,
        packageSourceMaps: true,
        renderer: {
          config: rendererConfig,
          entryPoints: [
            {
              html: "./src/index.html",
              js: "./src/renderer.ts",
              name: "main_window",
              preload: {
                js: "./src/preload.ts",
              },
            },
          ],
        },
      }),
      // Fuses are used to enable/disable various Electron functionality
      // at package time, before code signing the application
      new FusesPlugin({
        version: FuseVersion.V1,
        [FuseV1Options.RunAsNode]: false,
        [FuseV1Options.EnableCookieEncryption]: true,
        [FuseV1Options.EnableNodeOptionsEnvironmentVariable]: false,
        [FuseV1Options.EnableNodeCliInspectArguments]: false,
        [FuseV1Options.EnableEmbeddedAsarIntegrityValidation]: false,
        [FuseV1Options.OnlyLoadAppFromAsar]: true,
      }),
    ],
    publishers: [
      new PublisherS3({
        endpoint: "http://192.168.8.136:9000",
        accessKeyId: "minioadmin",
        secretAccessKey: "minioadmin",
        public: true,
        bucket: "aquareum",
        region: "unused",
      }),
    ],
  };
  if (
    process.env.NOTARIZATION_TEAM_ID &&
    process.env.NOTARIZATION_EMAIL &&
    process.env.NOTARIZATION_PASSWORD
  ) {
    packagerConfig.osxNotarize = {
      teamId: process.env.NOTARIZATION_TEAM_ID,
      appleId: process.env.NOTARIZATION_EMAIL,
      appleIdPassword: process.env.NOTARIZATION_PASSWORD,
    };
    packagerConfig.osxSign = {
      optionsForFile: (filePath) => {
        // Here, we keep it simple and return a single entitlements.plist file.
        // You can use this callback to map different sets of entitlements
        // to specific files in your packaged app.
        return {
          entitlements: "./entitlements.plist",
        };
      },
    };
  }
  return config;
}
