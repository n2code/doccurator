package doccurator

import (
	"fmt"
	"github.com/n2code/doccurator/internal/library"
	"io"
	"os"
)

type VerbosityLevel int
type OptimizationLevel int

// CreateConfig holds a set of common configuration switches that concern all calls to the doccurator API.
// The zero value is a sensible default.
type CreateConfig struct {
	Verbosity    VerbosityLevel
	Optimization OptimizationLevel
}

const (
	DefaultVerbosity VerbosityLevel = iota //normal level of information, all noteworthy facts without too much noise
	VerboseMode                            //exhaustive information about what is happening, repeating context
	QuietMode                              //only output errors and information that was explicitly requested (-> Print* functions)
)

const (
	DefaultOptimizations OptimizationLevel = iota //sensible defaults that reduce disk reads, for example by assuming that modified documents have an updated modification timestamp
	ThoroughMode                                  //sacrifices performance to avoid any possible oversights
)

// New creates a new doccurator library rooted at the given root directory.
// The library database file does not need to be located inside the root directory. However, its path must not be changed after creation.
func New(root string, database string, config CreateConfig) (Doccurator, error) {
	handle := makeDoccurator(config)
	err := handle.createLibrary(mustAbsFilepath(root), mustAbsFilepath(database))
	if err != nil {
		return nil, fmt.Errorf("library create error: %w", err)
	}
	return handle, nil
}

// Open loads the doccurator library database which tracks the given directory. (It does not need to be the library root directory.)
func Open(directory string, config CreateConfig) (Doccurator, error) {
	handle := makeDoccurator(config)
	err := handle.loadLibrary(mustAbsFilepath(directory))
	if err != nil {
		return nil, fmt.Errorf("library load error: %w", err)
	}
	return handle, nil
}

type doccurator struct {
	appLib            library.Api
	rollbackLog       []rollbackStep //series of steps to be executed in reverse order, errors shall be reported but not stop rollback execution
	libFile           string         //absolute, system-native path
	out               io.Writer      //essential output (i.e. requested information)
	extraOut          io.Writer      //more output for convenience (repeats context)
	verboseOut        io.Writer      //most output, talkative
	errOut            io.Writer      //error output
	optimizedFsAccess bool
}

func makeDoccurator(config CreateConfig) (instance *doccurator) {
	instance = &doccurator{out: os.Stdout, extraOut: io.Discard, verboseOut: io.Discard, errOut: os.Stderr}
	switch config.Verbosity {
	case VerboseMode:
		instance.verboseOut = os.Stdout
		fallthrough
	case DefaultVerbosity:
		instance.extraOut = os.Stdout
	}
	if config.Optimization == DefaultOptimizations {
		instance.optimizedFsAccess = true
	}
	return
}
