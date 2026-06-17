package filesystem

import "os"

type Probe struct{}

func (Probe) Ready(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
