package main

// @title           Feed Chamber API
// @version         1.0
// @description     Feed Chamber 社区论坛 API 文档
// @host            localhost:8080
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter "Bearer {token}" to authenticate

//go:generate swag init -g main.go -o docs --parseInternal

import (
	"fmt"
	"feedchamber/cmd"
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
