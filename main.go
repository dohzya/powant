package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/dohzya/choose-port/chooseport"
)

func die(msg string) {
	die2(msg, 1)
}

func die2(msg string, status int) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(status)
}

func prepareCmd(cmd []string, mapping func(string) string) *exec.Cmd {
	args := make([]string, len(cmd)-1)
	for i, v := range cmd[1:] {
		args[i] = os.Expand(v, mapping)
	}
	command := exec.Command(cmd[0], args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	return command
}

func prepareTrigger(cmd *string) *exec.Cmd {
	if cmd == nil {
		return nil
	}
	command := exec.Command("sh", "-c", *cmd)
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
	// helpers
	powName := flag.String("pow", "", "The pow's name of the app")
	port := flag.Int("port", 0, "The port (for pow-based apps)")
	flag.Parse()

	if len(flag.Args()) == 0 {
		die("[powant] Missing command")
	}

	if *debug {
		*verbose = true
	}
	if *tBefore == "noop" {
		tBefore = nil
	}
	if *tAfter == "noop" {
		tAfter = nil
	}
	if *powName == "" {
		powName = nil
	}

	env := make(map[string]string)

	if powName != nil {

		if *port == 0 {
			env := os.Getenv("POW_PORT")
			if env != "" {
				p64, err := strconv.ParseInt(env, 0, 0)
				if err != nil {
					die2(fmt.Sprintf("[powant] Can't parse port: %v", err), 127)
				}
				p := int(p64)
				port = &p
			} else {
				p := chooseport.Random()
				port = &p
			}
		}

		env["POW_PORT"] = fmt.Sprintf("%d", *port)
		os.Setenv("POW_PORT", env["POW_PORT"])

		if tBefore == nil {
			cmd := fmt.Sprintf("echo %d > '%s/.pow/%s'", *port, os.Getenv("HOME"), *powName)
			tBefore = &cmd
		}

		if tAfter == nil {
			cmd := fmt.Sprintf("rm '%s/.pow/%s'", os.Getenv("HOME"), *powName)
			tAfter = &cmd
		}

	}

	cBefore := prepareTrigger(tBefore)
	cAfter := prepareTrigger(tAfter)

	runTrigger(cBefore, "BEFORE")

	if *debug {
		fmt.Printf("[powant] env = %v\n", env)
	}
	command := prepareCmd(flag.Args(), func(key string) string {
		return env[key]
	})

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
