//go:build linux
// +build linux

package erofs

import (
	"os"
	"syscall"

	"github.com/containerd/continuity/fs"
)

func getSupportsDType(dir string) (bool, error) {
	return fs.SupportsDType(dir)
}

func lchown(target string, st os.FileInfo) error {
	stat := st.Sys().(*syscall.Stat_t)
	return os.Lchown(target, int(stat.Uid), int(stat.Gid))
}
