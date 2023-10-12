package conf

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var (
	appPath     string
	appPathOnce sync.Once
)

// AppPath returns the absolute path of the application's binary.
func AppPath() string {
	appPathOnce.Do(func() {
		var err error
		appPath, err = exec.LookPath(os.Args[0])
		if err != nil {
			panic("look executable path: " + err.Error())
		}

		appPath, err = filepath.Abs(appPath)
		if err != nil {
			panic("get absolute executable path: " + err.Error())
		}
	})

	return appPath
}

var (
	customDir     string
	customDirOnce sync.Once
)

func CustomDir() string {
	customDirOnce.Do(func() {
		customDir = os.Getenv("GOGS_CUSTOM")
		if customDir != "" {
			return
		}

		customDir = filepath.Join(WorkDir(), "custom")
	})

	return customDir
}

func CurDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	wd = filepath.Join(wd, "app.ini")
	return wd, nil
}

var (
	workDir     string
	workDirOnce sync.Once
)

// WorkDir returns the absolute path of work directory. It reads the value of environment
// variable GOGS_WORK_DIR. When not set, it uses the directory where the application's
// binary is located.
func WorkDir() string {
	workDirOnce.Do(func() {
		workDir = os.Getenv("GOGS_WORK_DIR")
		if workDir != "" {
			return
		}

		workDir = filepath.Dir(AppPath())
	})

	return workDir
}
