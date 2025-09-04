OUT_DIR?="bin"
$(shell mkdir -p $(OUT_DIR))

.PHONY: default
default: app node

VERSION?=$(shell go run ./pkg/config/git/git.go -v)
VERSION_NO_V=$(subst v,,$(VERSION))
VERSION_ELECTRON=$(subst -,-z,$(subst v,,$(VERSION)))
UUID?=$(shell go run ./pkg/config/uuid/uuid.go)
BRANCH?=$(shell go run ./pkg/config/git/git.go --branch)

BUILDOS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
BUILDARCH ?= $(shell uname -m | tr '[:upper:]' '[:lower:]')
ifeq ($(BUILDARCH),aarch64)
		BUILDARCH=arm64
endif
ifeq ($(BUILDARCH),x86_64)
		BUILDARCH=amd64
endif
MACOS_VERSION_FLAG=
ifeq ($(BUILDOS),darwin)
	MACOS_VERSION_FLAG=-mmacosx-version-min=$(shell sw_vers -productVersion)
endif
BUILDDIR?=build-$(BUILDOS)-$(BUILDARCH)
SHARED_LD_LIBRARY_PATH=$(shell pwd)/$(BUILDDIR)/lib/usr/local/lib/x86_64-linux-gnu
SHARED_DYLD_LIBRARY_PATH=$(shell pwd)/$(BUILDDIR)/lib/usr/local/lib
SHARED_PKG_CONFIG_PATH=$(shell pwd)/$(BUILDDIR)/meson-uninstalled

.PHONY: version
version:
	@go run ./pkg/config/git/git.go -v \
	&& go run ./pkg/config/git/git.go --env -o .ci/build.env

.PHONY: install
install:
	pnpm install

.PHONY: app-and-node
app-and-node:
	$(MAKE) app
	$(MAKE) node

.PHONY: app-and-node-and-test
app-and-node-and-test:
	$(MAKE) app
	$(MAKE) node
	$(MAKE) test
.PHONY: app
app: schema install
	pnpm run build

.PHONY: node
node: schema
	$(MAKE) meson-setup
	meson compile -C $(BUILDDIR) streamplace

js/app/dist/index.html: install
	pnpm run build

.PHONY: dev-setup
dev-setup: schema install js/app/dist/index.html
	$(MAKE) dev-setup-meson
	$(MAKE) dev

.PHONY: dev-setup-meson
dev-setup-meson:
	$(MAKE) dev-setup-meson-configure
	$(MAKE) dev-setup-meson-compile

.PHONY: dev-setup-meson-configure
dev-setup-meson-configure:
	meson setup --default-library=shared $(BUILDDIR) $(SHARED_OPTS)
	meson configure --default-library=shared $(BUILDDIR) $(SHARED_OPTS)

.PHONY: dev-setup-meson-compile
dev-setup-meson-compile:
	meson compile -C $(BUILDDIR) streamplace
	meson install --destdir lib -C $(BUILDDIR)

.PHONY: dev
dev:
	cp ./util/streamplace-dev.sh $(BUILDDIR)/streamplace
	PKG_CONFIG_PATH=$(SHARED_PKG_CONFIG_PATH) \
	LD_LIBRARY_PATH=$(SHARED_LD_LIBRARY_PATH) \
	DYLD_LIBRARY_PATH=$(SHARED_DYLD_LIBRARY_PATH) \
	CGO_LDFLAGS="$(MACOS_VERSION_FLAG)" go build -o $(BUILDDIR)/libstreamplace ./cmd/libstreamplace/...

.PHONY: golangci-lint
golangci-lint:
	@PKG_CONFIG_PATH=$(SHARED_PKG_CONFIG_PATH) \
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint run -c ./.golangci.yaml

.PHONY: dev-test
dev-test:
	PKG_CONFIG_PATH=$(SHARED_PKG_CONFIG_PATH) \
	LD_LIBRARY_PATH=$(SHARED_LD_LIBRARY_PATH) \
	DYLD_LIBRARY_PATH=$(SHARED_DYLD_LIBRARY_PATH) \
	go test -p 1 -timeout 300s ./...

.PHONY: schema
schema:
	mkdir -p js/app/generated \
	&& go run pkg/crypto/signers/eip712/export-schema/export-schema.go > js/app/generated/eip712-schema.json

.PHONY: lexicons
lexicons:
	$(MAKE) go-lexicons \
	&& $(MAKE) js-lexicons \
	&& $(MAKE) md-lexicons \
	&& make fix

.PHONY: go-lexicons
go-lexicons:
	rm -rf ./pkg/streamplace \
	&& mkdir -p ./pkg/streamplace \
	&& rm -rf ./pkg/streamplace/cbor_gen.go \
	&& $(MAKE) lexgen \
	&& sed -i.bak 's/\tutil/\/\/\tutil/' $$(find ./pkg/streamplace -type f) \
	&& go run golang.org/x/tools/cmd/goimports@latest -w $$(find ./pkg/streamplace -type f) \
	&& go run ./pkg/gen/gen.go \
	&& $(MAKE) lexgen \
	&& find . | grep bak$$ | xargs rm \
	&& rm -rf api

.PHONY: js-lexicons
js-lexicons:
	node_modules/.bin/lex gen-api ./js/streamplace/src/lexicons $$(find ./lexicons -type f -name '*.json') --yes \
		&& echo 'import { ComAtprotoRepoCreateRecord, ComAtprotoRepoDeleteRecord, ComAtprotoRepoGetRecord, ComAtprotoRepoListRecords } from "@atproto/api"' >> ./js/streamplace/src/lexicons/index.ts \
		&& sed -i.bak "s/'\.\.\/\.\.\/app/'@atproto\/api\/src\/client\/types\/app/" $$(find ./js/streamplace/src/lexicons/types/place/stream -type f) \
		&& sed -i.bak "s/'\.\.\/\.\.\/\.\.\/app/'@atproto\/api\/src\/client\/types\/app/" $$(find ./js/streamplace/src/lexicons/types/place/stream -type f) \
		&& sed -i.bak "s/'\.\.\/\.\.\/com/'@atproto\/api\/src\/client\/types\/com/" $$(find ./js/streamplace/src/lexicons/types/place/stream -type f) \
		&& sed -i.bak "s/'\.\.\/\.\.\/\.\.\/com/'@atproto\/api\/src\/client\/types\/com/" $$(find ./js/streamplace/src/lexicons/types/place/stream -type f) \
		&& sed -i.bak 's/AppBskyGraphBlock\.Main/AppBskyGraphBlock\.Record/' $$(find ./js/streamplace/src/lexicons/types/place/stream -type f) \
		&& sed -i.bak 's/PlaceStreamChatProfile\.Main/PlaceStreamChatProfile\.Record/' $$(find ./js/streamplace/src/lexicons/types/place/stream -type f) \
		&& sed -i.bak "s/import\ \*\ as\ AppBskyFeedDefs\ from\ '.\/defs'/import \{ AppBskyFeedDefs } from '@atproto\/api'/" $$(find ./js/streamplace/src/lexicons/types -type f) \
		&& sed -i.bak "s/import\ \*\ as\ AppBskyActorDefs\ from\ '.\/defs'/import \{ AppBskyActorDefs } from '@atproto\/api'/" $$(find ./js/streamplace/src/lexicons -type f) \
		&& sed -i.bak "s/import\ \*\ as\ ComAtprotoAdminDefs\ from\ .*$$/import \{ ComAtprotoAdminDefs } from '@atproto\/api'/" $$(find ./js/streamplace/src/lexicons -type f) \
		&& sed -i.bak "s/import\ \*\ as\ ComAtprotoRepoStrongRef\ from\ .*$$/import \{ ComAtprotoRepoStrongRef } from '@atproto\/api'/" $$(find ./js/streamplace/src/lexicons -type f) \
		&& sed -i.bak "s/import\ \*\ as\ ComAtprotoModerationDefs\ from\ .*$$/import \{ ComAtprotoModerationDefs } from '@atproto\/api'/" $$(find ./js/streamplace/src/lexicons -type f) \
		&& npx prettier --write $$(find ./js/streamplace/src/lexicons -type f -name '*.ts') \
		&& find . | grep bak$$ | xargs rm

.PHONY: md-lexicons
md-lexicons:
	pnpm exec lexmd \
	    ./lexicons \
		.build/temp \
		subprojects/atproto/lexicons \
		js/docs/src/content/docs/lex-reference/openapi.json \
	&& ls -R .build/temp \
	&& cp -rf .build/temp/place/stream/* js/docs/src/content/docs/lex-reference/ \
	&& rm -rf .build/temp \
	&& $(MAKE) fix

.PHONY: lexgen
lexgen:
	$(MAKE) lexgen-types
	$(MAKE) lexgen-server

.PHONY: lexgen-types
lexgen-types:
	go run github.com/bluesky-social/indigo/cmd/lexgen \
		-outdir ./pkg/spxrpc \
		--build-file util/lexgen-types.json \
		--external-lexicons subprojects/atproto/lexicons \
		lexicons/place/stream \
		./subprojects/atproto/lexicons

.PHONY: lexgen-server
lexgen-server:
	mkdir -p ./pkg/spxrpc \
	&& go run github.com/bluesky-social/indigo/cmd/lexgen \
		--gen-server \
		--types-import place.stream:stream.place/streamplace/pkg/streamplace \
		--types-import app.bsky:github.com/bluesky-social/indigo/api/bsky \
		--types-import com.atproto:github.com/bluesky-social/indigo/api/atproto \
		--types-import chat.bsky:github.com/bluesky-social/indigo/api/chat \
		--types-import tools.ozone:github.com/bluesky-social/indigo/api/ozone \
		-outdir ./pkg/spxrpc \
		--build-file util/lexgen-types.json \
		--external-lexicons subprojects/atproto/lexicons \
		--package spxrpc \
		lexicons/place/stream \
		lexicons/app/bsky \
		lexicons/com/atproto

.PHONY: ci-lexicons
ci-lexicons:
	$(MAKE) lexicons \
	&& if ! git diff --exit-code >/dev/null; then echo "lexicons are out of date, run 'make lexicons' to fix"; exit 1; fi

.PHONY: test
test:
	meson test -C $(BUILDDIR) go-tests

# test to make sure we haven't added any more dynamic dependencies
LINUX_LINK_COUNT=5
.PHONY: link-test-linux
link-test-linux:
	count=$(shell ldd ./build-linux-amd64/streamplace | wc -l) \
	&& echo $$count \
	&& if [ "$$count" != "$(LINUX_LINK_COUNT)" ]; then echo "ldd reports new libaries linked! want $(LINUX_LINK_COUNT) got $$count" \
		&& ldd ./build-linux-amd64/streamplace \
		&& exit 1; \
	fi

MACOS_LINK_COUNT=10
.PHONY: link-test-macos
link-test-macos:
	count=$(shell otool -L ./build-darwin-arm64/streamplace | wc -l | xargs) \
	&& echo $$count \
	&& if [ "$$count" != "$(MACOS_LINK_COUNT)" ]; then echo "otool -L reports new libaries linked! want $(MACOS_LINK_COUNT) got $$count" \
		&& otool -L ./build-darwin-arm64/streamplace \
		&& exit 1; \
	fi

WINDOWS_LINK_COUNT=16
.PHONY: link-test-windows
link-test-windows:
	count=$(shell x86_64-w64-mingw32-objdump -p ./build-windows-amd64/streamplace.exe | grep "DLL Name" | tr '[:upper:]' '[:lower:]' | sort | uniq | wc -l | xargs) \
	&& echo $$count \
	&& if [ "$$count" != "$(WINDOWS_LINK_COUNT)" ]; then echo "x86_64-w64-mingw32-objdump -p reports new libaries linked! want $(WINDOWS_LINK_COUNT) got $$count" \
		&& x86_64-w64-mingw32-objdump -p ./build-windows-amd64/streamplace.exe | grep "DLL Name" \
		&& exit 1; \
	fi

.PHONY: all
all: version install app test node-all-platforms android

.PHONY: ci
ci: version install app node-all-platforms ci-upload-node

.PHONY: ci-ios
ci-ios: version install app
	$(MAKE) ios
	$(MAKE) ci-upload-ios

.PHONY: ci-desktop-darwin
ci-desktop-darwin: version install
	./util/mac-codesign.sh \
	&& for arch in amd64 arm64; do \
		curl -v --fail-with-body -O "$$CI_API_V4_URL/projects/$$CI_PROJECT_ID/packages/generic/$(BRANCH)/$(VERSION)/streamplace-$(VERSION)-darwin-$$arch.tar.gz" || exit 1 \
		&& tar -xzvf streamplace-$(VERSION)-darwin-$$arch.tar.gz \
		&& ./streamplace --version \
		&& ./streamplace self-test \
		&& mkdir -p build-darwin-$$arch \
		&& mv ./streamplace ./build-darwin-$$arch/streamplace; \
	done \
	&& $(MAKE) desktop-darwin \
	&& for arch in amd64 arm64; do \
		export file=streamplace-desktop-$(VERSION)-darwin-$$arch.zip \
		&& $(MAKE) ci-upload-file upload_file=$$file \
		&& export file=streamplace-desktop-$(VERSION)-darwin-$$arch.dmg \
		&& $(MAKE) ci-upload-file upload_file=$$file; \
	done

.PHONY: ci-android
ci-android: version install android ci-upload-android

.PHONY: ci-android-debug
ci-android-debug: version install
	pnpm run app prebuild
	$(MAKE) android-debug
	$(MAKE) ci-upload-android-debug

.PHONY: ci-android-release
ci-android-release: version install
	pnpm run app prebuild
	$(MAKE) android-release
	$(MAKE) ci-upload-android-release

.PHONY: ci-test
ci-test: app
	meson setup $(BUILDDIR) $(OPTS)
	meson test -C $(BUILDDIR) go-tests

.PHONY: ci-npm-release
ci-npm-release: install
	echo //registry.npmjs.org/:_authToken=$$NPM_TOKEN > ~/.npmrc \
	&& npx lerna publish from-package --yes

ANDROID_KEYSTORE_PASSWORD?=streamplace
ANDROID_KEYSTORE_ALIAS?=alias_name
ANDROID_KEYSTORE_BASE64?=

.PHONY: android-keystore
android-keystore:
	if [ -n "$$ANDROID_KEYSTORE_BASE64" ]; then \
		echo "$$ANDROID_KEYSTORE_BASE64" | base64 -d > my-release-key.keystore; \
	fi; \
	if [ ! -f my-release-key.keystore ]; then \
		keytool -genkey -v -keystore my-release-key.keystore -alias alias_name -keyalg RSA -keysize 2048 -validity 10000 -storepass $(ANDROID_KEYSTORE_PASSWORD) -keypass $(ANDROID_KEYSTORE_PASSWORD) -dname "CN=Streamplace, OU=Streamplace, O=Streamplace, L=Streamplace, S=Streamplace, C=US"; \
	fi

.PHONY: android
android: app .build/bundletool.jar
	$(MAKE) android-release
	$(MAKE) android-debug

.PHONY: android-release
android-release: .build/bundletool.jar android-keystore
	export NODE_ENV=production \
	&& cd ./js/app/android \
	&& ./gradlew :app:bundleRelease \
	&& cd - \
	&& mv ./js/app/android/app/build/outputs/bundle/release/app-release.aab ./bin/streamplace-$(VERSION)-android-release.aab \
	&& cd bin \
	&& java -jar ../.build/bundletool.jar build-apks --ks ../my-release-key.keystore --ks-key-alias alias_name --ks-pass pass:$(ANDROID_KEYSTORE_PASSWORD) --bundle=streamplace-$(VERSION)-android-release.aab --output=streamplace-$(VERSION)-android-release.apks --mode=universal \
	&& unzip streamplace-$(VERSION)-android-release.apks && mv universal.apk streamplace-$(VERSION)-android-release.apk && rm toc.pb

.PHONY: android-debug
android-debug: .build/bundletool.jar android-keystore
	export NODE_ENV=production \
	&& cd ./js/app/android \
	&& ./gradlew :app:bundleDebug \
	&& cd - \
	&& mv ./js/app/android/app/build/outputs/bundle/debug/app-debug.aab ./bin/streamplace-$(VERSION)-android-debug.aab \
	&& cd bin \
	&& java -jar ../.build/bundletool.jar build-apks --ks ../my-release-key.keystore --ks-key-alias alias_name --ks-pass pass:$(ANDROID_KEYSTORE_PASSWORD) --bundle=streamplace-$(VERSION)-android-debug.aab --output=streamplace-$(VERSION)-android-debug.apks --mode=universal \
	&& unzip streamplace-$(VERSION)-android-debug.apks && mv universal.apk streamplace-$(VERSION)-android-debug.apk && rm toc.pb

.PHONY: ios
ios: app
	xcodebuild \
		-workspace ./js/app/ios/Streamplace.xcworkspace \
		-sdk iphoneos \
		-configuration Release \
		-scheme Streamplace \
		-archivePath ./bin/streamplace-$(VERSION)-ios-release.xcarchive \
		CODE_SIGN_IDENTITY=- \
		AD_HOC_CODE_SIGNING_ALLOWED=YES \
		CODE_SIGN_STYLE=Automatic \
		DEVELOPMENT_TEAM=ZZZZZZZZZZ \
		clean archive | xcpretty \
	&& cd bin \
	&& tar -czvf streamplace-$(VERSION)-ios-release.xcarchive.tar.gz streamplace-$(VERSION)-ios-release.xcarchive

# xcodebuild -exportArchive -archivePath ./bin/streamplace-$(VERSION)-ios-release.xcarchive -exportOptionsPlist ./js/app/exportOptions.plist -exportPath ./bin/streamplace-$(VERSION)-ios-release.ipa

.build/bundletool.jar:
	mkdir -p .build \
	&& curl -L -o ./.build/bundletool.jar https://github.com/google/bundletool/releases/download/1.17.0/bundletool-all-1.17.0.jar

BASE_OPTS = \
		--buildtype=debugoptimized \
		-D "gst-plugins-base:audioresample=enabled" \
		-D "gst-plugins-base:playback=enabled" \
		-D "gst-plugins-base:opus=enabled" \
		-D "gst-plugins-base:gio-typefinder=enabled" \
		-D "gst-plugins-base:videotestsrc=enabled" \
		-D "gst-plugins-base:videoconvertscale=enabled" \
		-D "gst-plugins-base:typefind=enabled" \
		-D "gst-plugins-base:compositor=enabled" \
		-D "gst-plugins-base:videorate=enabled" \
		-D "gst-plugins-base:app=enabled" \
		-D "gst-plugins-base:audiorate=enabled" \
		-D "gst-plugins-base:audiotestsrc=enabled" \
		-D "gst-plugins-base:audioconvert=enabled" \
		-D "gst-plugins-good:matroska=enabled" \
		-D "gst-plugins-good:multifile=enabled" \
		-D "gst-plugins-good:rtp=enabled" \
		-D "gst-plugins-bad:fdkaac=enabled" \
		-D "gst-plugins-good:audioparsers=enabled" \
		-D "gst-plugins-good:isomp4=enabled" \
		-D "gst-plugins-good:png=enabled" \
		-D "gst-plugins-good:videobox=enabled" \
		-D "gst-plugins-good:jpeg=enabled" \
		-D "gst-plugins-good:audioparsers=enabled" \
		-D "gst-plugins-bad:videoparsers=enabled" \
		-D "gst-plugins-bad:mpegtsmux=enabled" \
		-D "gst-plugins-bad:mpegtsdemux=enabled" \
		-D "gst-plugins-bad:codectimestamper=enabled" \
		-D "gst-plugins-bad:opus=enabled" \
		-D "gst-plugins-ugly:x264=enabled" \
		-D "gst-plugins-ugly:gpl=enabled" \
		-D "x264:asm=enabled" \
		-D "gstreamer-full:gst-full=enabled" \
		-D "gstreamer-full:gst-full-plugins=libgstopusparse.a;libgstcodectimestamper.a;libgstrtp.a;libgstaudioresample.a;libgstlibav.a;libgstmatroska.a;libgstmultifile.a;libgstjpeg.a;libgstaudiotestsrc.a;libgstaudioconvert.a;libgstaudioparsers.a;libgstfdkaac.a;libgstisomp4.a;libgstapp.a;libgstvideoconvertscale.a;libgstvideobox.a;libgstvideorate.a;libgstpng.a;libgstcompositor.a;libgstaudiorate.a;libgstx264.a;libgstopus.a;libgstvideotestsrc.a;libgstvideoparsersbad.a;libgstaudioparsers.a;libgstmpegtsmux.a;libgstmpegtsdemux.a;libgstplayback.a;libgsttypefindfunctions.a;libgstcoretracers.a" \
		-D "gstreamer-full:gst-full-libraries=gstreamer-controller-1.0,gstreamer-plugins-base-1.0,gstreamer-pbutils-1.0" \
		-D "gstreamer-full:gst-full-elements=coreelements:concat,filesrc,filesink,queue,queue2,multiqueue,typefind,tee,capsfilter,fakesink,identity" \
		-D "gstreamer-full:bad=enabled" \
		-D "gstreamer-full:tls=disabled" \
		-D "gstreamer-full:libav=enabled" \
		-D "gstreamer-full:ugly=enabled" \
		-D "gstreamer-full:gpl=enabled" \
		-D "gstreamer-full:gst-full-typefind-functions=" \
		-D "gstreamer:coretracers=enabled" \
		-D "gstreamer-full:glib_assert=false" \
		-D "gstreamer:glib_assert=false" \
		-D "gst-plugins-good:glib_assert=false" \
		-D "gst-plugins-bad:glib_assert=false" \
		-D "gst-plugins-base:glib_assert=false" \
		-D "gst-plugins-ugly:glib_assert=false" \
		-D "glib:glib_assert=false" \
		-D "glib:glib_assert=false" \
		-D "gst-libav:glib_assert=false" \
		-D "gst-plugins-good:adaptivedemux2=disabled"

OPTS = \
	$(BASE_OPTS) \
	-D "gstreamer-full:gst-full-target-type=static_library" \
	-D "gstreamer:registry=false"

SHARED_OPTS = \
	$(BASE_OPTS) \
	-D "FFmpeg:default_library=shared" \
	-D "gstreamer:registry=true"

.PHONY: meson-setup
meson-setup:
	@meson setup $(BUILDDIR) $(OPTS)
	@meson configure $(BUILDDIR) $(OPTS)

.PHONY: node-all-platforms
node-all-platforms: app
	meson setup build-linux-amd64 $(OPTS) --buildtype debugoptimized
	meson compile -C build-linux-amd64 archive
	$(MAKE) link-test-linux
	$(MAKE) linux-arm64
	$(MAKE) windows-amd64
	$(MAKE) windows-amd64-startup-test
	$(MAKE) desktop-linux
	$(MAKE) desktop-windows-amd64

.PHONY: desktop-linux
desktop-linux:
	$(MAKE) desktop-linux-amd64
	$(MAKE) desktop-linux-arm64

.PHONY: desktop-linux-amd64
desktop-linux-amd64:
	cd js/desktop \
	&& pnpm run make --platform linux --arch x64 \
	&& cd - \
	&& mv "js/desktop/out/make/AppImage/x64/Streamplace-$(VERSION_ELECTRON)-x64.AppImage" ./bin/streamplace-desktop-$(VERSION)-linux-amd64.AppImage

.PHONY: desktop-linux-arm64
desktop-linux-arm64:
	cd js/desktop \
	&& pnpm run make --platform linux --arch arm64 \
	&& cd - \
	&& mv "js/desktop/out/make/AppImage/arm64/Streamplace-$(VERSION_ELECTRON)-arm64.AppImage" ./bin/streamplace-desktop-$(VERSION)-linux-arm64.AppImage

.PHONY: desktop-windows-amd64
desktop-windows-amd64:
	cd js/desktop \
	&& pnpm run make --platform win32 --arch x64 \
	&& cd - \
	&& export SUM=$$(cat ./js/desktop/out/make/squirrel.windows/x64/streamplace_desktop-$(VERSION_ELECTRON)-full.nupkg | openssl sha1 | awk '{ print $$2 }') \
	&& echo $$SUM > ./bin/streamplace-desktop-$(VERSION)-windows-amd64.nupkg.sha1 \
	&& mv "js/desktop/out/make/squirrel.windows/x64/streamplace_desktop-$(VERSION_ELECTRON)-full.nupkg" ./bin/streamplace-desktop-$(VERSION)-windows-amd64.$$SUM.nupkg \
	&& mv "js/desktop/out/make/squirrel.windows/x64/Streamplace-$(VERSION_ELECTRON) Setup.exe" ./bin/streamplace-desktop-$(VERSION)-windows-amd64.exe

.PHONY: linux-amd64
linux-amd64:
	meson setup --buildtype debugoptimized build-linux-amd64 $(OPTS)
	meson compile -C build-linux-amd64 archive

.PHONY: linux-arm64
linux-arm64:
	rustup target add aarch64-unknown-linux-gnu
	meson setup --cross-file util/linux-arm64-gnu.ini --buildtype debugoptimized build-linux-arm64 $(OPTS)
	meson compile -C build-linux-arm64 archive

.PHONY: windows-amd64
windows-amd64:
	rustup target add x86_64-pc-windows-gnu
	$(MAKE) windows-amd64-meson-setup
	meson compile -C build-windows-amd64 archive 2>&1 | grep -v drectve
	$(MAKE) link-test-windows

.PHONY: windows-amd64-meson-setup
windows-amd64-meson-setup:
	meson setup --cross-file util/windows-amd64-gnu.ini --buildtype debugoptimized build-windows-amd64 $(OPTS)

# unbuffer here is a workaround for wine trying to pop up a terminal window and failing
.PHONY: windows-amd64-startup-test
windows-amd64-startup-test:
	bash -c 'set -euo pipefail && unbuffer wine64 ./build-windows-amd64/streamplace.exe self-test | cat'

.PHONY: darwin-amd64
darwin-amd64:
	export CC=x86_64-apple-darwin24.4-clang \
	&& export CC_X86_64_APPLE_DARWIN=x86_64-apple-darwin24.4-clang \
	&& export CXX_X86_64_APPLE_DARWIN=x86_64-apple-darwin24.4-clang++ \
	&& export AR_X86_64_APPLE_DARWIN=x86_64-apple-darwin24.4-ar \
	&& export CARGO_TARGET_X86_64_APPLE_DARWIN_LINKER=x86_64-apple-darwin24.4-clang \
	&& export LD=x86_64-apple-darwin24.4-ld \
	&& export CROSS_COMPILE=1 \
	&& meson setup --buildtype debugoptimized --cross-file util/osxcross-darwin-amd64.ini build-darwin-amd64 $(OPTS) \
	&& meson compile -C build-darwin-amd64 subprojects/glib-2.82.4/gio/gioenumtypes_h \
	&& meson compile -C build-darwin-amd64 streamplace \
	&& ./util/osxcross-codesign.sh ./build-darwin-amd64/streamplace \
	&& mkdir -p bin \
	&& cd build-darwin-amd64 \
	&& tar -czvf ../bin/streamplace-$(VERSION)-darwin-amd64.tar.gz ./streamplace \
	&& cd -

.PHONY: desktop-darwin-amd64
desktop-darwin-amd64:
	echo "TODO"

.PHONY: darwin-amd64
darwin-arm64:
	export CC=aarch64-apple-darwin24.4-clang \
	&& export CC_AARCH64_APPLE_DARWIN=aarch64-apple-darwin24.4-clang \
	&& export CXX_AARCH64_APPLE_DARWIN=aarch64-apple-darwin24.4-clang++ \
	&& export AR_AARCH64_APPLE_DARWIN=aarch64-apple-darwin24.4-ar \
	&& export CARGO_TARGET_AARCH64_APPLE_DARWIN_LINKER=aarch64-apple-darwin24.4-clang \
	&& export LD=aarch64-apple-darwin24.4-ld \
	&& export CROSS_COMPILE=1 \
	&& meson setup --buildtype debugoptimized --cross-file util/osxcross-darwin-arm64.ini build-darwin-arm64 $(OPTS) \
	&& meson compile -C build-darwin-arm64 streamplace \
	&& ./util/osxcross-codesign.sh ./build-darwin-arm64/streamplace \
	&& mkdir -p bin \
	&& cd build-darwin-arm64 \
	&& tar -czvf ../bin/streamplace-$(VERSION)-darwin-arm64.tar.gz ./streamplace \
	&& cd -

.PHONY: desktop-darwin-arm64
desktop-darwin-arm64:
	echo "TODO"

.PHONY: desktop-darwin
desktop-darwin:
	export DEBUG="electron-osx-sign*" \
	&& cd js/desktop \
	&& pnpm run make --platform darwin --arch arm64 \
	&& pnpm run make --platform darwin --arch x64 \
	&& cd - \
	&& mv js/desktop/out/make/Streamplace-$(VERSION_ELECTRON)-x64.dmg ./bin/streamplace-desktop-$(VERSION)-darwin-amd64.dmg \
	&& mv js/desktop/out/make/Streamplace-$(VERSION_ELECTRON)-arm64.dmg ./bin/streamplace-desktop-$(VERSION)-darwin-arm64.dmg \
	&& mv js/desktop/out/make/zip/darwin/x64/Streamplace-darwin-x64-$(VERSION_ELECTRON).zip ./bin/streamplace-desktop-$(VERSION)-darwin-amd64.zip \
	&& mv js/desktop/out/make/zip/darwin/arm64/Streamplace-darwin-arm64-$(VERSION_ELECTRON).zip ./bin/streamplace-desktop-$(VERSION)-darwin-arm64.zip

.PHONY: selftest-macos
selftest-macos:
	js/desktop/out/Streamplace-darwin-arm64/Streamplace.app/Contents/MacOS/Streamplace -- --self-test

# link your local version of mist for dev
.PHONY: link-mist
link-mist:
	rm -rf subprojects/mistserver
	ln -s $$(realpath ../mistserver) ./subprojects/mistserver

# link your local version of c2pa-go for dev
.PHONY: link-c2pa-go
link-c2pa-go:
	rm -rf subprojects/c2pa_go
	ln -s $$(realpath ../c2pa-go) ./subprojects/c2pa_go

# link your local version of gstreamer
.PHONY: link-gstreamer
link-gstreamer:
	rm -rf subprojects/gstreamer-full
	ln -s $$(realpath ../gstreamer) ./subprojects/gstreamer-full

# link your local version of ffmpeg for dev
.PHONY: link-ffmpeg
link-ffmpeg:
	rm -rf subprojects/FFmpeg
	ln -s $$(realpath ../ffmpeg) ./subprojects/FFmpeg

.PHONY: docker
docker:
	docker build -f docker/local.Dockerfile -t dist.stream.place/streamplace/streamplace:local .

.PHONY: docker-build
docker-build: docker-build-builder docker-build-in-container

.PHONY: docker-test
docker-test: docker-build-builder docker-test-in-container

DOCKER_BUILD_OPTS?=
BUILDER_TARGET?=builder
.PHONY: docker-build-builder
docker-build-builder:
	podman build --target=$(BUILDER_TARGET) --os=linux --arch=amd64 -f docker/build.Dockerfile -t dist.stream.place/streamplace/streamplace:$(BUILDER_TARGET) $(DOCKER_BUILD_OPTS) .

.PHONY: golangci-lint-container
golangci-lint-container: docker-build-builder
	podman run \
		-v $$(pwd):$$(pwd) \
		-w $$(pwd) \
		-e PKG_CONFIG_PATH=$$(pwd)/build-linux-amd64/meson-uninstalled \
		-d \
		--name golangci-lint \
		dist.stream.place/streamplace/streamplace:$(BUILDER_TARGET) \
		tail -f /dev/null
	podman exec golangci-lint mkdir -p js/app/dist
	podman exec golangci-lint touch js/app/dist/skip-build.txt
	podman exec golangci-lint make node

.PHONY: docker-build-in-container
docker-build-in-container:
	podman run -v $$(pwd):$$(pwd) -w $$(pwd) --rm -it dist.stream.place/streamplace/streamplace:$(BUILDER_TARGET) make app-and-node

.PHONY: docker-test-in-container
docker-test-in-container:
	podman run -v $$(pwd):$$(pwd) -w $$(pwd) --rm -it dist.stream.place/streamplace/streamplace:$(BUILDER_TARGET) make app-and-node-and-test

IN_CONTAINER_CMD?=echo 'usage: make in-container IN_CONTAINER_CMD=\"<command>\"'
DOCKER_BIN?=podman
DOCKER_REF?=dist.stream.place/streamplace/streamplace:$(BUILDER_TARGET)
DOCKER_OPTS?=
.PHONY: in-container
in-container: docker-build-builder
	$(DOCKER_BIN) run $(DOCKER_OPTS) -v $$(pwd):$$(pwd) -w $$(pwd) --rm $(DOCKER_REF) bash -c "$(IN_CONTAINER_CMD)"

STREAMPLACE_URL?=https://git.stream.place/streamplace/streamplace/-/package_files/10122/download
.PHONY: docker-release
docker-release:
	cd docker \
	&& docker build -f release.Dockerfile \
	  --build-arg TARGETARCH=$(BUILDARCH) \
		--build-arg STREAMPLACE_URL=$(STREAMPLACE_URL) \
		-t dist.stream.place/streamplace/streamplace \
		.

.PHONY: docker-mistserver
docker-mistserver:
	cd docker \
	&& docker build -f mistserver.Dockerfile \
	  --build-arg TARGETARCH=$(BUILDARCH) \
		--build-arg STREAMPLACE_URL=$(STREAMPLACE_URL) \
		-t dist.stream.place/streamplace/streamplace:mistserver \
		.

FPM_BASE_OPTS= \
	-s dir \
	-t deb \
	-v $(VERSION_NO_V) \
	--force \
	--license=GPL-3.0-or-later \
	--maintainer="Streamplace <support@stream.place>" \
	--vendor="Streamplace" \
	--url="https://stream.place"
SP_ARCH_NAME?=amd64
.PHONY: deb-pkg
deb-pkg:
	fpm $(FPM_BASE_OPTS) \
		-n streamplace \
		-a $(SP_ARCH_NAME) \
		-p bin/streamplace-$(VERSION)-linux-$(SP_ARCH_NAME).deb \
		--deb-systemd=util/systemd/streamplace.service \
		--deb-systemd-restart-after-upgrade \
		--after-install=util/systemd/after-install.sh \
		--description="Live video for the AT Protocol. Solving video for everybody forever." \
		build-linux-$(SP_ARCH_NAME)/streamplace=/usr/bin/streamplace \
	&& fpm $(FPM_BASE_OPTS) \
		-n streamplace-default-http \
		-a $(SP_ARCH_NAME) \
		-d streamplace \
		--deb-systemd-restart-after-upgrade \
		-p bin/streamplace-default-http-$(VERSION)-linux-$(SP_ARCH_NAME).deb \
		--description="Installing this package will install Streamplace as the default HTTP server on ports 80 and 443." \
		util/systemd/streamplace-http.socket=/lib/systemd/system/streamplace-http.socket \
		util/systemd/streamplace-https.socket=/lib/systemd/system/streamplace-https.socket

.PHONY: pkg-linux-amd64
pkg-linux-amd64:
	$(MAKE) deb-pkg SP_ARCH_NAME=amd64

.PHONY: pkg-linux-arm64
pkg-linux-arm64:
	$(MAKE) deb-pkg SP_ARCH_NAME=arm64

.PHONY: pkg-darwin-amd64
pkg-darwin-amd64:
	echo todo

.PHONY: pkg-darwin-arm64
pkg-darwin-arm64:
	echo todo

.PHONY: pkg-windows-amd64
pkg-windows-amd64:
	echo todo

.PHONY: ci-upload
ci-upload: ci-upload-node ci-upload-android

.PHONY: ci-upload-node-linux-amd64
ci-upload-node-linux-amd64:
	$(MAKE) ci-upload-file upload_file=streamplace-$(VERSION)-linux-amd64.tar.gz \
	&& $(MAKE) ci-upload-file upload_file=streamplace-desktop-$(VERSION)-linux-amd64.AppImage \
	&& $(MAKE) ci-upload-file upload_file=streamplace-default-http-$(VERSION)-linux-amd64.deb \
	&& $(MAKE) ci-upload-file upload_file=streamplace-$(VERSION)-linux-amd64.deb

.PHONY: ci-upload-node-linux-arm64
ci-upload-node-linux-arm64:
	$(MAKE) ci-upload-file upload_file=streamplace-$(VERSION)-linux-arm64.tar.gz \
	&& $(MAKE) ci-upload-file upload_file=streamplace-desktop-$(VERSION)-linux-arm64.AppImage \
	&& $(MAKE) ci-upload-file upload_file=streamplace-default-http-$(VERSION)-linux-arm64.deb \
	&& $(MAKE) ci-upload-file upload_file=streamplace-$(VERSION)-linux-arm64.deb

.PHONY: ci-upload-node-darwin-arm64
ci-upload-node-darwin-arm64:
	export file=streamplace-$(VERSION)-darwin-arm64.tar.gz \
	&& $(MAKE) ci-upload-file upload_file=$$file;

.PHONY: ci-upload-node-darwin-amd64
ci-upload-node-darwin-amd64:
	export file=streamplace-$(VERSION)-darwin-amd64.tar.gz \
	&& $(MAKE) ci-upload-file upload_file=$$file;

.PHONY: ci-upload-node-windows-amd64
ci-upload-node-windows-amd64:
	export file=streamplace-$(VERSION)-windows-amd64.zip \
	&& $(MAKE) ci-upload-file upload_file=$$file; \
	export file=streamplace-desktop-$(VERSION)-windows-amd64.exe \
	&& $(MAKE) ci-upload-file upload_file=$$file; \
	export SUM=$$(cat bin/streamplace-desktop-$(VERSION)-windows-amd64.nupkg.sha1) \
	&& export file=streamplace-desktop-$(VERSION)-windows-amd64.$$SUM.nupkg \
	&& $(MAKE) ci-upload-file upload_file=$$file;

.PHONY: ci-upload-node-macos
ci-upload-node-macos: node-all-platforms-macos
	for GOOS in darwin; do \
		for GOARCH in amd64 arm64; do \
			export file=streamplace-$(VERSION)-$$GOOS-$$GOARCH.tar.gz \
			&& $(MAKE) ci-upload-file upload_file=$$file; \
			export file=streamplace-desktop-$(VERSION)-$$GOOS-$$GOARCH.dmg \
			&& $(MAKE) ci-upload-file upload_file=$$file; \
			export file=streamplace-desktop-$(VERSION)-$$GOOS-$$GOARCH.zip \
			&& $(MAKE) ci-upload-file upload_file=$$file; \
		done \
	done;

.PHONY: ci-upload-android
ci-upload-android: android
	$(MAKE) ci-upload-android-debug \
	&& $(MAKE) ci-upload-android-release

.PHONY: ci-upload-android-debug
ci-upload-android-debug:
	$(MAKE) ci-upload-file upload_file=streamplace-$(VERSION)-android-debug.apk \
	&& $(MAKE) ci-upload-file upload_file=streamplace-$(VERSION)-android-debug.aab

.PHONY: ci-upload-android-release
ci-upload-android-release:
	$(MAKE) ci-upload-file upload_file=streamplace-$(VERSION)-android-release.apk \
	&& $(MAKE) ci-upload-file upload_file=streamplace-$(VERSION)-android-release.aab

.PHONY: ci-upload-ios
ci-upload-ios: ios
	$(MAKE) ci-upload-file upload_file=streamplace-$(VERSION)-ios-release.xcarchive.tar.gz

upload_file?=""
.PHONY: ci-upload-file
ci-upload-file:
	curl \
		--fail-with-body \
		--header "JOB-TOKEN: $$CI_JOB_TOKEN" \
		--upload-file bin/$(upload_file) \
		"$$CI_API_V4_URL/projects/$$CI_PROJECT_ID/packages/generic/$(BRANCH)/$(VERSION)/$(upload_file)";

download_file?=""
.PHONY: ci-download-file
ci-download-file:
	curl \
		--fail-with-body \
		--header "JOB-TOKEN: $$CI_JOB_TOKEN" \
		-o bin/$(download_file) \
		"$$CI_API_V4_URL/projects/$$CI_PROJECT_ID/packages/generic/$(BRANCH)/$(VERSION)/$(download_file)";

.PHONY: release
release: install
	$(MAKE) lexicons
	pnpm run release

.PHONY: ci-release
ci-release:
	go install gitlab.com/gitlab-org/release-cli/cmd/release-cli
	go run ./pkg/config/git/git.go -release -o release.yml
	release-cli create-from-file --file release.yml

.PHONY: deb-release
deb-release:
	aptly repo create -distribution=all -component=main streamplace-releases
	aptly mirror create old-version $$S3_PUBLIC_URL/debian all
	aptly mirror update old-version
	aptly repo import old-version streamplace-releases streamplace streamplace-default-http
	aptly repo add streamplace-releases \
		bin/streamplace-default-http-$(VERSION)-linux-arm64.deb \
		bin/streamplace-$(VERSION)-linux-arm64.deb \
		bin/streamplace-default-http-$(VERSION)-linux-amd64.deb \
		bin/streamplace-$(VERSION)-linux-amd64.deb
	aptly snapshot create streamplace-$(VERSION) from repo streamplace-releases
	aptly publish snapshot -distribution=all streamplace-$(VERSION) s3:streamplace-releases:

.PHONY: ci-deb-release
ci-deb-release:
	$(MAKE) ci-download-file download_file=streamplace-default-http-$(VERSION)-linux-amd64.deb
	$(MAKE) ci-download-file download_file=streamplace-$(VERSION)-linux-amd64.deb
	$(MAKE) ci-download-file download_file=streamplace-default-http-$(VERSION)-linux-arm64.deb
	$(MAKE) ci-download-file download_file=streamplace-$(VERSION)-linux-arm64.deb
	echo $$CI_SIGNING_KEY_BASE64 | base64 -d | gpg --import
	gpg --armor --export | gpg --no-default-keyring --keyring trustedkeys.gpg --import
	echo '{"S3PublishEndpoints":{"streamplace-releases":{"region":"'$$S3_REGION'","bucket":"'$$S3_BUCKET_NAME'","endpoint":"'$$S3_ENDPOINT'","acl":"public-read","prefix":"debian"}}}' > ~/.aptly.conf
	$(MAKE) deb-release

.PHONY: check
check: install
	$(MAKE) golangci-lint
	pnpm run check
	if [ "`gofmt -l . | wc -l`" -gt 0 ]; then echo 'gofmt failed, run make fix'; exit 1; fi

.PHONY: fix
fix:
	pnpm run fix
	gofmt -w .

.PHONY: precommit
precommit: dockerfile-hash-precommit

.PHONY: dockefile-hash-precommit
dockerfile-hash-precommit:
	@bash -c 'printf "variables:\n  DOCKERFILE_HASH: `git hash-object docker/build.Dockerfile`" > .ci/dockerfile-hash.yaml' \
	&& git add .ci/dockerfile-hash.yaml

.PHONY: rtcrec
rtcrec:
	go build -o $(BUILDDIR)/rtcrec ./pkg/rtcrec/cmd/...

.PHONY: homebrew
homebrew:
	$(MAKE) ci-download-file download_file=streamplace-$(VERSION)-linux-amd64.tar.gz
	$(MAKE) ci-download-file download_file=streamplace-$(VERSION)-linux-arm64.tar.gz
	$(MAKE) ci-download-file download_file=streamplace-$(VERSION)-darwin-amd64.tar.gz
	$(MAKE) ci-download-file download_file=streamplace-$(VERSION)-darwin-arm64.tar.gz
	git clone git@github.com:streamplace/homebrew-streamplace.git /tmp/homebrew-streamplace
	go run ./pkg/config/git/git.go -homebrew -o /tmp/homebrew-streamplace/Formula/streamplace.rb

.PHONY: ci-homebrew
ci-homebrew:
	git config --global user.name "Streamplace Homebrew Robot"
	git config --global user.email support@stream.place
	mkdir -p ~/.ssh
	echo "Host * \n\
	  StrictHostKeyChecking no \n\
	  UserKnownHostsFile=/dev/null" > ~/.ssh/config
	echo "$$CI_HOMEBREW_KEY_BASE64" | base64 -d > ~/.ssh/id_ed25519
	chmod 600 ~/.ssh/id_ed25519

	chmod 600 ~/.ssh/config
	$(MAKE) homebrew
	cd /tmp/homebrew-streamplace \
	&& git add . \
	&& git commit -m "Update streamplace $(VERSION)" \
	&& git push
