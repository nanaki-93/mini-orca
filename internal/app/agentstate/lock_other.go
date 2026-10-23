//go:build !unix && !windows

package agentstate

import (
	"fmt"
	"os"
)

func tryLock(*os.File) (bool, error) {
	return false, fmt.Errorf("durable agent journals require process locking on this platform")
}
