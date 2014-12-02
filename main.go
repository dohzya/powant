package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func die(msg string) {
	die2(msg, 1)
}

func die2(msg string, status int) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(status)
}

func prepareCmd(cmd ...string) *exec.Cmd {
	command := exec.Command(cmd[0], cmd[1:]...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	return command
}

func prepareTrigger(cmd string) *exec.Cmd {
	if cmd == "noop" {
		return nil
	}
	command := exec.Command("sh", "-c", cmd)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command
}

func runTrigger(trigger *exec.Cmd, name string) {
	if trigger != nil {
		if *debug {
			fmt.Printf("[powant] %v\n", trigger)
		}
		if err := trigger.Run(); err != nil {
			die2(fmt.Sprintf("[powant] Can't run the %v trigger: %v", name, err), 127)
		}
	}
}

var verbose *bool = flag.Bool("v", false, "Verbose")
var debug *bool = flag.Bool("d", false, "Debug")

func main() {
	tBefore := flag.String("b", "noop", "A trigger to call before running the command")
	tAfter := flag.String("a", "noop", "A trigger to call before running the command")
	flag.Parse()

	if *debug {
		*verbose = true
	}

	if len(flag.Args()) == 0 {
		die("[powant] Missing command")
	}

	cBefore := prepareTrigger(*tBefore)
	cAfter := prepareTrigger(*tAfter)

	runTrigger(cBefore, "BEFORE")

	command := prepareCmd(flag.Args()...)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		s := <-sigc
		if *verbose {
			fmt.Printf("[powant] Received signal %v\n", s)
		}
		command.Process.Signal(s)
	}()

	if *debug {
		fmt.Printf("[powant] %v\n", command)
	}
	if err := command.Start(); err != nil {
		die2(fmt.Sprintf("[powant] Can't start the process: %v", err), 127)
	}

	cmdErr := command.Wait()

	status := 0
	if cmdErr != nil {
		if command.ProcessState == nil {
			die2(fmt.Sprintf("[powant] Error occured: %v", cmdErr), 127)
		}
		status = command.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
	}

	runTrigger(cAfter, "AFTER")

	os.Exit(status)

}
