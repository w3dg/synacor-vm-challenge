package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"synacor.com/challenge/vm"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ch <ch_binary>")
		os.Exit(1)
	}

	_, args := os.Args[0], os.Args[1:]
	chBinary := args[0]

	bindata, err := os.ReadFile(chBinary)
	if err != nil {
		log.Fatal("could not open file: ", chBinary)
	}

	// Setup Logging

	// Read log level from environment variable
	level := os.Getenv("LOG_LEVEL")

	var logLevel slog.Level
	switch strings.ToUpper(level) {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "INFO":
		logLevel = slog.LevelInfo
	case "WARN":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	removeTime := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey && len(groups) == 0 {
			return slog.Attr{}
		}
		return a
	}

	// Create handler with the specified level and time removed
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:       logLevel,
		ReplaceAttr: removeTime,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)

	gamevm := vm.New()

	var memi, i vm.Value = 0, 0
	for int(i) < len(bindata) {
		bv := []byte{bindata[i], bindata[i+1]}
		gamevm.SetMemory(memi, bv)
		i += 2
		memi += 1
	}

	for gamevm.Step() {
		gamevm.DumpState()
	}
}
