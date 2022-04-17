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
			return fmt.Errorf("update failed: %s: %w", filePath, err)
		}
	case library.Tracked:
		d.Print(out.Normal, "No changes detected: %s\n", filePath)
	case library.Removed:
		return fmt.Errorf("no file found: %s", filePath)
	case library.Missing:
		return fmt.Errorf("use retire to accept missing file: %s", filePath)
	case library.Untracked, library.Duplicate:
		return fmt.Errorf("path not on record: %s", filePath)
	case library.Obsolete:
		return fmt.Errorf("path already retired: %s", filePath)
	case library.Error:
		return fmt.Errorf("path access failed: %s: %w", filePath, check.GetError())
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
		return fmt.Errorf("path to retire not on record: %s", path)
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
		return fmt.Errorf("document with ID %s unknown", id)
	}
	if !doc.IsObsolete() {
		if !forceRetire {
			return fmt.Errorf("document to forget (ID %s) not retired", id)
		}
		d.appLib.MarkDocumentAsObsolete(doc)
	}
	d.appLib.ForgetDocument(doc)
	return nil
}

func (d *doccurator) StandardizeLocation(id document.Id) error {
	doc, exists := d.appLib.GetDocumentById(id)
	if !exists {
		return fmt.Errorf("document with ID %s unknown", id)
	}
	oldRelPath := doc.AnchoredPath()
	changedName, err, rollback := doc.RenameToStandardNameFormat(false)
	if changedName != "" && err == nil {
		d.Print(out.Normal, "Renamed document %s (%s) to %s\n", id, oldRelPath, changedName)
	}
	d.rollbackLog = append(d.rollbackLog, rollback) //rollback is no-op on error
	return err
}
