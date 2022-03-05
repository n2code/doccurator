package doccurator

import (
	"fmt"
	"github.com/n2code/doccurator/internal"
	"github.com/n2code/doccurator/internal/document"
	"github.com/n2code/doccurator/internal/library"
)

func (d *doccurator) Add(id document.Id, filePath string, allowForDuplicateMovedAndObsolete bool) error {
	_, err := d.addSingle(id, filePath, allowForDuplicateMovedAndObsolete)
	return err
}

// AddMultiple takes multiple paths and adds one document for each. Flags control dealing with irregular situations.
// If aborted due to an error the library remains clean, i.e. has the same state as before.
func (d *doccurator) AddMultiple(filePaths []string, allowForDuplicateMovedAndObsolete bool, generateMissingIds bool, abortOnError bool) (added []document.Id, err error) {
	defer func() {
		if err != nil { //abort case only, otherwise errors are printed but not returned
			//rollback adding of documents
			for _, id := range added {
				forgetErr := d.ForgetById(id, true)
				internal.AssertNoError(forgetErr, "just created document has to exist and is obsolete")
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
		_, addErr := d.addSingle(newId, filePath, allowForDuplicateMovedAndObsolete)
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
			fmt.Fprintf(d.errOut, "Skipping uncheckable (%s): %s\n", checked.PathRelativeToLibraryRoot(), checked.GetError())
		}
	}

	added, err = d.AddMultiple(untrackedRootRelativePaths, allowForDuplicateMovedAndObsolete, generateMissingIds, abortOnError)
	return
}

// addSingle creates a new document with the given ID and path.
// On error the library remains clean, i.e. has the same state as before.
func (d *doccurator) addSingle(id document.Id, filePath string, allowForDuplicateMovedAndObsolete bool) (library.Document, error) { //TODO switch signature to command error
	absoluteFilePath := mustAbsFilepath(filePath)
	if !allowForDuplicateMovedAndObsolete {
		switch check := d.appLib.CheckFilePath(absoluteFilePath, false); check.Status() { //check on add must be accurate hence no performance optimization
		case library.Moved:
			return library.Document{}, newCommandError(fmt.Sprintf("document creation prevented: use update to accept move (%s)", filePath), nil)
		case library.Duplicate, library.Obsolete:
			return library.Document{}, newCommandError(fmt.Sprintf("document creation prevented: override required to add duplicate/obsolete file (%s)", filePath), nil)
		}
	}
	doc, err := d.appLib.CreateDocument(id)
	if err != nil {
		return library.Document{}, newCommandError("document creation blocked", err)
	}
	err = d.appLib.SetDocumentPath(doc, absoluteFilePath)
	if err != nil {
		d.appLib.ForgetDocument(doc)
		return library.Document{}, newCommandError("document creation impossible", err)
	}
	_, err = d.appLib.UpdateDocumentFromFile(doc)
	if err != nil {
		d.appLib.ForgetDocument(doc)
		return library.Document{}, newCommandError("document creation failed", err)
	}
	fmt.Fprintf(d.extraOut, "Added %s: %s\n", id, doc.PathRelativeToLibraryRoot())
	return doc, nil
}
