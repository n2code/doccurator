package doccurator

import (
	"fmt"
	"github.com/n2code/doccurator/internal"
	"github.com/n2code/doccurator/internal/document"
	"github.com/n2code/doccurator/internal/library"
	out "github.com/n2code/doccurator/internal/output"
)

func (d *doccurator) UpdateByPath(filePath string) error {
	absoluteFilePath := mustAbsFilepath(filePath)
	switch check := d.appLib.CheckFilePath(absoluteFilePath, false); check.Status() { //check on update must be accurate hence no performance optimization
	case library.Moved:
		err := d.appLib.SetDocumentPath(check.ReferencedDocument(), absoluteFilePath)
		internal.AssertNoError(err, "path already checked to be inside library and moved implies no conflicting record")
		fallthrough
	case library.Modified, library.Touched:
		_, err := d.appLib.UpdateDocumentFromFile(check.ReferencedDocument())
		if err != nil {
			return newCommandError(fmt.Sprintf("update failed: %s", filePath), err)
		}
	case library.Tracked:
		d.Print(out.Normal, "No changes detected: %s\n", filePath)
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

func (d *doccurator) RetireByPath(path string) error {
	absPath := mustAbsFilepath(path)
	doc, exists := d.appLib.GetActiveDocumentByPath(absPath)
	if !exists {
		if obsoletes := d.appLib.GetObsoleteDocumentsForPath(absPath); len(obsoletes) > 0 {
			d.Print(out.Normal, "Already retired: %s\n", path)
			return nil //i.e. command was a no-op
		}
		return newCommandError(fmt.Sprintf("path to retire not on record: %s", path), nil)
	}
	d.appLib.MarkDocumentAsObsolete(doc)
	return nil
}

func (d *doccurator) ForgetAllObsolete() {
	d.appLib.VisitAllRecords(func(doc library.Document) {
		if doc.IsObsolete() {
			d.appLib.ForgetDocument(doc)
		}
	})
}

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

func (d *doccurator) StandardizeLocation(id document.Id) error {
	doc, exists := d.appLib.GetDocumentById(id)
	if !exists {
		return newCommandError(fmt.Sprintf("document with ID %s unknown", id), nil)
	}
	oldRelPath := doc.AnchoredPath()
	changedName, err, rollback := doc.RenameToStandardNameFormat(false)
	if changedName != "" && err == nil {
		d.Print(out.Normal, "Renamed document %s (%s) to %s\n", id, oldRelPath, changedName)
	}
	d.rollbackLog = append(d.rollbackLog, rollback) //rollback is no-op on error
	return err
}
