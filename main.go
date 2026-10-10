package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

var (
	FORKLIFT_CommandNotRecognized = "Error: Command %s not recognized.\n%s\n"
	FORKLIFT_CommandNotSpecified  = "Error: Command not specified.\n%s\n"
	FORKLIFT_UsageString          = "Usage: forklift <cmd> [-b | -v] <image path> <container exec>"
	FORKLIFT_RunNoArgs            = "Error: No arguments specific for the 'run' command.\n%s\n"
)

var (
	FORKLIFT_RUNTIME_BaseFSDir   = "/var/lib/forklift"
	FORKLIFT_RUNTIME_ImagesFSDir = "images/"
)

type runtimeConfig struct {
	Hostname           *string
	ImagePath          *string
	RuntimeBaseFSDir   *string
	RuntimeImagesFSDir *string
	BackgroundFlag     *bool
	Args               []string
}

var debugFlag *bool

func main() {

	runCtx := runtimeConfig{
		Hostname:           nil,
		ImagePath:          nil,
		RuntimeBaseFSDir:   nil,
		RuntimeImagesFSDir: nil,
		BackgroundFlag:     nil,
		Args:               nil,
	}

	runFlagSet := flag.NewFlagSet("run", flag.ExitOnError)
	runCtx.BackgroundFlag = runFlagSet.Bool("b", false, "Spawn the container as a background process.")
	runCtx.Hostname = runFlagSet.String("h", "defaultcontainer", "The hostname or name of the container.")
	debugFlag = runFlagSet.Bool("v", false, "Print verbose debug information.")

	if len(os.Args) < 2 {
		fmt.Printf(FORKLIFT_CommandNotSpecified, FORKLIFT_UsageString)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runFlagSet.Parse(os.Args[2:])
		runCtx.Args = runFlagSet.Args()
		if len(runCtx.Args) < 1 {
			fmt.Printf(FORKLIFT_RunNoArgs, FORKLIFT_UsageString)
		}
		runCmd(runCtx)
	case "child":
		runFlagSet.Parse(os.Args[2:])
		runCtx.Args = runFlagSet.Args()
		if len(runCtx.Args) < 1 {
			fmt.Printf(FORKLIFT_RunNoArgs, FORKLIFT_UsageString)
			os.Exit(1)
		}
		childCmd(runCtx)
	default:
		fmt.Printf(FORKLIFT_CommandNotRecognized, os.Args[1], FORKLIFT_UsageString)
		os.Exit(1)
	}
}

func runCmd(r runtimeConfig) {
	printDebug(r.Args)

	execCommand := exec.Command("/proc/self/exe", append([]string{"child"}, r.Args...)...)
	execCommand.Stdin = os.Stdin
	execCommand.Stdout = os.Stdout
	execCommand.Stderr = os.Stderr
	execCommand.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWIPC | syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
		GidMappingsEnableSetgroups: false,
	}

	// Prepare host file system
	must(setupHostFileSystem(r))

	must(execCommand.Run())
}

func childCmd(r runtimeConfig) {
	printDebug(r.Args)

	// Set hostname
	must(syscall.Sethostname([]byte(*r.Hostname)))
	// Establish clean root to bind-mount on

	execCommand := exec.Command(r.Args[1], r.Args[2:]...)
	execCommand.Stdin = os.Stdin
	execCommand.Stdout = os.Stdout
	execCommand.Stderr = os.Stderr

	must(execCommand.Run())
}

func setupHostFileSystem(r runtimeConfig) error {

	must(validateFilePath(*r.RuntimeBaseFSDir, *r.RuntimeImagesFSDir))
	return nil
}

func validateFilePath(paths ...string) error {
	fullPath := filepath.Join(paths...)
	_, err := os.ReadDir(fullPath)
	if err != nil {
		return os.MkdirAll(fullPath, 0755)
	}
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
