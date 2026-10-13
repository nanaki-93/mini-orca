//go:build darwin

package appicon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSetRejectsMissingOrInvalidImage(t *testing.T) {
	for _, data := range [][]byte{nil, {}, []byte("not an image")} {
		if err := Set(data); err == nil {
			t.Fatal("invalid icon was accepted")
		}
	}
}

func TestIconReplacesApplicationImageOnMainQueue(t *testing.T) {
	// AppKit owns the process main thread. Use an isolated, windowless probe so
	// the Go test runner does not need to take over the application event loop.
	dir := t.TempDir()
	source := filepath.Join(dir, "probe.m")
	program := `#import <Cocoa/Cocoa.h>
#import "icon_darwin.h"
NSData *pixels(NSImage *image) {
    CGColorSpaceRef space = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
    CGContextRef canvas = CGBitmapContextCreate(NULL, 1024, 1024, 8, 4096, space,
        kCGImageAlphaPremultipliedLast | kCGBitmapByteOrder32Big);
    CGContextClearRect(canvas, CGRectMake(0, 0, 1024, 1024));
    CGContextDrawImage(canvas, CGRectMake(0, 0, 1024, 1024),
        [image CGImageForProposedRect:NULL context:nil hints:nil]);
    NSData *data = [NSData dataWithBytes:CGBitmapContextGetData(canvas) length:4194304];
    CGContextRelease(canvas);
    CGColorSpaceRelease(space);
    return data;
}
int main(int argc, char **argv) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];
        NSData *data = [NSData dataWithContentsOfFile:[NSString stringWithUTF8String:argv[1]]];
        NSImage *expected = [[NSImage alloc] initWithData:data];
        if (!expected.valid) return 2;
        // AppKit snapshots application icons at the display's backing scale.
        // Rasterize the reference with AppKit too, so Retina interpolation does
        // not make identical source images look like different pixel data.
        NSImage *reference = [[NSImage alloc] initWithSize:expected.size];
        [reference lockFocus];
        [expected drawInRect:NSMakeRect(0, 0, expected.size.width, expected.size.height)
            fromRect:NSZeroRect operation:NSCompositingOperationCopy fraction:1];
        [reference unlockFocus];
        [NSApp setApplicationIconImage:[[NSImage alloc] initWithSize:NSMakeSize(16, 16)]];
        if (!miniOrcaSetIcon(data.bytes, data.length)) return 3;
        dispatch_async(dispatch_get_main_queue(), ^{
            BOOL matches = NSEqualSizes(NSApp.applicationIconImage.size, expected.size)
                && [pixels(NSApp.applicationIconImage) isEqualToData:pixels(reference)];
            exit(matches ? 0 : 4);
        });
        dispatch_main();
    }
}
`
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "probe")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	compile := exec.CommandContext(ctx, "clang", "-framework", "Cocoa", "-I.", "icon_darwin.m", source, "-o", binary)
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compile icon probe: %v\n%s", err, output)
	}
	probe := exec.CommandContext(ctx, binary, "../../../../docs/icons/graphite/app-icon/app-icon-1024.png")
	if output, err := probe.CombinedOutput(); err != nil {
		t.Fatalf("apply bundled icon on AppKit main queue: %v\n%s", err, output)
	}
}
