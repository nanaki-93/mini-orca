//go:build darwin

package appicon

/*
#cgo LDFLAGS: -framework Cocoa
#include "icon_darwin.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// Set queues the icon on AppKit's main thread after the host has started.
// Explicit assignment avoids stale Launch Services icons during local rebuilds.
func Set(data []byte) error {
	if len(data) == 0 || C.miniOrcaSetIcon(unsafe.Pointer(&data[0]), C.size_t(len(data))) == 0 {
		return fmt.Errorf("decode desktop application icon")
	}
	return nil
}
