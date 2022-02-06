package doccurator

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/n2code/doccurator/internal/library"
)

const libraryLocatorPermissions = 0o440 //owner and group can read

func (d *doccurator) createLibrary(absoluteRoot string, absoluteDbFilePath string) error {
	d.appLib = library.MakeRuntimeLibrary()

	d.appLib.SetRoot(absoluteRoot)

	d.libFile = absoluteDbFilePath
	if err := d.appLib.SaveToLocalFile(absoluteDbFilePath, false); err != nil {
		return err
	}

	if err := d.createLocatorFile(absoluteRoot); err != nil {
		return err
	}

	fmt.Fprintf(d.extraOut, "Initialized library with root %s\n", absoluteRoot)
	fmt.Fprintf(d.verboseOut, "Database saved in %s\n", absoluteDbFilePath)
	return nil
}

func (d *doccurator) loadLibrary(startingDirectoryAbsolute string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("library not found: %w", err)
		}
	}()
	currentDir := startingDirectoryAbsolute
	for {
		locatorPath := filepath.Join(currentDir, library.LibraryLocatorFileName)
		stat, statErr := os.Stat(locatorPath)
		if statErr == nil && stat.Mode().IsRegular() {
			err = d.loadLibFilePathFromLocatorFile(locatorPath)
			if err != nil {
				return
			}
			d.appLib = library.MakeRuntimeLibrary()
			d.appLib.LoadFromLocalFile(d.libFile)
			fmt.Fprintf(d.verboseOut, "Loaded library rooted at %s from %s\n", d.appLib.GetRoot(), d.libFile)
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

func (d *doccurator) createLocatorFile(directory string) error {
	path := filepath.Join(directory, library.LibraryLocatorFileName)
	locationUri := url.URL{Scheme: "file", Path: d.libFile}
	if err := os.WriteFile(path, []byte(locationUri.String()), libraryLocatorPermissions); err != nil {
		return fmt.Errorf("writing library locator (%s) failed: %w", path, err)
	}
	fmt.Fprintf(d.verboseOut, "Created library locator %s\n", path)
	return nil
}

func (d *doccurator) loadLibFilePathFromLocatorFile(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	url, err := url.Parse(string(contents))
	if err != nil {
		return err
	}
	if url.Scheme != "file" {
		return fmt.Errorf(`scheme of URL in library locator file (%s) missing or unsupported: "%s"`, path, url.Scheme)
	}
	if !filepath.IsAbs(url.Path) {
		return fmt.Errorf(`no absolute path in library locator file (%s): "%s"`, path, url.Path)
	}
	d.libFile = url.Path
	fmt.Fprintf(d.verboseOut, "Used library locator %s\n", path)
	return nil
}
