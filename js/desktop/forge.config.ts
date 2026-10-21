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
import os from "os";
import path from "path";
import { mainConfig } from "./webpack.main.config";
import { rendererConfig } from "./webpack.renderer.config";

// Sign (and optionally notarize) a packaged macOS .app on a non-macOS host.
// Electron Forge's built-in osxSign/osxNotarize shells out to Apple's codesign
// and notarytool, so Linux uses rcodesign (apple-codesign) instead. The bundle
// is signed recursively, including the nested Electron helper apps/frameworks.
function signDarwinApp(appPath: string) {
  const args = [
    "sign",
    "--code-signature-flags",
    "runtime",
    "--entitlements-xml-path",
    path.join(__dirname, "entitlements.plist"),
  ];
  let p12Dir: string | undefined;
  try {
    if (process.env.DEVELOPER_CERTIFICATE_BASE64) {
      p12Dir = fs.mkdtempSync(path.join(os.tmpdir(), "streamplace-p12-"));
      const p12File = path.join(p12Dir, "certificate.p12");
      fs.writeFileSync(
        p12File,
        Buffer.from(process.env.DEVELOPER_CERTIFICATE_BASE64, "base64"),
      );
      args.push(
        "--p12-file",
        p12File,
        "--p12-password",
        process.env.DEVELOPER_CERTIFICATE_PASSWORD ?? "",
      );
    }
    args.push(appPath);
    console.log(`signing ${appPath} with rcodesign`);
    child_process.execFileSync("rcodesign", args, { stdio: "inherit" });
  } finally {
    if (p12Dir) fs.rmSync(p12Dir, { recursive: true, force: true });
  }

  if (process.env.APPLE_CODESIGN_API_KEY) {
    // Apple only accepts archives, so submit a zip and then staple the ticket
    // to the bundle that the makers distribute.
    const notarizeDir = fs.mkdtempSync(
      path.join(os.tmpdir(), "streamplace-notarize-"),
    );
    try {
      const zipPath = path.join(notarizeDir, "Streamplace.zip");
      child_process.execFileSync(
        "zip",
        ["-qry", zipPath, path.basename(appPath)],
        { cwd: path.dirname(appPath), stdio: "inherit" },
      );
      child_process.execFileSync(
        "rcodesign",
        [
          "notary-submit",
          "--api-key-file",
          process.env.APPLE_CODESIGN_API_KEY,
          "--wait",
          zipPath,
        ],
        { stdio: "inherit" },
      );
      child_process.execFileSync("rcodesign", ["staple", appPath], {
        stdio: "inherit",
      });
    } finally {
      fs.rmSync(notarizeDir, { recursive: true, force: true });
    }
  }
}

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
    name: "Streamplace",
    appVersion: version,
    buildVersion: version,
    icon: "./assets/images/streamplace-logo",
    extraResource: ["./assets/images/streamplace-logo.png"],
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
        let binary = `../../build-${platform}-${architecture}/streamplace`;
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
      postPackage: async (_config, packageResult) => {
        // macOS uses the built-in osxSign/osxNotarize above. Elsewhere (the
        // builder container) sign each .app with rcodesign so fuses, signing,
        // and notarization all still happen before the makers run.
        if (process.platform === "darwin") {
          return;
        }
        for (const outputPath of packageResult.outputPaths) {
          const app = fs
            .readdirSync(outputPath)
            .find((entry) => entry.endsWith(".app"));
          if (app) {
            signDarwinApp(path.join(outputPath, app));
          }
        }
      },
    },
    rebuildConfig: {},
    makers: [
      new MakerSquirrel({
        iconUrl:
          "https://git.stream.place/-/project/1/uploads/2e5899ffd2b4799ce661cf9b8675e610/streamplace-logo-256.ico",
        setupIcon: "./assets/images/streamplace-logo.ico",
      }),
      // @electron-forge/maker-dmg requires hdiutil, so non-macOS hosts build
      // the DMG from the signed bundle with hack/make-dmg.sh instead.
      ...(process.platform === "darwin"
        ? [
            new MakerDMG(
              {
                icon: "./assets/images/streamplace-logo.icns",
                appPath: "./Streamplace.app",
              },
              ["darwin"],
            ),
          ]
        : []),
      new MakerZIP({}, ["darwin"]),
      // new MakerRpm({}),
      new MakerAppImage({
        options: {
          bin: "Streamplace",
          icon: "./assets/images/streamplace-logo.png",
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
        bucket: "streamplace",
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
