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
	fmt.Fprintf(d.out, "Added %s: %s\n", id, doc.PathRelativeToLibraryRoot())
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
	fmt.Fprintf(d.out, "Library: %s\n--------\n", d.appLib.GetRoot())
	d.appLib.PrintAllRecords(d.out)
}

// Calculates states for all present and recorded paths.
//  Tracked and removed paths require special flag triggers to be listed.
func (d *doccinator) CommandScan() error {
	skipDbAndPointers := func(path string) bool {
		return path == d.libFile || filepath.Base(path) == libraryLocatorFileName
	}
	tree := output.NewVisualFileTree(d.appLib.GetRoot() + " [library root]")

	paths := d.appLib.Scan(skipDbAndPointers)
	for _, checkedPath := range paths {
		prefix := ""
		if status := checkedPath.Status(); status != Tracked {
			prefix = fmt.Sprintf("[%s] ", string(status))
		}
		tree.InsertPath(checkedPath.PathRelativeToLibraryRoot(), prefix)
	}
	fmt.Fprint(d.out, tree.Render())
	return nil
}

// Queries the given [possibly relative] paths about their affiliation and state with respect to the library
func (d *doccinator) CommandStatus(paths []string) error {
	var buckets map[PathStatus][]string = make(map[PathStatus][]string)
	fmt.Fprintf(d.out, "Checking %d path%s against library %s ...\n\n", len(paths), output.PluralS(paths), d.appLib.GetRoot())

	var errorMessages strings.Builder

	for _, path := range paths {
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		res, err := d.appLib.CheckFilePath(abs)

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
		fmt.Fprintf(d.out, "Errors occurred:\n%s\n", errorMessages.String())
	}
	for status, bucket := range buckets {
		fmt.Fprintf(d.out, "%s (%d file%s)\n", status, len(bucket), output.PluralS(bucket))
		for _, path := range bucket {
			fmt.Fprintf(d.out, "  [%s] %s\n", string(rune(status)), path)
		}
		fmt.Fprintln(d.out)
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