package theme

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestSanitizeAndTruncateVisibleWidth(t *testing.T) {
	if got := ClampWidth(40); got != 40 {
		t.Fatalf("expected ClampWidth(40)=40, got %d", got)
	}
	if got := ClampHeight(10); got != 10 {
		t.Fatalf("expected ClampHeight(10)=10, got %d", got)
	}
	code := SanitizeCodeText("a\tb\x1b[2Jc")
	if strings.Contains(code, "\x1b") || !strings.Contains(code, "\\x1b") {
		t.Fatalf("expected literal ESC to be escaped in code text, got %q", code)
	}
	padded := PadVisibleRight("\x1b[1mhello\x1b[0m", 10)
	if VisibleRuneCount(padded) != 10 {
		t.Fatalf("expected visible width 10, got %d", VisibleRuneCount(padded))
	}
}

func TestPalettesKeepTextReadableOnEverySurface(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		c := ColorsForPalette(palette)
		for _, bg := range []string{c.BgCode, c.SurfaceCode, c.SelBgCode} {
			for _, fg := range []string{c.FgCode, c.MutedCode, c.AccentCode, c.GreenCode, c.AmberCode, c.RedCode, c.PurpleCode, c.CyanCode} {
				assertContrast(t, palette, fg, bg)
			}
		}
		assertContrast(t, palette, c.OnAccentCode, c.AccentBgCode)
		assertContrast(t, palette, c.OnStatusCode, c.GreenCode)
		assertContrast(t, palette, c.OnStatusCode, c.AmberCode)
	}
}

func assertContrast(t *testing.T, palette Palette, fg, bg string) {
	t.Helper()
	a, b := paletteLuminance(t, fg), paletteLuminance(t, bg)
	ratio := (max(a, b) + 0.05) / (min(a, b) + 0.05)
	if ratio < 4.5 {
		t.Errorf("%s: foreground %s on background %s has %.2f:1 contrast, want at least 4.5:1", palette, fg, bg, ratio)
	}
}

func paletteLuminance(t *testing.T, code string) float64 {
	t.Helper()
	n, err := strconv.Atoi(code)
	if err != nil || n < 16 || n > 255 {
		t.Fatalf("expected an explicit ANSI 256 color, got %q", code)
	}
	var rgb [3]int
	if n >= 232 {
		rgb = [3]int{8 + (n-232)*10, 8 + (n-232)*10, 8 + (n-232)*10}
	} else {
		cube := []int{0, 95, 135, 175, 215, 255}
		rgb = [3]int{cube[(n-16)/36], cube[(n-16)/6%6], cube[(n-16)%6]}
	}
	var luminance float64
	for i, weight := range []float64{0.2126, 0.7152, 0.0722} {
		channel := float64(rgb[i]) / 255
		if channel <= 0.04045 {
			channel /= 12.92
		} else {
			channel = math.Pow((channel+0.055)/1.055, 2.4)
		}
		luminance += channel * weight
	}
	return luminance
}

func TestSurfaceRestoredAfterNestedStylesAndTruncation(t *testing.T) {
	bg, fg := "\x1b[48;5;235m", "\x1b[38;5;253m"
	for _, width := range []int{4, 20} {
		line := PaintLine(bg, fg, "\x1b[1mBold\x1b[0m label", width)
		if VisibleRuneCount(line) != width {
			t.Fatal("surface changed the terminal column count")
		}
		body := strings.TrimSuffix(line, "\x1b[0m")
		for _, segment := range strings.Split(body, "\x1b[0m") {
			if !strings.HasPrefix(segment, bg+fg) {
				t.Fatal("a nested style reset lost the surface background")
			}
		}
	}
}
