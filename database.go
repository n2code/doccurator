package doccurator

import (
	"errors"
	"fmt"
	out "github.com/n2code/doccurator/internal/output"
	"net/url"
	"os"
	"path/filepath"

	"github.com/n2code/doccurator/internal/library"
)

func (d *doccurator) PersistChanges() error {
	if err := d.appLib.SaveToLocalFile(d.libFile, true); err != nil {
		return fmt.Errorf("library save error: %w", err)
	}
	d.rollbackLog = nil
	return nil

}

type rollbackStep func() error

func (d *doccurator) RollbackAllFilesystemChanges() (complete bool) {
	complete = true
	if len(d.rollbackLog) == 0 { //early exit if rollback is no-op
		return
	}

	d.Print(out.Normal, "Executing filesystem rollback...")
	for i := len(d.rollbackLog) - 1; i >= 0; i-- {
		step := d.rollbackLog[i]
		if err := step(); err != nil {
			if complete { //i.e. first issue encountered
				d.Print(out.Normal, "\n")
			}
			d.Print(out.Normal, "  ")
			d.Print(out.Error, "rollback issue: %s\n", err)
			complete = false
			//errors are reported but execution continues to achieve best partial rollback possible
		}
	}
	if complete {
		d.Print(out.Normal, " DONE!\n")
	} else {
		d.Print(out.Normal, "  Rollback completed partially, issues occurred.\n")
	}
	d.rollbackLog = nil //note: failed rollback steps are not preserved
	return
}

func (d *doccurator) createLibrary(absoluteRoot string, absoluteDbFilePath string) error {
	d.appLib = library.NewLibrary()

	d.appLib.SetRoot(absoluteRoot)

	d.libFile = absoluteDbFilePath
	if err := d.appLib.SaveToLocalFile(absoluteDbFilePath, false); err != nil {
		return err
	}

	if err := d.createLocatorFile(absoluteRoot); err != nil {
		return err
	}

	d.Print(out.Normal, "Initialized library with root %s\n", absoluteRoot)
	d.Print(out.Verbose, "Database saved in %s\n", absoluteDbFilePath)
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
		locatorPath := filepath.Join(currentDir, library.LocatorFileName)
		stat, statErr := os.Stat(locatorPath)
		if statErr == nil && stat.Mode().IsRegular() {
			err = d.loadLibFilePathFromLocatorFile(locatorPath)
			if err != nil {
				return
			}
			d.appLib = library.NewLibrary()
			d.appLib.LoadFromLocalFile(d.libFile)
			d.Print(out.Verbose, "Loaded library rooted at %s from %s\n", d.appLib.GetRoot(), d.libFile)
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

const libraryLocatorPermissions = 0o440 //owner and group can read

func (d *doccurator) createLocatorFile(directory string) error {
	path := filepath.Join(directory, library.LocatorFileName)
	locationUri := url.URL{Scheme: "file", Path: d.libFile}
	if err := os.WriteFile(path, []byte(locationUri.String()), libraryLocatorPermissions); err != nil {
		return fmt.Errorf("writing library locator (%s) failed: %w", path, err)
	}
	d.Print(out.Verbose, "Created library locator %s\n", path)
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
	d.Print(out.Verbose, "Used library locator %s\n", path)
	return nil
}
