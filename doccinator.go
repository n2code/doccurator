package doccinator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	document "github.com/n2code/doccinator/internal/document"
	. "github.com/n2code/doccinator/internal/library"
)

type Doccinator interface {
	CommandAdd(id document.DocumentId, path string) error
	CommandUpdateByPath(fileAbsolutePath string) error //TODO: de-absolutize
	CommandRemoveByPath(fileAbsolutePath string) error //TODO: de-absolutize
	CommandDump()
	CommandScan() error
	CommandStatus(paths []string) error
	CommandAuto() error
	PersistChanges() error
}

type doccinator struct {
	appLib     LibraryApi
	libFile    string //absolute, system-native path
	out        io.Writer
	verboseOut io.Writer //TODO: make use of this
}

const libraryLocatorFileName = ".doccinator"

func New(root string, database string) (Doccinator, error) {
	handle := &doccinator{out: os.Stdout, verboseOut: io.Discard}
	err := handle.createLibrary(mustAbsFilepath(root), mustAbsFilepath(database))
	if err != nil {
		return nil, fmt.Errorf("library create error: %w", err)
	}
	return handle, nil
}

func Open(directory string) (Doccinator, error) {
	handle := &doccinator{out: os.Stdout, verboseOut: io.Discard}
	err := handle.loadLibrary(mustAbsFilepath(directory))
	if err != nil {
		return nil, fmt.Errorf("library load error: %w", err)
	}
	return handle, nil
}

func (d *doccinator) PersistChanges() error {
	if err := d.appLib.SaveToLocalFile(d.libFile, true); err != nil {
		return fmt.Errorf("library save error: %w", err)
	}
	return nil

}

func mustAbsFilepath(somePath string) string {
	abs, err := filepath.Abs(somePath)
	if err != nil {
		panic(err)
	}
	return abs
}
