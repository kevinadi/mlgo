//go:build windows

package daemon

import (
	"fmt"
	"time"
)

// StartAndWait is unused on Windows. mlgo starts servers there with start /b.
func StartAndWait(argv []string, port int, timeout time.Duration) (int, error) {
	return 0, fmt.Errorf("detached start is not used on windows")
}
