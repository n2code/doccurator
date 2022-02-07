package doccurator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/n2code/doccurator/internal/document"
	"github.com/n2code/doccurator/internal/library"
	"github.com/n2code/doccurator/internal/output"
)

func getSkipperForDbAndPointers(libFilePath string) func(path string) (skip bool) {
	return func(path string) bool {
		return path == libFilePath || filepath.Base(path) == library.LocatorFileName
	}
}

// Records a new document in the library
func (d *doccurator) CommandAddSingle(id document.Id, filePath string) error {
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

func (d *doccurator) CommandAddAllUntracked() error {
	results, noScanErrors := d.appLib.Scan(getSkipperForDbAndPointers(d.libFile))
	if !noScanErrors {
		fmt.Fprint(d.extraOut, "Issues during scan: Not all potential candidates accessible\n")
	}

	var untrackedRootRelativePaths []string
	for _, checked := range results {
		switch checked.Status() {
		case library.Untracked:
			untrackedRootRelativePaths = append(untrackedRootRelativePaths, checked.PathRelativeToLibraryRoot())
		case library.Error:
			fmt.Fprintf(d.extraOut, "Skipping uncheckable (%s): %s\n", checked.PathRelativeToLibraryRoot(), checked.GetError())
		}
	}

	for _, untracked := range untrackedRootRelativePaths {
		id, err := ExtractIdFromStandardizedFilename(untracked)
		if err != nil {
			return err
		}
		err = d.CommandAddSingle(id, filepath.Join(d.appLib.GetRoot(), untracked))
		if err != nil {
			return err
		}
	}
	return nil
}

// Updates an existing document in the library
func (d *doccurator) CommandUpdateByPath(filePath string) error {
	switch check := d.appLib.CheckFilePath(mustAbsFilepath(filePath)); check.Status() {
	case library.Modified, library.Touched:
		_, err := d.appLib.UpdateDocumentFromFile(check.ReferencedDocument())
		if err != nil {
			return newCommandError(fmt.Sprintf("update failed: %s", filePath), err)
		}
	case library.Tracked:
		fmt.Fprintf(d.extraOut, "No changes detected: %s\n", filePath)
	case library.Removed:
		return newCommandError(fmt.Sprintf("no file found: %s", filePath), nil)
	case library.Missing:
		return newCommandError(fmt.Sprintf("use retire to accept missing file: %s", filePath), nil)
	case library.Untracked:
		return newCommandError(fmt.Sprintf("path not on record: %s", filePath), nil)
	case library.Obsolete:
		return newCommandError(fmt.Sprintf("path already retired: %s", filePath), nil)
	case library.Error:
		return newCommandError(fmt.Sprintf("path access failed: %s", filePath), check.GetError())
	default:
		panic("unsupported")
	}
	return nil
}

// Marks an existing document as obsolete
func (d *doccurator) CommandRetireByPath(path string) error {
	absPath := mustAbsFilepath(path)
	doc, exists := d.appLib.GetActiveDocumentByPath(absPath)
	if !exists {
		if d.appLib.ObsoleteDocumentExistsForPath(absPath) {
			fmt.Fprintf(d.extraOut, "Already retired: %s\n", doc.PathRelativeToLibraryRoot())
			return nil //i.e. command was a no-op
		}
		return newCommandError(fmt.Sprintf("path to retire not on record: %s", path), nil)
	}
	d.appLib.MarkDocumentAsObsolete(doc)
	return nil
}

// Removes all retired documents from the library completely
func (d *doccurator) CommandForgetAllObsolete() {
	d.appLib.VisitAllRecords(func(doc library.Document) {
		if doc.IsObsolete() {
			d.appLib.ForgetDocument(doc)
		}
	})
}

// Removes a retired document from the library completely
func (d *doccurator) CommandForgetById(id document.Id) error {
	doc, exists := d.appLib.GetDocumentById(id)
	if !exists {
		return newCommandError(fmt.Sprintf("document with ID %s unknown", id), nil)
	}
	if !doc.IsObsolete() {
		return newCommandError(fmt.Sprintf("document to forget (ID %s) not retired", id), nil)
	}
	d.appLib.ForgetDocument(doc)
	return nil
}

// Outputs all library records
func (d *doccurator) CommandDump(excludeRetired bool) {
	fmt.Fprintf(d.extraOut, "Library: %s\n\n", d.appLib.GetRoot())
	count := 0
	d.appLib.VisitAllRecords(func(doc library.Document) {
		if doc.IsObsolete() && excludeRetired {
			return
		}
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
func (d *doccurator) CommandTree(excludeUnchanged bool) error {
	tree := output.NewVisualFileTree(d.appLib.GetRoot() + " [library root]")

	var pathsWithErrors []*library.CheckedPath
	paths, ok := d.appLib.Scan(getSkipperForDbAndPointers(d.libFile))
	for index, checkedPath := range paths {
		prefix := ""
		status := checkedPath.Status()
		if excludeUnchanged && (status == library.Tracked || status == library.Removed) {
			continue
		}
		if status != library.Tracked {
			prefix = fmt.Sprintf("[%s] ", string(status))
		}
		tree.InsertPath(checkedPath.PathRelativeToLibraryRoot(), prefix)
		if status == library.Error {
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
func (d *doccurator) CommandStatus(paths []string) error {
	//TODO [FEATURE]: pair up missing+moved and hide missing
	buckets := make(map[library.PathStatus][]string)

	if len(paths) > 0 {
		fmt.Fprintf(d.extraOut, "Status of %d specified path%s:\n", len(paths), output.PluralS(paths))
	}
	fmt.Fprintln(d.out)

	var errorMessages strings.Builder
	errorCount := 0
	hasChanges := false

	processResult := func(result *library.CheckedPath, absolutePath string) {
		switch status := result.Status(); status {
		case library.Error:
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
		results, _ := d.appLib.Scan(getSkipperForDbAndPointers(d.libFile))
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

func (d *doccurator) CommandStandardizeLocation(id document.Id) error {
	doc, exists := d.appLib.GetDocumentById(id)
	if !exists {
		return newCommandError(fmt.Sprintf("document with ID %s unknown", id), nil)
	}
	changedName, err := doc.RenameToStandardNameFormat()
	if changedName != "" && err == nil {
		fmt.Fprintf(d.extraOut, "Renamed document %s to %s\n", id, changedName)
	}
	return err
}

// Auto pilot adds untracked paths, updates touched & moved paths, and removes duplicates.
//  Modified and missing are not changed but reported.
//  If additional flags are passed modified paths are updated and/or missing paths removed.
//  Unknown paths are reported.
func (d *doccurator) CommandAuto() error {

	return nil
}
