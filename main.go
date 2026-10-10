package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/null0routed/dockyard-forklift/internal/archives"
)

var (
	FORKLIFT_ErrCommandNotRecognized = "Error: Command %s not recognized.\n%s\n"
	FORKLIFT_ErrCommandNotSpecified  = "Error: Command not specified.\n%s\n"
	FORKLIFT_ErrUsageString          = "Usage: forklift <cmd> [-b | -v] <image path> <container exec>"
	FORKLIFT_ErrRunNoArgs            = "Error: No arguments specific for the 'run' command.\n%s\n"
	FORKLIFT_ErrCouldNotReadFile     = "Error: Unable to process image at %s.\n"
	FORKLIFT_ErrCouldNotProcessImage = "Error: Image %s was not a valid image.\n"
)

var (
	FORKLIFT_RUNTIME_BaseFSDir       = "/var/lib/forklift"
	FORKLIFT_RUNTIME_ImagesFSDir     = filepath.Join(FORKLIFT_RUNTIME_BaseFSDir, "images/")
	FORKLIFT_RUNTIME_ContainersFSDir = filepath.Join(FORKLIFT_RUNTIME_BaseFSDir, "containers/")
)

type runtimeConfig struct {
	Hostname               *string
	ImagePath              *string
	ImageName              *string
	RuntimeBaseFSDir       *string
	RuntimeImagesFSDir     *string
	RuntimeContainersFSDir *string
	BackgroundFlag         *bool
	Args                   []string
}

var debugFlag *bool

func main() {

	runCtx := runtimeConfig{
		Hostname:               nil,
		ImagePath:              nil,
		RuntimeBaseFSDir:       nil,
		RuntimeImagesFSDir:     nil,
		RuntimeContainersFSDir: nil,
		BackgroundFlag:         nil,
		Args:                   nil,
	}

	upFlagSet := flag.NewFlagSet("up", flag.ExitOnError)
	runCtx.BackgroundFlag = upFlagSet.Bool("b", false, "Spawn the container as a background process.")
	runCtx.Hostname = upFlagSet.String("h", "defaultcontainer", "The hostname or name of the container.")
	runCtx.RuntimeBaseFSDir = upFlagSet.String("base-dir", FORKLIFT_RUNTIME_BaseFSDir, "The base directory for forklift.")
	runCtx.RuntimeImagesFSDir = upFlagSet.String("images-dir", FORKLIFT_RUNTIME_ImagesFSDir, "The directory to store local container images.")
	runCtx.RuntimeContainersFSDir = upFlagSet.String("containers-dir", FORKLIFT_RUNTIME_ContainersFSDir, "The directory to store active container mounts.")
	debugFlag = upFlagSet.Bool("v", false, "Print verbose debug information.")

	if len(os.Args) < 2 {
		fmt.Printf(FORKLIFT_ErrCommandNotSpecified, FORKLIFT_ErrUsageString)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		upFlagSet.Parse(os.Args[2:])
		runCtx.Args = upFlagSet.Args()
		if len(runCtx.Args) < 1 {
			fmt.Printf(FORKLIFT_ErrRunNoArgs, FORKLIFT_ErrUsageString)
		}
		upCmd(runCtx)
	case "child":
		upFlagSet.Parse(os.Args[2:])
		runCtx.Args = upFlagSet.Args()
		if len(runCtx.Args) < 1 {
			fmt.Printf(FORKLIFT_ErrRunNoArgs, FORKLIFT_ErrUsageString)
			os.Exit(1)
		}
		childCmd(runCtx)
	default:
		fmt.Printf(FORKLIFT_ErrCommandNotRecognized, os.Args[1], FORKLIFT_ErrUsageString)
		os.Exit(1)
	}
}

func upCmd(r runtimeConfig) {
	printDebug(r.Args)

	// Prepare host file system
	must(setupHostFileSystem(r))

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

	must(execCommand.Run())
}

func childCmd(r runtimeConfig) {
	printDebug(r.Args)

	// Set hostname
	must(syscall.Sethostname([]byte(*r.Hostname)))

	// Stop propagation of mounts to host and establish private mounts
	must(syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""))

	// Build local mount

	execCommand := exec.Command(r.Args[1], r.Args[2:]...)
	execCommand.Stdin = os.Stdin
	execCommand.Stdout = os.Stdout
	execCommand.Stderr = os.Stderr

	must(execCommand.Run())
}

func setupHostFileSystem(r runtimeConfig) error {

	// File path for downloaded images
	must(checkOrBuildFilePath(*r.RuntimeImagesFSDir))

	// File path for where running container mounts live
	must(checkOrBuildFilePath(*r.RuntimeContainersFSDir))

	// Validate image is .tar.gz or a directory in the runtime path
	if strings.HasSuffix(*r.ImagePath, ".tar.gz") || strings.HasSuffix(*r.ImagePath, ".tgz") {
		// Ungzip to tmp
		imageName := filepath.Base(*r.ImagePath)
		targetPath := ""

		if strings.HasSuffix(imageName, ".tar.gz") {
			imageName = strings.TrimSuffix(imageName, ".tar.gz")
		} else if strings.HasSuffix(imageName, ".tgz") {
			imageName = strings.TrimSuffix(imageName, ".tgz")
		}
		targetPath = filepath.Join("/tmp", imageName, ".tar")

		must(archives.UnGzip(*r.ImagePath, targetPath))

		must(archives.Untar(targetPath, filepath.Join(*r.RuntimeImagesFSDir, imageName)))

		r.ImageName = &imageName
	} else {
		fileinfo, err := os.Stat(*r.ImagePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, FORKLIFT_ErrCouldNotReadFile, *r.ImagePath)
			os.Exit(1)
		}

		if fileinfo.IsDir() {
			// Check if its in image folder
			if !strings.HasPrefix(*r.ImagePath, *r.RuntimeImagesFSDir) {
				fmt.Fprintf(os.Stderr, FORKLIFT_ErrCouldNotProcessImage, *r.ImagePath)
				os.Exit(1)
			}

			imageName := filepath.Base(*r.ImagePath)
			r.ImageName = &imageName
			return nil
		}
	}

	fmt.Fprintf(os.Stderr, FORKLIFT_ErrCouldNotProcessImage, *r.ImagePath)
	os.Exit(1)
	return nil
}

func setupContainerFS(r runtimeConfig) error {
	return nil
}

func checkOrBuildFilePath(paths ...string) error {
	fullPath := filepath.Join(paths...)
	_, err := os.ReadDir(fullPath)
	if err != nil {
		return os.MkdirAll(fullPath, 0755)
	}

	return nil
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
