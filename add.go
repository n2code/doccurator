package doccurator

import (
	"fmt"
	"github.com/n2code/doccurator/internal"
	"github.com/n2code/doccurator/internal/document"
	"github.com/n2code/doccurator/internal/library"
	out "github.com/n2code/doccurator/internal/output"
	"os"
)

func (d *doccurator) AddWithId(id document.Id, path string, allowForDuplicateMovedAndObsolete bool, allowEmpty bool) error {
	_, err := d.addSingle(id, path, allowForDuplicateMovedAndObsolete, allowEmpty)
	return err
}

// AddMultiple takes multiple paths and adds one document for each. Flags control dealing with irregular situations.
// If aborted due to an error the library remains clean, i.e. has the same state as before.
func (d *doccurator) AddMultiple(filePaths []string, allowForDuplicateMovedAndObsolete bool, allowEmpty bool, generateMissingIds bool, abortOnError bool) (added []document.Id, err error) {
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
				d.Print(out.Normal, "Skipping bad path (%s): %s\n", filePath, idErr)
				continue
			}
			newId = d.GetFreeId()
		}
		_, addErr := d.addSingle(newId, filePath, allowForDuplicateMovedAndObsolete, allowEmpty)
		if addErr != nil {
			if abortOnError {
				err = addErr
				return
			}
			d.Print(out.Normal, "Skipping failure (%s): %s\n", filePath, addErr)
			continue
		}
		added = append(added, newId)
	}
	return
}

func (d *doccurator) AddAllUntracked(allowForDuplicateMovedAndObsolete bool, recordEmptyFiles bool, generateMissingIds bool, abortOnError bool) (added []document.Id, err error) {
	results, noScanErrors := d.appLib.Scan(d.getScanSkipEvaluators(), nil, true) //read can be skipped because it does not affect correct detection of "untracked" status
	if !noScanErrors {
		d.Print(out.Normal, "Issues during scan: Not all potential candidates accessible\n")
	}

	var untrackedPaths []string
	for _, checked := range results {
		switch checked.Status() {
		case library.Untracked:
			absolute := d.absolutizeAnchored(checked.AnchoredPath())
			if stat, err := os.Stat(absolute); err == nil && stat.Size() == 0 && !recordEmptyFiles { //any error is re-triggered in the subsequent batch add operation
				d.Print(out.Normal, "Skipping untracked (%s): Adding empty files not requested\n", d.displayablePath(absolute, true, false))
				continue
			}
			untrackedPaths = append(untrackedPaths, absolute)
		case library.Error:
			if abortOnError {
				err = fmt.Errorf("encountered uncheckable (%s): %w", checked.AnchoredPath(), checked.GetError())
				return
			}
			d.Print(out.Error, "Skipping uncheckable (%s): %s\n", checked.AnchoredPath(), checked.GetError())
		}
	}

	added, err = d.AddMultiple(untrackedPaths, allowForDuplicateMovedAndObsolete, true, generateMissingIds, abortOnError)
	return
}

// addSingle creates a new document with the given ID and path.
// On error the library remains clean, i.e. has the same state as before.
func (d *doccurator) addSingle(id document.Id, filePath string, allowForDuplicateMovedAndObsolete bool, allowEmpty bool) (library.Document, error) {
	d.Print(out.Verbose, "Adding (%s) ...\n", filePath)
	absoluteFilePath := mustAbsFilepath(filePath)
	if !allowForDuplicateMovedAndObsolete {
		switch check := d.appLib.CheckFilePath(absoluteFilePath, false); check.Status() { //check on add must be accurate hence no performance optimization
		case library.Moved:
			return library.Document{}, fmt.Errorf("document creation prevented: use update to accept move (%s)", filePath)
		case library.Duplicate, library.Obsolete:
			return library.Document{}, fmt.Errorf("document creation prevented: override required to add duplicate/obsolete file (%s)", filePath)
		}
	}
	doc, err := d.appLib.CreateDocument(id)
	if err != nil {
		return library.Document{}, fmt.Errorf("document creation blocked: %w", err)
	}
	err = d.appLib.SetDocumentPath(doc, absoluteFilePath)
	if err != nil {
		d.appLib.ForgetDocument(doc)
		return library.Document{}, fmt.Errorf("document creation impossible: %w", err)
	}
	_, err = d.appLib.UpdateDocumentFromFile(doc)
	if err != nil {
		d.appLib.ForgetDocument(doc)
		return library.Document{}, fmt.Errorf("document creation failed: %w", err)
	}
	if size, _, _ := doc.RecordProperties(); size == 0 && !allowEmpty {
		d.appLib.ForgetDocument(doc)
		return library.Document{}, fmt.Errorf("document creation prevented: file to record is empty (%s): %w", filePath, recordEmptyContentError)
	}
	d.Print(out.Normal, "Added %s: %s\n", id, d.displayablePath(absoluteFilePath, true, false))
	d.Print(out.Verbose, "%s\n", out.Indent(2, doc.String()))
	return doc, nil
}
