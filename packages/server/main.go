package main

import (
	"fmt"
	"orinforum/cmd"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"
)

func init() {
	_, file, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(file)
		if err := os.Chdir(dir); err != nil {
			logrus.Warnf("failed to chdir to %s: %v", dir, err)
		}
	}
}

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
