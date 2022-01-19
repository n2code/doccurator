package doccinator

import (
	"fmt"
	"path/filepath"
	"strings"

	document "github.com/n2code/doccinator/internal/document"
	. "github.com/n2code/doccinator/internal/library"
	output "github.com/n2code/doccinator/internal/output"
)

// Records a new document in the library
func (d *doccinator) CommandAdd(id document.DocumentId, filePath string) error {
	//TODO [FEATURE]: detect and prevent adding existing paths
	doc, err := d.appLib.CreateDocument(id)
	if err != nil {
		return newCommandError("document creation blocked", err)
	}
	err = d.appLib.SetDocumentPath(doc, mustAbsFilepath(filePath))
	if err != nil {
		return newCommandError("document creation impossible", err)
	}
	_, err = d.appLib.UpdateDocumentFromFile(doc)
	if err != nil {
		return newCommandError("document creation failed", err)
	}
	fmt.Fprintf(d.extraOut, "Added %s: %s\n", id, doc.PathRelativeToLibraryRoot())
	return nil
}

// Updates an existing document in the library
func (d *doccinator) CommandUpdateByPath(filePath string) error {
	doc, exists := d.appLib.GetDocumentByPath(mustAbsFilepath(filePath))
	if !exists {
		return newCommandError(fmt.Sprintf("path not on record: %s", filePath), nil)
	}
	changed, err := d.appLib.UpdateDocumentFromFile(doc)
	if !changed {
		fmt.Fprintf(d.extraOut, "No changes detected in %s\n", doc.PathRelativeToLibraryRoot())
	}
	return err
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
//  Tracked and removed paths are listed depending on the flag.
func (d *doccinator) CommandTree(excludeUnchanged bool) error {
	skipDbAndPointers := func(path string) bool {
		return path == d.libFile || filepath.Base(path) == libraryLocatorFileName
	}
	tree := output.NewVisualFileTree(d.appLib.GetRoot() + " [library root]")

	var pathsWithErrors []*CheckedPath
	paths, ok := d.appLib.Scan(skipDbAndPointers)
	for index, checkedPath := range paths {
		prefix := ""
		status := checkedPath.Status()
		if excludeUnchanged && (status == Tracked || status == Removed) {
			continue
		}
		if status != Tracked {
			prefix = fmt.Sprintf("[%s] ", string(status))
		}
		tree.InsertPath(checkedPath.PathRelativeToLibraryRoot(), prefix)
		if status == Error {
			pathsWithErrors = append(pathsWithErrors, &paths[index])
		}
	}

	fmt.Fprint(d.out, tree.Render())

	//TODO [FEATURE]: coloring
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

	if len(paths) > 0 {
		fmt.Fprintf(d.extraOut, "Status of %d specified path%s:\n", len(paths), output.PluralS(paths))
	}
	fmt.Fprintln(d.out)

	var errorMessages strings.Builder
	errorCount := 0
	hasChanges := false

	processResult := func(result *CheckedPath, absolutePath string) {
		switch status := result.Status(); status {
		case Error:
			fmt.Fprintf(&errorMessages, "  [E] @%s\n      %s\n", absolutePath, result.GetError())
			errorCount++
		default:
			buckets[status] = append(buckets[status], mustRelFilepathToWorkingDir(absolutePath))
			if status.RepresentsChange() {
				hasChanges = true
			}
		}
	}

	if len(paths) > 0 {
		for _, path := range paths {
			abs := mustAbsFilepath(path)
			result := d.appLib.CheckFilePath(abs)
			processResult(&result, abs)
		}
	} else {
		skipDbAndPointers := func(path string) bool {
			return path == d.libFile || filepath.Base(path) == libraryLocatorFileName
		}
		results, _ := d.appLib.Scan(skipDbAndPointers)
		for _, result := range results {
			processResult(&result, filepath.Join(d.appLib.GetRoot(), result.PathRelativeToLibraryRoot()))
		}
	}

	//TODO [FEATURE]: coloring
	for status, bucket := range buckets {
		if !status.RepresentsChange() && len(paths) == 0 {
			continue //to hide unchanged files when no explicit paths are queried
		}
		fmt.Fprintf(d.out, " %s (%d file%s)\n", status, len(bucket), output.PluralS(bucket))
		for _, path := range bucket {
			fmt.Fprintf(d.out, "  [%s] %s\n", string(rune(status)), path)
		}
		fmt.Fprintln(d.out)
	}
	if errorCount > 0 {
		fmt.Fprintf(d.out, " Error%s occurred:\n%s\n", output.PluralS(errorCount != 1), errorMessages.String()) //not on stderr because it was explicitly queried
	} else if hasChanges == false && len(paths) == 0 {
		fmt.Fprint(d.out, " Library in sync with all records.\n\n")
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
