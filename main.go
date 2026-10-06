package main

import (
	"flag"
	"fmt"
	"os"
)

type runFlagStruct struct {
	backgroundFlag *bool
	args           []string
}

var debugFlag *bool

func main() {

	runFlagCtx := runFlagStruct{
		backgroundFlag: nil,
	}

	runFlagSet := flag.NewFlagSet("run", flag.ExitOnError)
	runFlagCtx.backgroundFlag = runFlagSet.Bool("b", false, "Spawn the container as a background process.")
	debugFlag = runFlagSet.Bool("v", false, "Print verbose debug information.")

	if len(os.Args) < 2 {
		print_debug(*debugFlag, os.Args)
		fmt.Printf("Error: Command not recognized.\nUsage: forklift <cmd> [-b | -v] <image path>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runFlagSet.Parse(os.Args[2:])
		runFlagCtx.args = runFlagSet.Args()
		runCmd(runFlagCtx)
	default:
		fmt.Printf("Error: Command %v not recognized.\nUsage: forklift <cmd> [-b | -v] <image path>", os.Args)
		os.Exit(1)
	}

}

func runCmd(r runFlagStruct) {
	print_debug(*debugFlag, r.args)
	os.Exit(0)
}

func print_debug(cond bool, a any) {
	if cond {
		fmt.Println(a)
	}
}
