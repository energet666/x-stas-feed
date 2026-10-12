WEB_DIR := web
BUILD_DIR := build
RUN_ADDR := :8080
RUN_CONTENT_DIR := test-content
RUN_STATIC_DIR := $(WEB_DIR)/dist
PACKAGE_DIR := $(BUILD_DIR)/feed-ai-win64
PACKAGE_WEB_DIST := $(PACKAGE_DIR)/web/dist
PACKAGE_CONTENT_DIR := $(PACKAGE_DIR)/test-content
PACKAGE_ZIP := $(BUILD_DIR)/feed-ai-win64.zip
LITE_PACKAGE_NAME := feed-ai-win64-lite
LITE_PACKAGE_DIR := $(BUILD_DIR)/$(LITE_PACKAGE_NAME)
LITE_README_SRC := tools/package/windows-lite-README.txt
SERVER_WIN := $(PACKAGE_DIR)/server.exe
STICKER_PACK_SRC := test-content/.boards/sticker-pack
STICKER_PACK_DST := $(PACKAGE_CONTENT_DIR)/.boards/sticker-pack
FFMPEG_WIN_SRC := tools/ffmpeg/windows-amd64/ffmpeg.exe
FFMPEG_WIN_DST := $(PACKAGE_DIR)/tools/ffmpeg/windows-amd64/ffmpeg.exe
FFPROBE_WIN_SRC := tools/ffmpeg/windows-amd64/ffprobe.exe
FFPROBE_WIN_DST := $(PACKAGE_DIR)/tools/ffmpeg/windows-amd64/ffprobe.exe
FFMPEG_WIN_LICENSE_SRC := tools/ffmpeg/windows-amd64/LICENSE.txt
FFMPEG_WIN_LICENSE_DST := $(PACKAGE_DIR)/tools/ffmpeg/windows-amd64/LICENSE.txt

.PHONY: check docs-check web-build web-package run server-win sticker-pack-win ffmpeg-win package-win zip-win package-win-lite

check:
	go run ./tools/docscheck
	cd $(WEB_DIR) && npm run check
	go test ./...

docs-check:
	go run ./tools/docscheck

web-build:
	cd $(WEB_DIR) && npm run build

run: web-build
	go run ./cmd/server -addr $(RUN_ADDR) -content-dir $(RUN_CONTENT_DIR) -static-dir $(RUN_STATIC_DIR)

web-package:
	mkdir -p $(PACKAGE_WEB_DIST)
	cd $(WEB_DIR) && npm run build -- --outDir ../$(PACKAGE_WEB_DIST) --emptyOutDir

server-win:
	mkdir -p $(PACKAGE_DIR)
	GOOS=windows GOARCH=amd64 go build -o $(SERVER_WIN) ./cmd/server

sticker-pack-win:
	@rm -rf "$(STICKER_PACK_DST)"
	@if [ -d "$(STICKER_PACK_SRC)" ]; then \
		mkdir -p "$$(dirname "$(STICKER_PACK_DST)")"; \
		cp -R "$(STICKER_PACK_SRC)" "$(STICKER_PACK_DST)"; \
		find "$(STICKER_PACK_DST)" -name '.DS_Store' -delete; \
	else \
		echo "warning: $(STICKER_PACK_SRC) not found; package will not include starter board assets"; \
	fi

ffmpeg-win:
	@mkdir -p "$$(dirname "$(FFMPEG_WIN_DST)")"
	@if [ -f "$(FFMPEG_WIN_SRC)" ]; then \
		cp "$(FFMPEG_WIN_SRC)" "$(FFMPEG_WIN_DST)"; \
	else \
		rm -f "$(FFMPEG_WIN_DST)"; \
		echo "warning: $(FFMPEG_WIN_SRC) not found; package will rely on system ffmpeg"; \
	fi
	@if [ -f "$(FFPROBE_WIN_SRC)" ]; then \
		cp "$(FFPROBE_WIN_SRC)" "$(FFPROBE_WIN_DST)"; \
	else \
		rm -f "$(FFPROBE_WIN_DST)"; \
		echo "warning: $(FFPROBE_WIN_SRC) not found; package will rely on system ffprobe"; \
	fi
	@if [ -f "$(FFMPEG_WIN_LICENSE_SRC)" ]; then \
		cp "$(FFMPEG_WIN_LICENSE_SRC)" "$(FFMPEG_WIN_LICENSE_DST)"; \
	else \
		rm -f "$(FFMPEG_WIN_LICENSE_DST)"; \
	fi

zip-win:
	cd $(BUILD_DIR) && zip -r -FS feed-ai-win64.zip feed-ai-win64 -x '*/.DS_Store' '*.DS_Store'

package-win: web-package server-win sticker-pack-win ffmpeg-win
	$(MAKE) zip-win

# The lite package leaves FFmpeg out so the archive stays small; the server
# falls back to PATH or to binaries the user drops into the empty tools folder.
package-win-lite:
	$(MAKE) web-package server-win sticker-pack-win PACKAGE_DIR=$(LITE_PACKAGE_DIR)
	rm -rf "$(LITE_PACKAGE_DIR)/tools"
	mkdir -p "$(LITE_PACKAGE_DIR)/tools/ffmpeg/windows-amd64"
	cp "$(LITE_README_SRC)" "$(LITE_PACKAGE_DIR)/README.txt"
	cd $(BUILD_DIR) && zip -r -FS $(LITE_PACKAGE_NAME).zip $(LITE_PACKAGE_NAME) -x '*/.DS_Store' '*.DS_Store'
