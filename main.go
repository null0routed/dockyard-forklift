package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

var (
	FORKLIFT_CommandNotRecognized = "Error: Command %s not recognized.\n%s\n"
	FORKLIFT_UsageString          = "Usage: forklift <cmd> [-b | -v] <image path> <container exec>"
	FORKLIFT_RunNoArgs            = "Error: No arguments specific for the 'run' command.\n%s\n"
)

type runFlagStruct struct {
	backgroundFlag *bool
	args           []string
}

var debugFlag *bool

func main() {

	runCtx := runFlagStruct{
		backgroundFlag: nil,
	}

	runFlagSet := flag.NewFlagSet("run", flag.ExitOnError)
	runCtx.backgroundFlag = runFlagSet.Bool("b", false, "Spawn the container as a background process.")
	debugFlag = runFlagSet.Bool("v", false, "Print verbose debug information.")

	if len(os.Args) < 2 {
		fmt.Printf(FORKLIFT_CommandNotRecognized, os.Args[1], FORKLIFT_UsageString)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runFlagSet.Parse(os.Args[2:])
		runCtx.args = runFlagSet.Args()
		if len(runCtx.args) < 1 {
			fmt.Printf(FORKLIFT_RunNoArgs, FORKLIFT_UsageString)
		}
		runCmd(runCtx)
	case "child":
		runFlagSet.Parse(os.Args[2:])
		runCtx.args = runFlagSet.Args()
		if len(runCtx.args) < 1 {
			fmt.Printf(FORKLIFT_RunNoArgs, FORKLIFT_UsageString)
		}
		childCmd(runCtx)
	default:
		fmt.Printf(FORKLIFT_CommandNotRecognized, os.Args[1], FORKLIFT_UsageString)
	}
}

func runCmd(r runFlagStruct) {
	printDebug(r.args)

	execCommand := exec.Command("/proc/self/exe", append([]string{"child"}, r.args[2:]...)...)
	execCommand.Stdin = os.Stdin
	execCommand.Stdout = os.Stdout
	execCommand.Stderr = os.Stderr
	execCommand.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWIPC,
	}

	must(execCommand.Run())
}

func childCmd(r runFlagStruct) {
	printDebug(r.args)

	// Establish clean root to bind-mount on

	execCommand := exec.Command(r.args[1], r.args[2:]...)
	execCommand.Stdin = os.Stdin
	execCommand.Stdout = os.Stdout
	execCommand.Stderr = os.Stderr

	must(execCommand.Run())
}

func setupHostFileSystem() {

}

func printDebug(a any) {
	if *debugFlag {
		fmt.Fprintln(os.Stderr, a)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
