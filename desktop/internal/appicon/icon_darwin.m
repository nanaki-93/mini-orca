#import <Cocoa/Cocoa.h>
#import "icon_darwin.h"

int miniOrcaSetIcon(const void *bytes, size_t length) {
    @autoreleasepool {
        NSData *data = [NSData dataWithBytes:bytes length:length];
        NSImage *image = [[NSImage alloc] initWithData:data];
        if (image == nil || !image.valid) {
            [image release];
            return 0;
        }
        dispatch_async(dispatch_get_main_queue(), ^{
            [NSApp setApplicationIconImage:image];
            [image release];
        });
        return 1;
    }
}
