package doccinator

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	document "github.com/n2code/doccinator/internal/document"
	. "github.com/n2code/doccinator/internal/library"
)

const libraryPointerFileName = ".doccinator"

var appLib LibraryApi
var libFile string

type CommandError struct {
	message string
	cause   error
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("%s: %s", e.message, e.cause)
}

func (e *CommandError) Unwrap() error {
	return e.cause
}

func newCommandError(message string, cause error) *CommandError {
	return &CommandError{message: message, cause: cause}
}

// Records a new document in the library
func CommandAdd(id document.DocumentId, fileAbsolutePath string) error {
	doc, err := appLib.CreateDocument(id)
	if err != nil {
		return newCommandError("document creation failed", err)
	}
	appLib.SetDocumentPath(doc, fileAbsolutePath)
	appLib.UpdateDocumentFromFile(doc)
	return nil
}

// Updates an existing document in the library
func CommandUpdateByPath(fileAbsolutePath string) error {
	doc, exists := appLib.GetDocumentByPath(fileAbsolutePath)
	if !exists {
		return newCommandError(fmt.Sprintf("path unknown: %s", fileAbsolutePath), nil)
	}
	appLib.UpdateDocumentFromFile(doc)
	return nil
}

// Removes an existing document from the library
func CommandRemoveByPath(fileAbsolutePath string) error {
	doc, exists := appLib.GetDocumentByPath(fileAbsolutePath)
	if !exists {
		return newCommandError(fmt.Sprintf("path not on record: %s", fileAbsolutePath), nil)
	}
	appLib.ForgetDocument(doc)
	return nil
}

// Outputs all library records
func CommandDump(out io.Writer) {
	allRecords := appLib.AllRecordsAsText()

	fmt.Fprint(out, allRecords)
	if len(allRecords) == 0 {
		fmt.Fprintln(out, "<no records>")
	}
}

// Calculates states for all present and recorded paths.
//  Tracked and removed paths require special flag triggers to be listed.
func CommandScan(out io.Writer) error {
	appLib.ChdirToRoot()
	workingDir, _ := os.Getwd()
	fmt.Fprintf(out, "Scanning library in %s ...\n", workingDir)
	skipDbAndPointers := func(path string) bool {
		return path == libFile || filepath.Base(path) == libraryPointerFileName
	}
	paths := appLib.Scan(skipDbAndPointers)
	for _, checkedPath := range paths {
		fmt.Fprintf(out, "[%s] %s\n", string(checkedPath.Status()), checkedPath.PathRelativeToLibraryRoot())
	}
	return nil
}

// Queries the given [possibly relative] paths about their affiliation and state with respect to the library
func CommandStatus(paths []string, out io.Writer) error {
	var buckets map[PathStatus][]string = make(map[PathStatus][]string)

	var errorMessages strings.Builder

	for _, path := range paths {
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		res, err := appLib.CheckFilePath(abs)

		switch status := res.Status(); status {
		case Error:
			fmt.Fprintf(&errorMessages, "  [E] %s (%s)\n", err, abs)
		default:
			displayPath := "" //relative to working directory, if possible
			if wd, err := os.Getwd(); err != nil {
				displayPath, _ = filepath.Rel(wd, abs)
			}
			if len(displayPath) == 0 {
				displayPath = path
			}
			buckets[status] = append(buckets[status], displayPath)
		}

	}

	if errorMessages.Len() > 0 {
		fmt.Fprintf(out, "Errors occurred:\n%s\n", errorMessages.String())
	}
	for status, bucket := range buckets {
		fmt.Fprintf(out, "%s (%d file%s)\n", status, len(bucket), func() (pluralS string) {
			if len(bucket) != 1 {
				pluralS = "s"
			}
			return
		}())
		for _, path := range bucket {
			fmt.Fprintf(out, "  [%s] %s\n", string(rune(status)), path)
		}
		fmt.Fprintln(out)
	}
	return nil
}

// Auto pilot adds untracked paths, updates touched & moved paths, and removes duplicates.
//  Modified and missing are not changed but reported.
//  If additional flags are passed modified paths are updated and/or missing paths removed.
//  Unknown paths are reported.
func CommandAuto() error {
	return nil
}

func InitLibrary(absoluteRoot string, absoluteDbFilePath string) {
	appLib = MakeRuntimeLibrary()
	appLib.SetRoot(absoluteRoot)
	libFile = absoluteDbFilePath
	appLib.SaveToLocalFile(absoluteDbFilePath, false)
	err := os.WriteFile(filepath.Join(absoluteRoot, libraryPointerFileName), []byte("file://"+absoluteDbFilePath), fs.ModePerm)
	if err != nil {
		panic(err)
	}
}

func DiscoverAppLibrary(startingDirectoryAbsolute string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("library not found: %w", err)
		}
	}()
	currentDir := startingDirectoryAbsolute
	for {
		pointerFile := path.Join(currentDir, libraryPointerFileName)
		stat, statErr := os.Stat(pointerFile)
		if statErr == nil && stat.Mode().IsRegular() {
			contents, readErr := os.ReadFile(pointerFile)
			if readErr != nil {
				return readErr
			}
			url, parseErr := url.Parse(string(contents))
			if parseErr != nil {
				return parseErr
			}
			if url.Scheme != "file" {
				return errors.New("scheme of URL in library locator file missing or unsupported: " + url.Scheme)
			}
			libFile = url.Path
			appLib = MakeRuntimeLibrary()
			appLib.LoadFromLocalFile(libFile)
			return nil
		} else if errors.Is(statErr, os.ErrNotExist) {
			if currentDir == "/" {
				return errors.New("stopping at filesystem root")
			}
			currentDir = path.Dir(currentDir)
		} else {
			return statErr
		}
	}
}

func PersistDatabase() {
	appLib.SaveToLocalFile(libFile, true)
}
