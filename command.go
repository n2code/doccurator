package doccinator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	document "github.com/n2code/doccinator/internal/document"
	. "github.com/n2code/doccinator/internal/library"
	output "github.com/n2code/doccinator/internal/output"
)

// Records a new document in the library
func (d *doccinator) CommandAdd(id document.DocumentId, filePath string) error {
	doc, err := d.appLib.CreateDocument(id)
	if err != nil {
		return newCommandError("document creation failed", err)
	}
	d.appLib.SetDocumentPath(doc, mustAbsFilepath(filePath))
	d.appLib.UpdateDocumentFromFile(doc)
	fmt.Fprintf(d.extraOut, "Added %s: %s\n", id, doc.PathRelativeToLibraryRoot())
	return nil
}

// Updates an existing document in the library
func (d *doccinator) CommandUpdateByPath(fileAbsolutePath string) error {
	doc, exists := d.appLib.GetDocumentByPath(fileAbsolutePath)
	if !exists {
		return newCommandError(fmt.Sprintf("path unknown: %s", fileAbsolutePath), nil)
	}
	d.appLib.UpdateDocumentFromFile(doc)
	return nil
}

// Removes an existing document from the library
func (d *doccinator) CommandRemoveByPath(fileAbsolutePath string) error {
	doc, exists := d.appLib.GetDocumentByPath(fileAbsolutePath)
	if !exists {
		return newCommandError(fmt.Sprintf("path not on record: %s", fileAbsolutePath), nil)
	}
	d.appLib.ForgetDocument(doc)
	return nil
}

// Outputs all library records
func (d *doccinator) CommandDump() {
	fmt.Fprintf(d.extraOut, "Library: %s\n\n", d.appLib.GetRoot())
	count := 0
	d.appLib.VisitAllRecords(func(doc document.DocumentApi) {
		fmt.Fprintf(d.out, "%s\n", doc)
		count++
	})
	if count == 0 {
		fmt.Fprintln(d.extraOut, "<no records>")
	} else {
		fmt.Fprintf(d.extraOut, "\n%d in total\n", count)
	}
}

// Calculates states for all present and recorded paths.
//  Tracked and removed paths require special flag triggers to be listed. //<-- TODO [FEATURE]: implement said flags
func (d *doccinator) CommandScan() error {
	skipDbAndPointers := func(path string) bool {
		return path == d.libFile || filepath.Base(path) == libraryLocatorFileName
	}
	tree := output.NewVisualFileTree(d.appLib.GetRoot() + " [library root]")

	var pathsWithErrors []*CheckedPath
	paths, ok := d.appLib.Scan(skipDbAndPointers)
	for index, checkedPath := range paths {
		prefix := ""
		status := checkedPath.Status()
		if status != Tracked {
			prefix = fmt.Sprintf("[%s] ", string(status))
		}
		tree.InsertPath(checkedPath.PathRelativeToLibraryRoot(), prefix)
		if status == Error {
			pathsWithErrors = append(pathsWithErrors, &paths[index])
		}
	}

	fmt.Fprint(d.out, tree.Render())

	if !ok {
		var msg strings.Builder
		fmt.Fprintf(&msg, "Scanning error%s occurred:\n", output.PluralS(len(pathsWithErrors) != 1))
		for _, errorPath := range pathsWithErrors {
			fmt.Fprintf(&msg, "@%s: %s\n", errorPath.PathRelativeToLibraryRoot(), errorPath.GetError())
		}
		return fmt.Errorf(msg.String())
	} else {
		return nil
	}
}

// Queries the given [possibly relative] paths about their affiliation and state with respect to the library
func (d *doccinator) CommandStatus(paths []string) error {
	var buckets map[PathStatus][]string = make(map[PathStatus][]string)
	fmt.Fprintf(d.verboseOut, "Checking %d path%s against library %s ...\n", len(paths), output.PluralS(paths), d.appLib.GetRoot())
	fmt.Fprintln(d.out)

	var errorMessages strings.Builder
	errorCount := 0

	for _, path := range paths {
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		res := d.appLib.CheckFilePath(abs)

		switch status := res.Status(); status {
		case Error:
			fmt.Fprintf(&errorMessages, "  [E] @%s: %s\n", abs, res.GetError())
			errorCount++
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

	for status, bucket := range buckets {
		fmt.Fprintf(d.out, "%s (%d file%s)\n", status, len(bucket), output.PluralS(bucket))
		for _, path := range bucket {
			fmt.Fprintf(d.out, "  [%s] %s\n", string(rune(status)), path)
		}
		fmt.Fprintln(d.out)
	}
	if errorCount > 0 {
		fmt.Fprintf(d.out, "Error%s occurred:\n%s\n", output.PluralS(errorCount != 1), errorMessages.String()) //not on stderr because it was explicitly queried
	}
	return nil
}

// Auto pilot adds untracked paths, updates touched & moved paths, and removes duplicates.
//  Modified and missing are not changed but reported.
//  If additional flags are passed modified paths are updated and/or missing paths removed.
//  Unknown paths are reported.
func (d *doccinator) CommandAuto() error {
	return nil
}
