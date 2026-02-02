APP_NAME := scouter.client
BUNDLE_NAME := ScouterQt.app
DIST_DIR := dist
QT_LIB_DIR := $(shell brew --prefix qt)/lib

# Qt6 requires C++17
export CGO_CXXFLAGS := -std=c++17
export CGO_LDFLAGS := -F/opt/homebrew/lib -framework QtCore -framework QtGui -framework QtWidgets

.PHONY: build bundle dist clean test lint fmt run

build:
	@mkdir -p $(DIST_DIR)
	CGO_CXXFLAGS="$(CGO_CXXFLAGS)" go build -o $(DIST_DIR)/$(APP_NAME) .

bundle: build
	@mkdir -p $(DIST_DIR)/$(BUNDLE_NAME)/Contents/MacOS
	@mkdir -p $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Resources
	@cp $(DIST_DIR)/$(APP_NAME) $(DIST_DIR)/$(BUNDLE_NAME)/Contents/MacOS/$(APP_NAME)
	@echo "Patching minimum macOS version to 15.0..."
	vtool -set-build-version macos 15.0 26.2 -replace -output $(DIST_DIR)/$(BUNDLE_NAME)/Contents/MacOS/$(APP_NAME) $(DIST_DIR)/$(BUNDLE_NAME)/Contents/MacOS/$(APP_NAME)
	@cp assets/AppIcon.icns $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Resources/
	@/usr/libexec/PlistBuddy -c "Clear dict" $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist 2>/dev/null || true
	@/usr/libexec/PlistBuddy \
		-c "Add :CFBundleExecutable string $(APP_NAME)" \
		-c "Add :CFBundleIdentifier string com.scouter.client.go" \
		-c "Add :CFBundleName string Scouter Client" \
		-c "Add :CFBundleDisplayName string Scouter Client" \
		-c "Add :CFBundleShortVersionString string 1.0.0" \
		-c "Add :CFBundleVersion string 1.0.0" \
		-c "Add :CFBundleIconFile string AppIcon" \
		-c "Add :CFBundlePackageType string APPL" \
		$(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@/usr/libexec/PlistBuddy \
		-c "Add :CFBundleInfoDictionaryVersion string 6.0" \
		-c "Add :LSMinimumSystemVersion string 15.0" \
		-c "Add :CFBundleSupportedPlatforms array" \
		-c "Add :CFBundleSupportedPlatforms:0 string MacOSX" \
		-c "Add :NSHighResolutionCapable bool true" \
		-c "Add :LSEnvironment dict" \
		-c "Add :LSEnvironment:GODEBUG string asyncpreemptoff=1" \
		$(DIST_DIR)/$(BUNDLE_NAME)/Contents/Info.plist
	@echo "Created $(DIST_DIR)/$(BUNDLE_NAME)"

BUNDLE_FRAMEWORKS := $(DIST_DIR)/$(BUNDLE_NAME)/Contents/Frameworks

# copy-qt-framework: copies a Qt framework into the bundle and fixes its install name
# Usage: $(call copy-qt-framework,QtDBus)
define copy-qt-framework
	@if [ ! -d $(BUNDLE_FRAMEWORKS)/$(1).framework ]; then \
		echo "Copying $(1).framework..."; \
		mkdir -p $(BUNDLE_FRAMEWORKS)/$(1).framework/Versions/A; \
		cp $(QT_LIB_DIR)/$(1).framework/Versions/A/$(1) $(BUNDLE_FRAMEWORKS)/$(1).framework/Versions/A/$(1); \
		ln -sf A $(BUNDLE_FRAMEWORKS)/$(1).framework/Versions/Current; \
		ln -sf Versions/Current/$(1) $(BUNDLE_FRAMEWORKS)/$(1).framework/$(1); \
		if [ -d $(QT_LIB_DIR)/$(1).framework/Versions/A/Resources ]; then \
			cp -R $(QT_LIB_DIR)/$(1).framework/Versions/A/Resources $(BUNDLE_FRAMEWORKS)/$(1).framework/Versions/A/; \
			ln -sf Versions/Current/Resources $(BUNDLE_FRAMEWORKS)/$(1).framework/Resources; \
		fi; \
		install_name_tool -id @rpath/$(1).framework/Versions/A/$(1) $(BUNDLE_FRAMEWORKS)/$(1).framework/Versions/A/$(1); \
	fi
endef

# fix-dylib-deps: rewrites Homebrew absolute paths in a dylib to use @rpath
# Usage: $(call fix-dylib-deps,path/to/lib.dylib)
define fix-dylib-deps
	@for dep in $$(otool -L $(1) | grep '/opt/homebrew' | awk '{print $$1}'); do \
		base=$$(basename "$$dep"); \
		install_name_tool -change "$$dep" @rpath/"$$base" $(1) 2>/dev/null || true; \
	done
endef

dist: bundle
	@echo "Running macdeployqt to bundle Qt frameworks..."
	macdeployqt $(DIST_DIR)/$(BUNDLE_NAME) -always-overwrite
	@echo "Copying missing Qt framework dependencies..."
	$(call copy-qt-framework,QtDBus)
	@# Copy libdbus-1 (QtDBus dependency)
	@if [ ! -f $(BUNDLE_FRAMEWORKS)/libdbus-1.3.dylib ]; then \
		echo "Copying libdbus-1.3.dylib..."; \
		cp $$(brew --prefix dbus)/lib/libdbus-1.3.dylib $(BUNDLE_FRAMEWORKS)/; \
		install_name_tool -id @rpath/libdbus-1.3.dylib $(BUNDLE_FRAMEWORKS)/libdbus-1.3.dylib; \
	fi
	@echo "Fixing library paths..."
	$(call fix-dylib-deps,$(BUNDLE_FRAMEWORKS)/QtDBus.framework/Versions/A/QtDBus)
	$(call fix-dylib-deps,$(BUNDLE_FRAMEWORKS)/libdbus-1.3.dylib)
	@echo "Adding rpath for plugin @rpath resolution..."
	install_name_tool -add_rpath @executable_path/../Frameworks $(DIST_DIR)/$(BUNDLE_NAME)/Contents/MacOS/$(APP_NAME)
	@echo "Ad-hoc code signing all bundled libraries..."
	@find $(BUNDLE_FRAMEWORKS) -type f \( -name "*.dylib" -o -name "Qt*" \) ! -name "*.plist" ! -name "*.xcprivacy" | while read lib; do \
		codesign --force --sign - "$$lib"; \
	done
	@find $(DIST_DIR)/$(BUNDLE_NAME)/Contents/PlugIns -type f -name "*.dylib" | while read lib; do \
		codesign --force --sign - "$$lib"; \
	done
	codesign --force --sign - $(DIST_DIR)/$(BUNDLE_NAME)
	@echo "Verifying code signature..."
	codesign -v --deep --strict $(DIST_DIR)/$(BUNDLE_NAME)
	@echo "Creating ZIP archive..."
	@cd $(DIST_DIR) && zip -r -y ScouterQt.zip $(BUNDLE_NAME)
	@echo "Created $(DIST_DIR)/ScouterQt.zip"

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