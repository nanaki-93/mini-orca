// Command assets generates the native bundle metadata and icon from maintained
// project sources. Run it from the desktop/webapp module before packaging.
package main

import (
	"fmt"
	"os"
	"regexp"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() error {
	source, err := os.ReadFile("../../internal/version/version.go")
	if err != nil {
		return err
	}
	version := regexp.MustCompile(`Version = "([0-9]+\.[0-9]+\.[0-9]+)"`).FindSubmatch(source)
	if len(version) != 2 {
		return fmt.Errorf("canonical release version was not found")
	}
	if err := os.MkdirAll("build/darwin", 0755); err != nil {
		return err
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleName</key><string>Mini-Orca</string>
<key>CFBundleExecutable</key><string>mini-orca-desktop</string>
<key>CFBundleIdentifier</key><string>io.miniorca.webdesktop</string>
<key>CFBundleVersion</key><string>%s</string>
<key>CFBundleShortVersionString</key><string>%s</string>
<key>CFBundleIconFile</key><string>iconfile</string>
<key>LSMinimumSystemVersion</key><string>13.0</string>
<key>NSHighResolutionCapable</key><true/>
<key>NSHumanReadableCopyright</key><string>Mini-Orca contributors</string>
</dict></plist>
`, version[1], version[1])
	if err := os.WriteFile("build/darwin/Info.plist", []byte(plist), 0644); err != nil {
		return err
	}
	icon, err := os.ReadFile("../../docs/icons/graphite/app-icon/app-icon-1024.png")
	if err != nil {
		return fmt.Errorf("read desktop app icon: %w", err)
	}
	if err := os.WriteFile("build/appicon.png", icon, 0644); err != nil {
		return fmt.Errorf("write desktop app icon: %w", err)
	}
	return nil
}
