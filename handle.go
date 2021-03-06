package doccurator

import (
	"fmt"
	"github.com/n2code/doccurator/internal/library"
	"github.com/n2code/doccurator/internal/output"
)

type VerbosityMode int
type OptimizationLevel int

// HandleConfig holds a set of common configuration switches that concern all calls to the doccurator API.
// The zero value is a sensible default.
type HandleConfig struct {
	Verbosity             VerbosityMode     //amount of detail in output
	Optimization          OptimizationLevel //performance-vs-thoroughness
	SuppressTerminalCodes bool              //do use fancy terminal formatting options such as ANSI escape sequences to add color
	IncludeAllNamesInScan bool              //if set all names are considered in directory scans (i.e. hidden files/folders starting with "." will be included)
}

const (
	DefaultVerbosity VerbosityMode = iota //normal level of information, all noteworthy facts without too much noise
	VerboseMode                           //exhaustive information about what is happening, repeating context
	QuietMode                             //only output errors and information that was explicitly requested (-> Print* functions)
)

const (
	DefaultOptimizations OptimizationLevel = iota //sensible defaults that reduce disk reads, for example by assuming that modified documents have an updated modification timestamp
	ThoroughMode                                  //sacrifices performance to avoid any possible oversights
)

// New creates a new doccurator library rooted at the given root directory.
// The library database file does not need to be located inside the root directory. However, its path must not be changed after creation.
func New(root string, database string, config HandleConfig) (Doccurator, error) {
	handle := makeDoccurator(config)
	err := handle.createLibrary(mustAbsFilepath(root), mustAbsFilepath(database))
	if err != nil {
		return nil, fmt.Errorf("library create error: %w", err)
	}
	return handle, nil
}

// Open loads the doccurator library database which tracks the given directory. (It does not need to be the library root directory.)
func Open(directory string, config HandleConfig) (Doccurator, error) {
	handle := makeDoccurator(config)
	err := handle.loadLibrary(mustAbsFilepath(directory))
	if err != nil {
		return nil, fmt.Errorf("library load error: %w", err)
	}
	return handle, nil
}

type doccurator struct {
	appLib                library.Api
	rollbackLog           []rollbackStep //series of steps to be executed in reverse order, errors shall be reported but not stop rollback execution
	libFile               string         //absolute, system-native path
	optimizedFsAccess     bool
	printer               output.Printer
	fancyTerminalFeatures bool
	scanAll               bool
}

func makeDoccurator(config HandleConfig) (instance *doccurator) {
	instance = &doccurator{}

	classes := []output.Class{output.Required, output.Error}
	switch config.Verbosity {
	case VerboseMode:
		classes = append(classes, output.Verbose)
		fallthrough
	case DefaultVerbosity:
		classes = append(classes, output.Normal)
	}
	instance.fancyTerminalFeatures = !config.SuppressTerminalCodes
	instance.printer = output.NewPrinter(classes, !config.SuppressTerminalCodes)
	instance.optimizedFsAccess = config.Optimization == DefaultOptimizations
	instance.scanAll = config.IncludeAllNamesInScan
	return
}

func (d doccurator) Print(class output.Class, format string, values ...interface{}) {
	d.printer.ClassifiedPrintf(class, format, values...)
}
