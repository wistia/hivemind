package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var colors = []int{2, 3, 4, 5, 6, 42, 130, 103, 129, 108}
var defaultProcShell = "sh"

type hivemindConfig struct {
	Title              string
	Procfile           string
	ProcNames          string
	Root               string
	PortBase, PortStep int
	Timeout            int
	NoPrefix           bool
	PrintTimestamps    bool
	CanDieProcNames    string
	ProcShell          string
}

type hivemind struct {
	title       string
	output      *multiOutput
	procs       []*process
	procWg      sync.WaitGroup
	done        chan bool
	interrupted chan os.Signal
	exitCodeCh  chan int
	timeout     time.Duration
	exitCode    int
}

func newHivemind(conf hivemindConfig) (h *hivemind) {
	h = &hivemind{timeout: time.Duration(conf.Timeout) * time.Second}

	if len(conf.Title) > 0 {
		h.title = conf.Title
	} else {
		h.title = filepath.Base(conf.Root)
	}

	h.output = &multiOutput{printProcName: !conf.NoPrefix, printTimestamp: conf.PrintTimestamps}

	entries := parseProcfile(conf.Procfile, conf.PortBase, conf.PortStep)
	h.procs = make([]*process, 0)
	h.exitCodeCh = make(chan int, len(conf.ProcNames))

	procNames := splitAndTrim(conf.ProcNames)
	canDieProcMap := parseCanDieProcNames(conf.CanDieProcNames)

	procShell := defaultProcShell
	if conf.ProcShell != "" {
		procShell = conf.ProcShell
	}

	for i, entry := range entries {
		if len(procNames) == 0 || stringsContain(procNames, entry.Name) {
			h.procs = append(h.procs, newProcess(entry.Name, entry.Command, colors[i%len(colors)], conf.Root, entry.Port, canDieProcMap[entry.Name], procShell, h.output))
		}
	}

	return
}

func (h *hivemind) runProcess(proc *process) {
	h.procWg.Add(1)

	go func() {
		defer h.procWg.Done()
		if !proc.CanDie {
			defer func() { h.done <- true }()
		}

		h.exitCodeCh <- proc.Run()
	}()
}

func (h *hivemind) waitForDoneOrInterrupt() {
	select {
	case <-h.done:
	case <-h.interrupted:
	}
}

func (h *hivemind) waitForTimeoutOrInterrupt() {
	select {
	case <-time.After(h.timeout):
	case <-h.interrupted:
	}
}

func (h *hivemind) waitForExit() {
	h.waitForDoneOrInterrupt()

	for _, proc := range h.procs {
		go proc.Interrupt()
	}

	h.waitForTimeoutOrInterrupt()

	for _, proc := range h.procs {
		go proc.Kill()
	}
}

func (h *hivemind) processExitCodes() {
	for {
		select {
		case exitCode := <-h.exitCodeCh:
			if exitCode > h.exitCode {
				h.exitCode = exitCode
			}
		case <-h.interrupted:
		}
	}
}

func (h *hivemind) Run() {
	fmt.Printf("\033]0;%s | hivemind\007", h.title)

	h.done = make(chan bool, len(h.procs))

	h.interrupted = make(chan os.Signal)
	signal.Notify(h.interrupted, syscall.SIGINT, syscall.SIGTERM)

	for _, proc := range h.procs {
		h.runProcess(proc)
	}

	go h.processExitCodes()
	go h.waitForExit()

	h.procWg.Wait()
	os.Exit(h.exitCode)
}

func parseCanDieProcNames(str string) map[string]bool {
	canDieProcNames := splitAndTrim(str)
	canDieProcMap := map[string]bool{}
	for _, procName := range canDieProcNames {
		canDieProcMap[procName] = true
	}
	return canDieProcMap
}
