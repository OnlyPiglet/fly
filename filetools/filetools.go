package filetools

import (
	"fmt"
	"os"
	"sort"
	"time"
)

type FileInfo struct {
	ModTime  time.Time
	FileName string
}

func RecentFileMaxKept(dir string, maxKept int) error {

	rd, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	all := make([]FileInfo, 0)

	for _, entry := range rd {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		all = append(all, FileInfo{
			ModTime:  info.ModTime(),
			FileName: info.Name(),
		})
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].ModTime.After(all[j].ModTime)
	})

	for i, fi := range all {
		if i < maxKept {
			continue
		}
		fn := fmt.Sprintf("%s%s%s", dir, string(os.PathSeparator), fi.FileName)
		err := os.Remove(fn)
		if err != nil {
			return err
		}
	}

	return nil
}
