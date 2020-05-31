package internal

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

type Script interface {
	Dir() string
	Env() []string
	Name() string
	Run() string
	Shell() string
	Prefix() string
}

type run struct {
	cmd       *exec.Cmd
	script    Script
	pid       int
	startLock bool
	index     int
	ctx       context.Context
}

type runGroup struct {
	runs []*run
	done chan bool
	ctx  context.Context
}

func NewRunGroup(ctx context.Context, scripts []Script) *runGroup {
	runs := make([]*run, len(scripts))
	for i, s := range scripts {
		runs[i] = &run{
			index:  i,
			ctx:    ctx,
			cmd:    nil,
			pid:    -1,
			script: s,
		}
	}

	return &runGroup{
		ctx:  ctx,
		runs: runs,
		done: nil,
	}
}

func (rg *runGroup) Start() {
	for _, r := range rg.runs {
		// Something else already starting it
		if r.startLock {
			continue
		}

		// Kill it if it's already running
		if r.cmd != nil && r.cmd.ProcessState == nil {
			_ = r.cmd.Process.Kill()
		}

		// Make a new command
		r.cmd = mkCmd(rg.ctx, r.script, r.index)

		r.startLock = true

		go func(r *run) {
			err := r.cmd.Start()
			if err != nil {
				panic(err)
			}
			r.startLock = false

			r.pid = r.cmd.Process.Pid
			_ = r.cmd.Wait()
			allDone := true
			for _, r := range rg.runs {
				if r.cmd.ProcessState == nil {
					allDone = false
					break
				}
			}

			if allDone {
				rg.done <- true
			}
		}(r)
	}

	rg.done = make(chan bool)
}

func (rg *runGroup) Restart() {
	for _, r := range rg.runs {
		exited := r.cmd.ProcessState != nil
		if exited {
			_ = r.cmd.Process.Kill()
		}
	}

	rg.Start()
}

func (rg *runGroup) Wait() error {
	if rg.done == nil {
		return errors.New("run group has not been started")
	}

	<-rg.done
	close(rg.done)

	return nil
}

func mkCmd(ctx context.Context, s Script, index int) *exec.Cmd {
	shell := DefaultStr(s.Shell(), os.Getenv("SHELL"), "bash")

	cmd := exec.CommandContext(ctx, shell, "-c", s.Run())

	cmd.Env = append(os.Environ(), s.Env()...)
	cmd.Dir = s.Dir()

	prefix := DefaultStr(s.Prefix(), s.Name())
	cmd.Stdout = newPrefixedWriter(os.Stdout, prefix, GetColor(index))
	cmd.Stderr = newPrefixedWriter(os.Stderr, prefix, GetColor(index))
	cmd.Stdin = os.Stdin

	return cmd
}
