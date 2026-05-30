package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/google/uuid"
)

func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func IsWritable(path string) bool {
	tmp := filepath.Join(path, ".plomvix_write_check")
	f, err := os.Create(tmp)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(tmp)
	return true
}

func BytesToHuman(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	labels := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), labels[exp])
}

func GetGoVersion() string {
	return runtime.Version()
}

func GetOSArch() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}

func NewRequestID() string {
	return uuid.New().String()
}
