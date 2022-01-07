package doccinator

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"

	. "github.com/n2code/doccinator/internal/library"
)

func (d *doccinator) createLibrary(absoluteRoot string, absoluteDbFilePath string) error {
	d.appLib = MakeRuntimeLibrary()

	d.appLib.SetRoot(absoluteRoot)

	d.libFile = absoluteDbFilePath
	if err := d.appLib.SaveToLocalFile(absoluteDbFilePath, false); err != nil {
		return err
	}

	locatorLocation := filepath.Join(absoluteRoot, libraryLocatorFileName)
	//TODO: generate slash-direction-safe URL
	if err := os.WriteFile(locatorLocation, []byte("file://"+absoluteDbFilePath), fs.ModePerm); err != nil {
		return fmt.Errorf("writing library locator (%s) failed:\n%w", locatorLocation, err)
	}

	fmt.Fprintf(d.out, "Initialized library with root %s\n", absoluteRoot)
	return nil
}

func (d *doccinator) loadLibrary(startingDirectoryAbsolute string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("library not found: %w", err)
		}
	}()
	currentDir := startingDirectoryAbsolute
	for {
		locatorFile := filepath.Join(currentDir, libraryLocatorFileName)
		stat, statErr := os.Stat(locatorFile)
		if statErr == nil && stat.Mode().IsRegular() {
			contents, readErr := os.ReadFile(locatorFile)
			if readErr != nil {
				return readErr
			}
			url, parseErr := url.Parse(string(contents))
			if parseErr != nil {
				return parseErr
			}
			if url.Scheme != "file" {
				return fmt.Errorf(`scheme of URL in library locator file (%s) missing or unsupported: "%s"`, locatorFile, url.Scheme)
			}
			//TODO: adapt to slash-agnostic URL format
			d.libFile = url.Path
			d.appLib = MakeRuntimeLibrary()
			d.appLib.LoadFromLocalFile(d.libFile)
			return nil
		} else if errors.Is(statErr, os.ErrNotExist) {
			if currentDir == "/" {
				return errors.New("stopping at filesystem root")
			}
			currentDir = filepath.Dir(currentDir)
		} else {
			return statErr
		}
	}
}
