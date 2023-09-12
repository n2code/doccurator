package doccurator

import (
	"fmt"
	"github.com/n2code/doccurator/internal/library"
	"github.com/n2code/doccurator/internal/output"
	"os"
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
// The library database file does not need to be located inside the root directory.
// However, its path must not be changed after creation.
func New(root string, database string, config HandleConfig) (Doccurator, error) {
	handle := makeDoccurator(config)
	absoluteRoot := mustAbsFilepath(root)

	err := handle.createLibrary(absoluteRoot, mustAbsFilepath(database))
	if err != nil {
		return nil, fmt.Errorf("library create error: %w", err)
	}
	handle.Print(output.Normal, "Initialized library with root %s\n", absoluteRoot)

	if err := handle.createLocatorFile(absoluteRoot, false); err != nil {
		return handle, err //the handle is usable regardless whether locator creation failed
	}

	return handle, nil
}

// Open loads the doccurator library database which tracks the given directory.
// (It does not need to be the library root directory.)
func Open(directory string, config HandleConfig) (Doccurator, error) {
	handle := makeDoccurator(config)

	err := handle.discoverLibraryFile(mustAbsFilepath(directory))
	if err != nil {
		return nil, fmt.Errorf("library discovery error: %w", err)
	}

	handle.loadLibrary()

	root := handle.appLib.GetRoot()
	stat, statErr := os.Stat(root)
	if statErr != nil {
		return nil, fmt.Errorf("library open error: %w", statErr)
	} else if !stat.Mode().IsDir() {
		return nil, fmt.Errorf("library open error: root %s is not a directory", root)
	}

	return handle, nil
}

// Move updates the doccurator library root without touching the persisted document paths.
// (If the entire library root directory has moved no changes will be detected afterward.
// If the root is set to a parent directory all documents will be considered moved.)
func Move(newRoot string, database string, config HandleConfig) error {
	handle := makeDoccurator(config)
	handle.libFile = mustAbsFilepath(database)
	handle.loadLibrary()

	absNewRoot := mustAbsFilepath(newRoot)
	handle.appLib.SetRoot(absNewRoot)

	if err := handle.PersistChanges(); err != nil {
		return err
	}
	handle.Print(output.Normal, "Re-Initialized library with root %s\n", absNewRoot)

	if err := handle.createLocatorFile(absNewRoot, true); err != nil {
		return err
	}

	return nil
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

func (d *doccurator) Print(class output.Class, format string, values ...interface{}) {
	d.printer.ClassifiedPrintf(class, format, values...)
}
