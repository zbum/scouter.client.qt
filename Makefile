APP_NAME := scouter.client
BUNDLE_NAME := ScouterQt.app
DIST_DIR := dist

# Qt6 requires C++17
export CGO_CXXFLAGS := -std=c++17
export CGO_LDFLAGS := -F/opt/homebrew/lib -framework QtCore -framework QtGui -framework QtWidgets

.PHONY: build bundle clean test lint fmt run

build:
	@mkdir -p $(DIST_DIR)
	CGO_CXXFLAGS="$(CGO_CXXFLAGS)" go build -o $(DIST_DIR)/$(APP_NAME) .

bundle: build
	@mkdir -p $(DIST_DIR)/$(BUNDLE_NAME)/Contents/MacOS
	@mkdir -p $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Resources
	@cp $(DIST_DIR)/$(APP_NAME) $(DIST_DIR)/$(BUNDLE_NAME)/Contents/MacOS/$(APP_NAME)-bin
	@echo '#!/bin/bash' > $(DIST_DIR)/$(BUNDLE_NAME)/Contents/MacOS/$(APP_NAME)
	@echo 'DIR="$$(cd "$$(dirname "$$0")" && pwd)"' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/MacOS/$(APP_NAME)
	@echo 'export GODEBUG=asyncpreemptoff=1' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/MacOS/$(APP_NAME)
	@echo 'exec "$$DIR/$(APP_NAME)-bin" "$$@"' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/MacOS/$(APP_NAME)
	@chmod +x $(DIST_DIR)/$(BUNDLE_NAME)/Contents/MacOS/$(APP_NAME)
	@cp assets/AppIcon.icns $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Resources/
	@echo '<?xml version="1.0" encoding="UTF-8"?>' > $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '<plist version="1.0">' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '<dict>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <key>CFBundleExecutable</key>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <string>$(APP_NAME)</string>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <key>CFBundleIdentifier</key>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <string>com.miqtexample.miqtapp</string>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <key>CFBundleName</key>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <string>MIQT App</string>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <key>CFBundleIconFile</key>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <string>AppIcon</string>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <key>CFBundlePackageType</key>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <string>APPL</string>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <key>CFBundleVersion</key>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <string>1.0</string>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <key>LSMinimumSystemVersion</key>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <string>10.15</string>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <key>NSHighResolutionCapable</key>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '    <true/>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '</dict>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo '</plist>' >> $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo "Created $(DIST_DIR)/$(BUNDLE_NAME)"

clean:
	rm -rf $(DIST_DIR)
	go clean

test:
	go test -v ./...

lint:
	golangci-lint run

fmt:
	go fmt ./...

run: bundle
	GODEBUG=asyncpreemptoff=1 $(DIST_DIR)/$(BUNDLE_NAME)/Contents/MacOS/$(APP_NAME)