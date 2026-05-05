WEB_DIR := web
BUILD_DIR := build
PACKAGE_DIR := $(BUILD_DIR)/feed-ai-win64
PACKAGE_WEB_DIST := $(PACKAGE_DIR)/web/dist
PACKAGE_ZIP := $(BUILD_DIR)/feed-ai-win64.zip
SERVER_WIN := $(PACKAGE_DIR)/server.exe

.PHONY: check web-build web-package server-win package-win zip-win

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

zip-win:
	cd $(BUILD_DIR) && zip -r -FS feed-ai-win64.zip feed-ai-win64

package-win: web-package server-win zip-win
