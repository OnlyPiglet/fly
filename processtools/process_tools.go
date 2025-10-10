package processtools

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

// SoleProcess 判断启动单个程序实例，不允许重复,如需释放可以 向 chan 中释放一个信号，或者 close chan
func SoleProcess(lockName string) (error, chan struct{}) {
	lockFile := filepath.Join(os.TempDir(), lockName)

	slog.Info(lockFile)

	fileLock := flock.New(lockFile)

	locked, err := fileLock.TryLock()
	if err != nil {
		return err, nil
	}

	if !locked {
		return fmt.Errorf("cannot acquire lock for %s,another process has held lock", lockName), nil
	}
	stop := make(chan struct{}, 1)

	go func() {
		select {
		case <-stop:
			fileLock.Unlock()
		}
	}()
	//maybe no need to unlock file
	return nil, stop
}
