package doccurator

import (
	"fmt"
	. "github.com/n2code/doccurator/internal"
	"os"
	"path/filepath"
	"strings"

	"github.com/n2code/doccurator/internal/document"
	"github.com/n2code/doccurator/internal/library"
	"github.com/n2code/doccurator/internal/output"
)

func (d *doccurator) isLibFilePath(path string) bool {
	return path == d.libFile
}

func (d *doccurator) Add(id document.Id, filePath string, allowForDuplicateMovedAndObsolete bool) error {
	return d.addSingle(id, filePath, allowForDuplicateMovedAndObsolete)
}

// addSingle creates a new document with the given ID and path.
// On error the library remains clean, i.e. has the same state as before.
func (d *doccurator) addSingle(id document.Id, filePath string, allowForDuplicateMovedAndObsolete bool) error { //TODO switch signature to command error
	absoluteFilePath := mustAbsFilepath(filePath)
	if !allowForDuplicateMovedAndObsolete {
		switch check := d.appLib.CheckFilePath(absoluteFilePath, false); check.Status() { //check on add must be accurate hence no performance optimization
		case library.Moved:
			return newCommandError(fmt.Sprintf("document creation prevented: use update to accept move (%s)", filePath), nil)
		case library.Duplicate, library.Obsolete:
			return newCommandError(fmt.Sprintf("document creation prevented: override required to add duplicate/obsolete file (%s)", filePath), nil)
		}
	}
	doc, err := d.appLib.CreateDocument(id)
	if err != nil {
		return newCommandError("document creation blocked", err)
	}
	err = d.appLib.SetDocumentPath(doc, absoluteFilePath)
	if err != nil {
		d.appLib.ForgetDocument(doc)
		return newCommandError("document creation impossible", err)
	}
	_, err = d.appLib.UpdateDocumentFromFile(doc)
	if err != nil {
		d.appLib.ForgetDocument(doc)
		return newCommandError("document creation failed", err)
	}
	fmt.Fprintf(d.extraOut, "Added %s: %s\n", id, doc.PathRelativeToLibraryRoot())
	return nil
}

// AddMultiple takes multiple paths and adds one document for each. Flags control dealing with irregular situations.
// If aborted due to an error the library remains clean, i.e. has the same state as before.
func (d *doccurator) AddMultiple(filePaths []string, allowForDuplicateMovedAndObsolete bool, generateMissingIds bool, abortOnError bool) (added []document.Id, err error) {
	defer func() {
		if err != nil { //abort case only, otherwise errors are printed but not returned
			//rollback adding of documents
			for _, id := range added {
				forgetErr := d.ForgetById(id, true)
				AssertNoError(forgetErr, "just created document has to exist and is obsolete")
			}
			added = nil
		}
	}()

	for _, filePath := range filePaths {
		newId, idErr := ExtractIdFromStandardizedFilename(filePath)
		if idErr != nil {
			if !generateMissingIds {
				if abortOnError {
					err = fmt.Errorf(`bad path %s: (%w)`, filePath, idErr)
					return
				}
				fmt.Fprintf(d.extraOut, "Skipping bad path (%s): %s\n", filePath, idErr)
				continue
			}
			newId = d.GetFreeId()
		}
		addErr := d.addSingle(newId, filePath, allowForDuplicateMovedAndObsolete)
		if addErr != nil {
			if abortOnError {
				err = addErr
				return
			}
			fmt.Fprintf(d.extraOut, "Skipping failure (%s): %s\n", filePath, addErr)
			continue
		}
		added = append(added, newId)
	}
	return
}

func (d *doccurator) AddAllUntracked(allowForDuplicateMovedAndObsolete bool, generateMissingIds bool, abortOnError bool) (added []document.Id, err error) {
	results, noScanErrors := d.appLib.Scan(d.isLibFilePath, true) //read can be skipped because it does not affect correct detection of "untracked" status
	if !noScanErrors {
		fmt.Fprint(d.extraOut, "Issues during scan: Not all potential candidates accessible\n")
	}

	var untrackedRootRelativePaths []string
	for _, checked := range results {
		switch checked.Status() {
		case library.Untracked:
			untrackedRootRelativePaths = append(untrackedRootRelativePaths, checked.PathRelativeToLibraryRoot())
		case library.Error:
			if abortOnError {
				err = fmt.Errorf("encountered uncheckable (%s): %w", checked.PathRelativeToLibraryRoot(), checked.GetError())
				return
			}
			fmt.Fprintf(d.extraOut, "Skipping uncheckable (%s): %s\n", checked.PathRelativeToLibraryRoot(), checked.GetError())
		}
	}

	added, err = d.AddMultiple(untrackedRootRelativePaths, allowForDuplicateMovedAndObsolete, generateMissingIds, abortOnError)
	return
}

// Updates an existing document in the library
func (d *doccurator) UpdateByPath(filePath string) error {
	absoluteFilePath := mustAbsFilepath(filePath)
	switch check := d.appLib.CheckFilePath(absoluteFilePath, false); check.Status() { //check on update must be accurate hence no performance optimization
	case library.Moved:
		err := d.appLib.SetDocumentPath(check.ReferencedDocument(), absoluteFilePath)
		AssertNoError(err, "path already checked to be inside library and moved implies no conflicting record")
		fallthrough
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
	case library.Untracked, library.Duplicate:
		return newCommandError(fmt.Sprintf("path not on record: %s", filePath), nil)
	case library.Obsolete:
		return newCommandError(fmt.Sprintf("path already retired: %s", filePath), nil)
	case library.Error:
		return newCommandError(fmt.Sprintf("path access failed: %s", filePath), check.GetError())
	}
	return nil
}

// Marks an existing document as obsolete
func (d *doccurator) RetireByPath(path string) error {
	absPath := mustAbsFilepath(path)
	doc, exists := d.appLib.GetActiveDocumentByPath(absPath)
	if !exists {
		if d.appLib.ObsoleteDocumentExistsForPath(absPath) {
			fmt.Fprintf(d.extraOut, "Already retired: %s\n", path)
			return nil //i.e. command was a no-op
		}
		return newCommandError(fmt.Sprintf("path to retire not on record: %s", path), nil)
	}
	d.appLib.MarkDocumentAsObsolete(doc)
	return nil
}

// Removes all retired documents from the library completely
func (d *doccurator) ForgetAllObsolete() {
	d.appLib.VisitAllRecords(func(doc library.Document) {
		if doc.IsObsolete() {
			d.appLib.ForgetDocument(doc)
		}
	})
}

// Removes a retired document from the library completely
func (d *doccurator) ForgetById(id document.Id, forceRetire bool) error {
	doc, exists := d.appLib.GetDocumentById(id)
	if !exists {
		return newCommandError(fmt.Sprintf("document with ID %s unknown", id), nil)
	}
	if !doc.IsObsolete() {
		if !forceRetire {
			return newCommandError(fmt.Sprintf("document to forget (ID %s) not retired", id), nil)
		}
		d.appLib.MarkDocumentAsObsolete(doc)
	}
	d.appLib.ForgetDocument(doc)
	return nil
}

// Outputs all library records
func (d *doccurator) PrintAllRecords(excludeRetired bool) {
	fmt.Fprintf(d.extraOut, "Library: %s\n\n\n", d.appLib.GetRoot())
	count := 0
	d.appLib.VisitAllRecords(func(doc library.Document) {
		if doc.IsObsolete() && excludeRetired {
			return
		}
		fmt.Fprintf(d.out, "%s\n\n", doc)
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
func (d *doccurator) PrintTree(excludeUnchanged bool) error {
	tree := output.NewVisualFileTree(d.appLib.GetRoot() + " [library root]")

	var pathsWithErrors []*library.CheckedPath
	paths, ok := d.appLib.Scan(d.isLibFilePath, d.optimizedFsAccess) //full scan may optimize performance if allowed to
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
	errorCount := len(pathsWithErrors)

	fmt.Fprint(d.out, tree.Render())

	//TODO [FEATURE]: coloring
	if !ok {
		var msg strings.Builder
		fmt.Fprintf(&msg, "%d scanning %s occurred:\n", errorCount, output.Plural(errorCount, "error", "errors"))
		for _, errorPath := range pathsWithErrors {
			fmt.Fprintf(&msg, "@%s: %s\n", errorPath.PathRelativeToLibraryRoot(), errorPath.GetError())
		}
		return fmt.Errorf(msg.String())
	} else {
		return nil
	}
}

// Queries the given [possibly relative] paths about their affiliation and state with respect to the library
func (d *doccurator) PrintStatus(paths []string) error {
	//TODO [FEATURE]: pair up missing+moved and hide missing
	buckets := make(map[library.PathStatus][]string)

	if len(paths) > 0 {
		fmt.Fprintf(d.extraOut, "Status of %d specified %s:\n", len(paths), output.Plural(paths, "path", "paths"))
	}
	fmt.Fprintln(d.out)

	var errorMessages strings.Builder
	errorCount := 0
	hasChanges := false
	explicitQueryForPaths := len(paths) > 0

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

	if explicitQueryForPaths {
		for _, path := range paths {
			abs := mustAbsFilepath(path)
			result := d.appLib.CheckFilePath(abs, false) //explicit status query must not sacrifice correctness for performance
			processResult(&result, abs)
		}
	} else {
		results, _ := d.appLib.Scan(d.isLibFilePath, d.optimizedFsAccess) //full scan may optimize performance if allowed to
		for _, result := range results {
			processResult(&result, filepath.Join(d.appLib.GetRoot(), result.PathRelativeToLibraryRoot()))
		}
	}

	//TODO [FEATURE]: coloring
	for status, bucket := range buckets {
		if !status.RepresentsChange() && !explicitQueryForPaths {
			continue //to hide unchanged files when no explicit paths are queried
		}
		fmt.Fprintf(d.out, " %s (%d %s)\n", status, len(bucket), output.Plural(bucket, "file", "files"))
		for _, path := range bucket {
			fmt.Fprintf(d.out, "  [%s] %s\n", string(rune(status)), path)
		}
		fmt.Fprintln(d.out)
	}
	if errorCount > 0 {
		fmt.Fprintf(d.out, " %s occurred:\n%s\n", output.Plural(errorCount, "Error", "Errors"), errorMessages.String()) //not on stderr because it was explicitly queried
	} else if hasChanges == false && len(paths) == 0 {
		fmt.Fprint(d.out, " Library in sync with all records.\n\n")
	}
	return nil
}

func (d *doccurator) StandardizeLocation(id document.Id) error {
	doc, exists := d.appLib.GetDocumentById(id)
	if !exists {
		return newCommandError(fmt.Sprintf("document with ID %s unknown", id), nil)
	}
	oldRelPath := doc.PathRelativeToLibraryRoot()
	changedName, err, rollback := doc.RenameToStandardNameFormat()
	if changedName != "" && err == nil {
		fmt.Fprintf(d.extraOut, "Renamed document %s (%s) to %s\n", id, oldRelPath, changedName)
		d.rollbackLog = append(d.rollbackLog, rollback)
	}
	return err
}

func (d *doccurator) SearchByIdPart(part string) (results []SearchResult) {
	partInUpper := strings.ToUpper(part)
	d.appLib.VisitAllRecords(func(doc library.Document) {
		if id := doc.Id(); strings.Contains(id.String(), partInUpper) {
			absolute := filepath.Join(d.appLib.GetRoot(), doc.PathRelativeToLibraryRoot())
			relative := mustRelFilepathToWorkingDir(absolute)
			results = append(results, SearchResult{
				Id:           id,
				RelativePath: relative,
				StatusText:   d.appLib.CheckFilePath(absolute, d.optimizedFsAccess).Status().String()})
		}
	})
	return
}

func (d *doccurator) PrintRecord(id document.Id) {
	doc, exists := d.appLib.GetDocumentById(id)
	if exists {
		fmt.Fprintln(d.out, doc)
	}
}

// InteractiveTidy guides through library updates and file system changes that can be executed automatically:
//  It adds untracked paths, updates touched & moved paths, and removes duplicates.
//  Modified and missing are not changed but reported.
//  If additional flags are passed modified paths are updated and/or missing paths removed.
//  Unknown paths are reported.
func (d *doccurator) InteractiveTidy(choice RequestChoice, removeWaste bool) error {
	fmt.Fprint(d.verboseOut, "Tidying up library...\n")
	paths, _ := d.appLib.Scan(func(absoluteFilePath string) bool {
		return false
	}, d.optimizedFsAccess)

	buckets := make(map[library.PathStatus][]*library.CheckedPath)
	for i, path := range paths {
		buckets[path.Status()] = append(buckets[path.Status()], &paths[i])
	}

	var deletionCommitQueue []func() error

	for _, status := range []library.PathStatus{library.Touched, library.Moved, library.Modified, library.Obsolete, library.Duplicate} {
		count := len(buckets[status])
		if count == 0 {
			continue
		}
		var declarationSingle, declarationMultiple, promptMassProcessing, question, subject, pastParticiple string
		switch status {
		case library.Touched, library.Moved, library.Modified:
			declarationSingle = "1 document on record has its file %s.\n"         // touched/moved/modified
			declarationMultiple = "%d documents on record have their files %s.\n" // <count> + touched/moved/modified
			promptMassProcessing = "Update %s records?"                           // touched/moved/modified
			question = "Update record?"
			subject = "document"
			pastParticiple = "updated"
		case library.Obsolete, library.Duplicate:
			if !removeWaste {
				continue
			}
			declarationSingle = "1 file present has %s content.\n"      // duplicate/obsolete
			declarationMultiple = "%d files present have %s content.\n" // <count> + duplicate/obsolete
			promptMassProcessing = "Delete %s files?"                   // duplicate/obsolete
			question = "Delete file?"
			subject = "file"
			pastParticiple = "marked for deletion"
		}

		upperStatus := strings.ToUpper(status.String())
		lowerStatus := strings.ToLower(status.String())
		var doChange, decideIndividually bool
		{
			fmt.Fprintf(d.extraOut, "\n")
			if count == 1 {
				fmt.Fprintf(d.verboseOut, declarationSingle, upperStatus)
				decideIndividually = true
			} else {
				fmt.Fprintf(d.verboseOut, declarationMultiple, count, upperStatus)
				switch choice(fmt.Sprintf(promptMassProcessing, lowerStatus), []string{"All", "Decide individually", "Skip"}, true) {
				case "All":
					decideIndividually = false
					doChange = true
				case "Decide individually":
					decideIndividually = true
				case "Skip":
					decideIndividually = false
					doChange = false
				case ChoiceAborted:
					return fmt.Errorf("<ERROR: PROMPT ABORTED>")
				}
			}
		}

		changeCount := 0
	NextChange:
		for _, path := range buckets[status] {
			absolute := filepath.Join(d.appLib.GetRoot(), path.PathRelativeToLibraryRoot())
			displayPath := mustRelFilepathToWorkingDir(absolute)

			if decideIndividually {
				switch choice(fmt.Sprintf("@%s [%s] - %s", displayPath, lowerStatus, question), []string{"Yes", "No"}, true) {
				case "Yes":
					doChange = true
				case "No":
					doChange = false
				case ChoiceAborted:
					return fmt.Errorf("<ERROR: PROMPT ABORTED>")
				}
			}

			if doc := path.ReferencedDocument(); doChange {
				switch status {
				case library.Moved:
					err := d.appLib.SetDocumentPath(doc, absolute)
					if err != nil {
						fmt.Fprintf(d.errOut, "update failed (%s): %s\n", displayPath, err)
						continue NextChange
					}
					fallthrough
				case library.Touched, library.Modified:
					_, err := d.appLib.UpdateDocumentFromFile(doc)
					if err != nil {
						fmt.Fprintf(d.errOut, "update failed (%s): %s\n", displayPath, err)
						continue NextChange
					} else {
						fmt.Fprintf(d.extraOut, "@%s [%s] - Updated %s.\n", displayPath, lowerStatus, doc.Id())
					}
				case library.Obsolete, library.Duplicate:
					tempDir, err := os.MkdirTemp("", "doccurator-tidy-delete-staging-*")
					if err != nil {
						fmt.Fprintf(d.errOut, "deletion preparation failed (%s): %s\n", displayPath, err)
						continue NextChange
					}
					deleteStagingDir := func(stagingDir string) func() error {
						return func() error {
							if err := os.RemoveAll(stagingDir); err != nil {
								return fmt.Errorf("could not delete temporary staging directory (%s): %w", stagingDir, err)
							}
							return nil
						}
					}(tempDir)

					backup := filepath.Join(tempDir, filepath.Base(absolute))
					if err := os.Rename(absolute, backup); err != nil {
						fmt.Fprintf(d.errOut, "deletion failed (%s): %s\n", displayPath, err)
						if err := deleteStagingDir(); err != nil {
							fmt.Fprintf(d.errOut, "%s\n", err)
						}
						continue NextChange
					}
					deletionCommitQueue = append(deletionCommitQueue, deleteStagingDir)

					d.rollbackLog = append(d.rollbackLog, func(source string, target string, stagingDir string) func() error {
						return func() error {
							if err := os.Rename(source, target); err != nil {
								return err
							}
							return os.RemoveAll(stagingDir)
						}
					}(backup, absolute, tempDir))

					fmt.Fprintf(d.extraOut, "@%s [%s] - Marked for delete.\n", displayPath, lowerStatus)
				}
				changeCount++
			} else {
				fmt.Fprintf(d.extraOut, "@%s - Skipped.\n", displayPath)
			}
		}
		fmt.Fprintf(d.extraOut, "%d %s %s %s.\n", changeCount, upperStatus, output.Plural(changeCount, subject, subject+"s"), pastParticiple)
	}

	fmt.Fprint(d.extraOut, "\n")

	if len(deletionCommitQueue) > 0 {
		fmt.Fprintf(d.verboseOut, "Committing deletions...\n")
		for _, commitDelete := range deletionCommitQueue {
			if err := commitDelete(); err != nil {
				//errors are reported but do not constitute an overall failure as a rollback would not work and removal from the original directory is already complete by now
				// => failure is only possible theoretically anyway as the application should be able to remove the staging directory it has just created
				fmt.Fprintf(d.errOut, "deletion has leftovers: %s\n", err)
			}
		}
	}

	fmt.Fprint(d.verboseOut, "Tidy operation complete.\n")
	return nil
}
