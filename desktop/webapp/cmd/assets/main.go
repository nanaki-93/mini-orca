// Command assets generates the native bundle metadata and icon from maintained
// project sources. Run it from the desktop/webapp module before packaging.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
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
	file, err := os.Create("build/appicon.png")
	if err != nil {
		return err
	}
	encodeErr := png.Encode(file, appIcon())
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}

func appIcon() image.Image {
	const size = 1024
	result := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)-512, float64(y)-512
			corner := math.Hypot(math.Max(math.Abs(dx)-292, 0), math.Max(math.Abs(dy)-292, 0))
			alpha := uint8(math.Max(0, math.Min(1, 212-corner)) * 255)
			pixel := color.NRGBA{R: 19, G: 24, B: 26, A: alpha}
			distance := math.Hypot(dx, dy)
			rotatedY := dx*0.5 + dy*math.Sqrt(3)/2
			if distance < 280 && (distance > 266 || rotatedY > 0) {
				pixel = color.NRGBA{R: 180, G: 229, B: 203, A: 255}
			}
			result.SetNRGBA(x, y, pixel)
		}
	}
	return result
}
