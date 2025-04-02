OUT_DIR?="bin"
$(shell mkdir -p $(OUT_DIR))

.PHONY: default
default: app node

VERSION?=$(shell go run ./pkg/config/git/git.go -v)
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
BUILDDIR?=build-$(BUILDOS)-$(BUILDARCH)

.PHONY: version
version:
	@go run ./pkg/config/git/git.go -v \
	&& go run ./pkg/config/git/git.go --env -o .ci/build.env

.PHONY: install
install:
	yarn install --inline-builds

.PHONY: app-and-node
app-and-node:
	$(MAKE) app
	$(MAKE) node

.PHONY: app
app: schema install
	yarn run build

.PHONY: node
node: schema .build/subprojects2.tar.gz
	$(MAKE) meson-setup
	meson compile -C $(BUILDDIR) streamplace

.PHONY: schema
schema:
	mkdir -p js/app/generated \
	&& go run pkg/crypto/signers/eip712/export-schema/export-schema.go > js/app/generated/eip712-schema.json

.PHONY: lexicons
lexicons:
	$(MAKE) go-lexicons \
	&& $(MAKE) js-lexicons

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
	&& rm -rf ./pkg/streamplace/*.bak \
	&& rm -rf api

.PHONY: js-lexicons
js-lexicons:
	node_modules/.bin/lex gen-api ./js/app/lexicons $$(find ./lexicons -type f -name '*.json') --yes \
		&& echo 'import { ComAtprotoRepoCreateRecord, ComAtprotoRepoDeleteRecord, ComAtprotoRepoGetRecord, ComAtprotoRepoListRecords } from "@atproto/api"' >> ./js/app/lexicons/index.ts \
		&& sed -i.bak "s/'\.\.\/\.\.\/app/'@atproto\/api\/src\/client\/types\/app/" $$(find ./js/app/lexicons/types/place/stream -type f) \
		&& sed -i.bak "s/'\.\.\/\.\.\/com/'@atproto\/api\/src\/client\/types\/com/" $$(find ./js/app/lexicons/types/place/stream -type f) \
		&& sed -i.bak 's/AppBskyGraphBlock\.Main/AppBskyGraphBlock\.Record/' $$(find ./js/app/lexicons/types/place/stream -type f) \
		&& rm -rf ./js/app/lexicons/types/place/stream/*.bak

.PHONY: lexgen
lexgen:
	go run github.com/bluesky-social/indigo/cmd/lexgen --package streamplace \
		--types-import place.stream:stream.place/streamplace/pkg/streamplace \
		-outdir ./pkg/streamplace \
		--prefix place.stream \
		--build-file util/lexgen-build.json \
		lexicons/place/stream \
		../atproto/lexicons

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
all: version install check app test node-all-platforms android

.PHONY: ci
ci: version install check app node-all-platforms ci-upload-node .build/subprojects2.tar.gz

.PHONY: ci-macos
ci-macos: version install check app node-all-platforms-macos ci-upload-node-macos ios ci-upload-ios .build/subprojects2.tar.gz

.PHONY: ci-macos
ci-android: version install check android ci-upload-android .build/subprojects2.tar.gz

.PHONY: ci-test
ci-test: app .build/subprojects2.tar.gz
	meson setup $(BUILDDIR) $(OPTS)
	meson test -C $(BUILDDIR) go-tests

.PHONY: android
android: app .build/bundletool.jar
	$(MAKE) android-release
	$(MAKE) android-debug

.PHONY: android-release
android-release:
	export NODE_ENV=production \
	&& cd ./js/app/android \
	&& ./gradlew :app:bundleRelease \
	&& cd - \
	&& mv ./js/app/android/app/build/outputs/bundle/release/app-release.aab ./bin/streamplace-$(VERSION)-android-release.aab \
	&& cd bin \
	&& java -jar ../.build/bundletool.jar build-apks --ks ../my-release-key.keystore --ks-key-alias alias_name --ks-pass pass:aquareum --bundle=streamplace-$(VERSION)-android-release.aab --output=streamplace-$(VERSION)-android-release.apks --mode=universal \
	&& unzip streamplace-$(VERSION)-android-release.apks && mv universal.apk streamplace-$(VERSION)-android-release.apk && rm toc.pb

.PHONY: android-debug
android-debug:
	export NODE_ENV=production \
	&& cd ./js/app/android \
	&& ./gradlew :app:bundleDebug \
	&& cd - \
	&& mv ./js/app/android/app/build/outputs/bundle/debug/app-debug.aab ./bin/streamplace-$(VERSION)-android-debug.aab \
	&& cd bin \
	&& java -jar ../.build/bundletool.jar build-apks --ks ../my-release-key.keystore --ks-key-alias alias_name --ks-pass pass:aquareum --bundle=streamplace-$(VERSION)-android-debug.aab --output=streamplace-$(VERSION)-android-debug.apks --mode=universal \
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

OPTS = -D "gst-plugins-base:audioresample=enabled" \
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
		-D "gstreamer-full:gst-full-plugins=libgstopusparse.a;libgstcodectimestamper.a;libgstrtp.a;libgstaudioresample.a;libgstlibav.a;libgstmatroska.a;libgstmultifile.a;libgstjpeg.a;libgstaudiotestsrc.a;libgstaudioconvert.a;libgstaudioparsers.a;libgstfdkaac.a;libgstisomp4.a;libgstapp.a;libgstvideoconvertscale.a;libgstvideobox.a;libgstvideorate.a;libgstpng.a;libgstcompositor.a;libgstaudiorate.a;libgstx264.a;libgstopus.a;libgstvideotestsrc.a;libgstvideoparsersbad.a;libgstaudioparsers.a;libgstmpegtsmux.a;libgstmpegtsdemux.a;libgstplayback.a;libgsttypefindfunctions.a" \
		-D "gstreamer-full:gst-full-libraries=gstreamer-controller-1.0,gstreamer-plugins-base-1.0,gstreamer-pbutils-1.0" \
		-D "gstreamer-full:gst-full-target-type=static_library" \
		-D "gstreamer-full:gst-full-elements=coreelements:concat,filesrc,filesink,queue,queue2,multiqueue,typefind,tee,capsfilter,fakesink,identity" \
		-D "gstreamer-full:bad=enabled" \
		-D "gstreamer-full:tls=disabled" \
		-D "gstreamer-full:libav=enabled" \
		-D "gstreamer-full:ugly=enabled" \
		-D "gstreamer-full:gpl=enabled" \
		-D "gstreamer-full:gst-full-typefind-functions="

.PHONY: meson-setup
meson-setup:
	@meson setup $(BUILDDIR) $(OPTS)
	@meson configure $(BUILDDIR) $(OPTS)

.PHONY: node-all-platforms
node-all-platforms: app .build/subprojects2.tar.gz
	meson setup build-linux-amd64 $(OPTS) --buildtype debugoptimized
	meson compile -C build-linux-amd64 archive
	$(MAKE) link-test-linux
	$(MAKE) linux-arm64
	$(MAKE) windows-amd64
	$(MAKE) windows-amd64-startup-test
	$(MAKE) desktop-linux
	$(MAKE) desktop-windows

.PHONY: desktop-linux
desktop-linux:
	cd js/desktop \
	&& yarn run make --platform linux --arch x64 \
	&& yarn run make --platform linux --arch arm64 \
	&& cd - \
	&& mv "js/desktop/out/make/AppImage/x64/Streamplace-$(VERSION_ELECTRON)-x64.AppImage" ./bin/streamplace-desktop-$(VERSION)-linux-amd64.AppImage \
	&& mv "js/desktop/out/make/AppImage/arm64/Streamplace-$(VERSION_ELECTRON)-arm64.AppImage" ./bin/streamplace-desktop-$(VERSION)-linux-arm64.AppImage

.PHONY: desktop-windows
desktop-windows:
	cd js/desktop \
	&& yarn run make --platform win32 --arch x64 \
	&& cd - \
	&& export SUM=$$(cat ./js/desktop/out/make/squirrel.windows/x64/streamplace_desktop-$(VERSION_ELECTRON)-full.nupkg | openssl sha1 | awk '{ print $$2 }') \
	&& echo $$SUM > ./bin/streamplace-desktop-$(VERSION)-windows-amd64.nupkg.sha1 \
	&& mv "js/desktop/out/make/squirrel.windows/x64/streamplace_desktop-$(VERSION_ELECTRON)-full.nupkg" ./bin/streamplace-desktop-$(VERSION)-windows-amd64.$$SUM.nupkg \
	&& mv "js/desktop/out/make/squirrel.windows/x64/Streamplace-$(VERSION_ELECTRON) Setup.exe" ./bin/streamplace-desktop-$(VERSION)-windows-amd64.exe

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

.PHONY: node-all-platforms-macos
node-all-platforms-macos: app .build/subprojects2.tar.gz
	meson setup --buildtype debugoptimized build-darwin-arm64 $(OPTS)
	meson compile -C build-darwin-arm64
	./util/mac-codesign.sh ./build-darwin-arm64/streamplace
	cd build-darwin-arm64 \
	&& tar -czvf ../bin/streamplace-$(VERSION)-darwin-arm64.tar.gz ./streamplace \
	&& cd -
	./build-darwin-arm64/streamplace --version
	./build-darwin-arm64/streamplace self-test
	$(MAKE) link-test-macos
	rustup target add x86_64-apple-darwin
	meson setup --buildtype debugoptimized --cross-file util/darwin-amd64-apple.ini build-darwin-amd64 $(OPTS)
	meson compile -C build-darwin-amd64
	./util/mac-codesign.sh ./build-darwin-amd64/streamplace
	cd build-darwin-amd64 \
	&& tar -czvf ../bin/streamplace-$(VERSION)-darwin-amd64.tar.gz ./streamplace \
	&& cd -
	./build-darwin-amd64/streamplace --version
	./build-darwin-arm64/streamplace self-test
	$(MAKE) desktop-macos
	meson test -C build-darwin-arm64 go-tests

.PHONY: desktop-macos
desktop-macos:
	export DEBUG="electron-osx-sign*" \
	&& cd js/desktop \
	&& yarn run make --platform darwin --arch arm64 \
	&& yarn run make --platform darwin --arch x64 \
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

.PHONY: docker-build
docker-build: docker-build-builder docker-build-in-container

.PHONY: docker-build-builder
docker-build-builder:
	cd docker \
	&& docker build --target=builder --os=linux --arch=amd64 -f build.Dockerfile -t dist.stream.place/streamplace/streamplace:builder .

.PHONY: docker-build-builder
docker-build-in-container:
	docker run -v $$(pwd):$$(pwd) -w $$(pwd) --rm -it dist.stream.place/streamplace/streamplace:builder make app-and-node

.PHONY: docker-release
docker-release:
	cd docker \
	&& docker build -f release.Dockerfile \
	  --build-arg TARGETARCH=$(BUILDARCH) \
		-t dist.stream.place/streamplace/streamplace \
		.

.PHONY: ci-upload
ci-upload: ci-upload-node ci-upload-android

.PHONY: ci-upload-node
ci-upload-node: node-all-platforms
	for GOOS in linux; do \
		for GOARCH in amd64 arm64; do \
			export file=streamplace-$(VERSION)-$$GOOS-$$GOARCH.tar.gz \
			&& $(MAKE) ci-upload-file upload_file=$$file; \
			export file=streamplace-desktop-$(VERSION)-$$GOOS-$$GOARCH.AppImage \
			&& $(MAKE) ci-upload-file upload_file=$$file; \
		done \
	done;
	for GOOS in windows; do \
		for GOARCH in amd64; do \
			export file=streamplace-$(VERSION)-$$GOOS-$$GOARCH.zip \
			&& $(MAKE) ci-upload-file upload_file=$$file; \
			export file=streamplace-desktop-$(VERSION)-$$GOOS-$$GOARCH.exe \
			&& $(MAKE) ci-upload-file upload_file=$$file; \
			export SUM=$$(cat bin/streamplace-desktop-$(VERSION)-$$GOOS-$$GOARCH.nupkg.sha1) \
			&& export file=streamplace-desktop-$(VERSION)-$$GOOS-$$GOARCH.$$SUM.nupkg \
			&& $(MAKE) ci-upload-file upload_file=$$file; \
		done \
	done;

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
	$(MAKE) ci-upload-file upload_file=streamplace-$(VERSION)-android-release.apk \
	&& $(MAKE) ci-upload-file upload_file=streamplace-$(VERSION)-android-debug.apk \
	&& $(MAKE) ci-upload-file upload_file=streamplace-$(VERSION)-android-release.aab \
	&& $(MAKE) ci-upload-file upload_file=streamplace-$(VERSION)-android-debug.aab

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

.PHONY: release
release:
	yarn run release

.PHONY: ci-release
ci-release:
	go install gitlab.com/gitlab-org/release-cli/cmd/release-cli
	go run ./pkg/config/git/git.go -release -o release.yml
	release-cli create-from-file --file release.yml

.PHONY: check
check: install
	yarn run check
	if [ "`gofmt -l . | wc -l`" -gt 0 ]; then echo 'gofmt failed, run make fix'; exit 1; fi

.PHONY: fix
fix:
	yarn run fix
	gofmt -w .

.PHONY: precommit
precommit: dockerfile-hash-precommit

.PHONY: dockefile-hash-precommit
dockerfile-hash-precommit:
	@bash -c 'printf "variables:\n  DOCKERFILE_HASH: `git hash-object docker/build.Dockerfile`" > .ci/dockerfile-hash.yaml' \
	&& git add .ci/dockerfile-hash.yaml

.build/subprojects2.tar.gz:
	mkdir -p .build \
	&& curl -L https://storage.googleapis.com/aquareum-crap/subprojects2.tar.gz -o .build/subprojects2.tar.gz \
	&& tar -xzvf .build/subprojects2.tar.gz
