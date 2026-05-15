package vm

import (
	"fmt"
	"log"
	"os"
	"time"
)

type CmdHistWriter struct {
	FileName string
	File     *os.File
}

func NewCmdHistWriter() *CmdHistWriter {
	filenameWithTimestamp := fmt.Sprintf(".hist-synacor-%d", time.Now().Unix())
	file, err := os.Create(filenameWithTimestamp)
	if err != nil {
		log.Fatal("Cannot create history file for current session. Exiting")
	}

	return &CmdHistWriter{
		FileName: filenameWithTimestamp,
		File:     file,
	}
}

func (c *CmdHistWriter) AppendToFile(line []byte) {
	_, err := c.File.Write(line)
	if err != nil {
		log.Fatal("Could not write to the file")
	}
}
