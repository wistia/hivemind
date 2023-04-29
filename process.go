package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

type process struct {
	*exec.Cmd

	Name   string
	Color  int
	CanDie bool

	output *multiOutput
}

func newProcess(name, command string, color int, root string, port int, canDie bool, shell string, output *multiOutput) (proc *process) {
	proc = &process{
		exec.Command(fmt.Sprintf("/bin/%s", shell), "-c", command),
		name,
		color,
		canDie,
		output,
	}

	proc.Dir = root
	proc.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))

	proc.output.Connect(proc)

	return
}

func (p *process) writeLine(b []byte) {
	p.output.WriteLine(p, b)
}

func (p *process) writeErr(err error) {
	p.output.WriteErr(p, err)
}

func (p *process) signal(sig os.Signal) {
	group, err := os.FindProcess(-p.Process.Pid)
	if err != nil {
		p.writeErr(err)
		return
	}

	if err = group.Signal(sig); err != nil {
		p.writeErr(err)
	}
}

func (p *process) Running() bool {
	return p.Process != nil && p.ProcessState == nil
}

func (p *process) Run() int {
	p.output.PipeOutput(p)
	defer p.output.ClosePipe(p)

	ensureKill(p)

	p.writeLine([]byte("\033[1mRunning...\033[0m"))

	if err := p.Cmd.Run(); err != nil {
		p.writeErr(err)
		return err.(*exec.ExitError).ExitCode()
	} else {
		p.writeLine([]byte("\033[1mProcess exited\033[0m"))
		return 0
	}
}

func (p *process) Interrupt() {
	if p.Running() {
		p.writeLine([]byte("\033[1mInterrupting...\033[0m"))
		p.signal(syscall.SIGINT)
	}
}

func (p *process) Kill() {
	if p.Running() {
		p.writeLine([]byte("\033[1mKilling...\033[0m"))
		p.signal(syscall.SIGKILL)
	}
}
