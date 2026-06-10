WEB_DIR := web
BUILD_DIR := build
PACKAGE_DIR := $(BUILD_DIR)/feed-ai-win64
PACKAGE_WEB_DIST := $(PACKAGE_DIR)/web/dist
PACKAGE_CONTENT_DIR := $(PACKAGE_DIR)/test-content
PACKAGE_ZIP := $(BUILD_DIR)/feed-ai-win64.zip
SERVER_WIN := $(PACKAGE_DIR)/server.exe
STICKER_PACK_SRC := test-content/.boards/sticker-pack
STICKER_PACK_DST := $(PACKAGE_CONTENT_DIR)/.boards/sticker-pack
FFMPEG_WIN_SRC := tools/ffmpeg/windows-amd64/ffmpeg.exe
FFMPEG_WIN_DST := $(PACKAGE_DIR)/tools/ffmpeg/windows-amd64/ffmpeg.exe
FFPROBE_WIN_SRC := tools/ffmpeg/windows-amd64/ffprobe.exe
FFPROBE_WIN_DST := $(PACKAGE_DIR)/tools/ffmpeg/windows-amd64/ffprobe.exe
FFMPEG_WIN_LICENSE_SRC := tools/ffmpeg/windows-amd64/LICENSE.txt
FFMPEG_WIN_LICENSE_DST := $(PACKAGE_DIR)/tools/ffmpeg/windows-amd64/LICENSE.txt

.PHONY: check web-build web-package server-win sticker-pack-win ffmpeg-win package-win zip-win

check:
	cd $(WEB_DIR) && npm run check
	go test ./...

web-build:
	cd $(WEB_DIR) && npm run build

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
