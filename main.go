package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
)

func main() {
	log, err := zap.NewProduction()
	if err != nil {
		panic("failed to create logger:" + err.Error())
	}
	cmd, done, err := NewCommand(context.Background(), log, `bash`, `-c`, `./input.sh`)
	if err != nil {
		log.Error("failed to create command", zap.Error(err))
	}

	reader := bufio.NewScanner(cmd)
	if diff := diffLine(reader, "Enter your name:"); diff != "" {
		log.Fatal("unexpected name prompt", zap.String("diff", diff))
	}
	_, err = io.WriteString(cmd, "Adrian\n")
	if err != nil {
		log.Fatal("failed to write name", zap.Error(err))
	}
	if diff := diffLine(reader, "Welcome Adrian!"); diff != "" {
		log.Fatal("unexpected name output", zap.String("diff", diff))
	}
	if diff := diffLine(reader, "Are you happy? Y/N"); diff != "" {
		log.Fatal("unexpected happy prompt", zap.String("diff", diff))
	}
	_, err = io.WriteString(cmd, "N\n")
	if err != nil {
		log.Fatal("failed to input happiness", zap.Error(err))
	}
	// Check the exit was OK.
	err = <-done
	if err != nil {
		log.Error("command error", zap.Error(err))
	}
}

func diffLine(r *bufio.Scanner, want string) (diff string) {
	read := r.Scan()
	if !read {
		return "expected to read line, but didn't"
	}
	return cmp.Diff(want, strings.TrimSpace(r.Text()))
}

func NewCommand(ctx context.Context, log *zap.Logger, cmd string, args ...string) (rwc processReadWriteCloser, done chan error, err error) {
	_, err = exec.LookPath(cmd)
	if errors.Is(err, exec.ErrNotFound) {
		err = fmt.Errorf("cannot find %q on the path (%q)", cmd, os.Getenv("PATH"))
		return
	}
	if err != nil {
		return
	}
	return newProcessReadWriteCloser(log, exec.Command(cmd, args...))
}

// newProcessReadWriteCloser creates a processReadWriteCloser to allow stdin/stdout to be used as reader/writer.
func newProcessReadWriteCloser(zapLogger *zap.Logger, cmd *exec.Cmd) (rwc processReadWriteCloser, done chan error, err error) {
	done = make(chan error, 1)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	rwc = processReadWriteCloser{
		in:  stdin,
		out: stdout,
	}
	go func() {
		done <- cmd.Run()
	}()
	return
}

type processReadWriteCloser struct {
	in  io.WriteCloser
	out io.ReadCloser
}

func (prwc processReadWriteCloser) Read(p []byte) (n int, err error) {
	return prwc.out.Read(p)
}

func (prwc processReadWriteCloser) Write(p []byte) (n int, err error) {
	return prwc.in.Write(p)
}

func (prwc processReadWriteCloser) Close() error {
	errInClose := prwc.in.Close()
	errOutClose := prwc.out.Close()
	if errInClose != nil || errOutClose != nil {
		return fmt.Errorf("error closing process - in: %v, out: %v", errInClose, errOutClose)
	}
	return nil
}
