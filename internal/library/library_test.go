package library

import (
	"github.com/n2code/doccurator/internal/document"
	"github.com/n2code/ndocid"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetAnchoredPath(t *testing.T) {
	lib := library{rootPath: "/dummy/root"}

	assertRelPath := func(full string, expRel string) {
		actRel, inLibrary := lib.getAnchoredPath(full)
		if inLibrary != true {
			t.Error("expected detection to determine that path", full, "is inside library")
		}
		if actRel != expRel {
			t.Error("expected", expRel, "but got", actRel)
		}
	}

	assertNotInLib := func(full string) {
		actRel, inLibrary := lib.getAnchoredPath(full)
		if inLibrary != false {
			t.Error("expected detection to determine that path", full, "is outside library")
		}
		if !strings.HasPrefix(actRel, ".."+string(filepath.Separator)) {
			t.Error("expected path", full, "to start with ../ but got", actRel)
		}
	}

	assertRelPath("/dummy/root/file", "file")
	assertRelPath("/dummy/root/a/b/file", "a/b/file")
	assertNotInLib("/dummy/different/a/b/file")
	assertNotInLib("/root_file")
}

func TestDocumentCreation(t *testing.T) {
	//GIVEN
	lib := NewLibrary()
	maxId := document.Id(1<<64 - 1)
	regularId := document.Id(42)
	minId := document.Id(1)
	illegalZeroId := document.Id(0)

	//WHEN
	regularDoc, err := lib.CreateDocument(regularId)
	//THEN
	if err != nil || regularDoc == (Document{}) {
		t.Fatal("creation with regular ID failed")
	}

	//WHEN
	minDoc, err := lib.CreateDocument(minId)
	//THEN
	if err != nil || minDoc == (Document{}) {
		t.Fatal("creation with min. ID failed")
	}

	//WHEN
	maxDoc, err := lib.CreateDocument(maxId)
	//THEN
	if err != nil || maxDoc == (Document{}) {
		t.Fatal("creation with max. ID failed")
	}

	//WHEN
	docNone, err := lib.CreateDocument(regularId)
	//THEN
	if err == nil || docNone != (Document{}) {
		t.Fatal("creation not rejected as expected")
	}

	//WHEN
	forbiddenDoc, err := lib.CreateDocument(illegalZeroId)
	//THEN
	if err == nil || forbiddenDoc != (Document{}) {
		t.Fatal("creation with illegal ID (0) not blocked")
	}
}

func TestGetDocumentById(t *testing.T) {
	//GIVEN
	lib := NewLibrary()
	const mainDocumentId document.Id = 42

	t.Run("UnrecordedDocument", func(Test *testing.T) {
		//WHEN
		unrecordedDoc, exists := lib.GetDocumentById(mainDocumentId)
		//THEN
		if exists || unrecordedDoc != (Document{}) {
			Test.Fatal("unrecorded document found somehow")
		}
	})

	//GIVEN
	recordedDoc, _ := lib.CreateDocument(mainDocumentId)

	t.Run("QueryActiveDocument", func(Test *testing.T) {
		//WHEN
		queriedDoc, exists := lib.GetDocumentById(mainDocumentId)
		//THEN
		if !exists || queriedDoc != recordedDoc {
			Test.Fatal("retrieval of active failed")
		}
	})

	//GIVEN
	lib.MarkDocumentAsObsolete(recordedDoc)

	t.Run("QueryObsoleteDocument", func(Test *testing.T) {
		//WHEN
		queriedDoc, stillExists := lib.GetDocumentById(mainDocumentId)
		//THEN
		if !stillExists || queriedDoc != recordedDoc {
			Test.Fatal("retrieval of obsolete failed")
		}
	})

	//GIVEN
	lib.ForgetDocument(recordedDoc)

	t.Run("QueryForgottenDocument", func(Test *testing.T) {
		//WHEN
		queriedDocument, stillExists := lib.GetDocumentById(mainDocumentId)
		//THEN
		if stillExists || queriedDocument != (Document{}) {
			Test.Fatal("forgotten document still accessible")
		}
	})
}

func setupLibraryInTemp(t *testing.T) (tempRootDir string, library Api) {
	tempRootDir = t.TempDir()
	library = NewLibrary()
	library.SetRoot(tempRootDir)
	return
}

func writeFile(path string, content string) {
	os.WriteFile(path, []byte(content), fs.ModePerm)
}

func TestDocumentModification(t *testing.T) {
	//GIVEN
	tempDir, lib := setupLibraryInTemp(t)
	doc, _ := lib.CreateDocument(42)
	filePath := filepath.Join(tempDir, "file_for_update")
	lib.SetDocumentPath(doc, filePath)
	writeFile(filePath, "any")

	t.Run("InitialDocument", func(Test *testing.T) {
		//WHEN
		changed, err := lib.UpdateDocumentFromFile(doc) //will report changed because record is initial (i.e. checksum does not match zero length)
		//THEN
		if err != nil {
			Test.Fatal("update reported error")
		}
		if changed == false {
			Test.Fatal("update did not report change")
		}
	})

	t.Run("UnchangedDocument", func(Test *testing.T) {
		//WHEN
		changed, err := lib.UpdateDocumentFromFile(doc)
		//THEN
		if err != nil {
			t.Fatal("repeated update reported error")
		}
		if changed == true {
			t.Fatal("repeated update reported change")
		}
	})

	//GIVEN
	writeFile(filePath, "modification")

	t.Run("ModifiedDocument", func(Test *testing.T) {
		//WHEN
		changed, err := lib.UpdateDocumentFromFile(doc)
		//THEN
		if err != nil {
			Test.Fatal("update after modification reported error")
		}
		if changed == false {
			Test.Fatal("update after modification did not report change")
		}
	})

	//GIVEN
	os.Chmod(filePath, 0o333)

	t.Run("InaccessibleDocument", func(Test *testing.T) {
		//WHEN
		changed, err := lib.UpdateDocumentFromFile(doc)
		//THEN
		if err == nil {
			t.Fatal("update did not report error")
		}
		if changed == true {
			t.Fatal("update reported change although error occurred")
		}
	})

	//GIVEN
	replacement, _ := lib.CreateDocument(99)

	t.Run("ConflictingDocumentPath", func(Test *testing.T) {
		//WHEN
		err := lib.SetDocumentPath(replacement, filePath) //same as for other document
		//THEN
		if err == nil {
			t.Fatal("path setting not refused because of known active path")
		}
	})
}

func TestActiveVersusObsoleteOrchestration(t *testing.T) {
	//GIVEN
	tempRootDir, lib := setupLibraryInTemp(t)
	filePath := filepath.Join(tempRootDir, "file_for_lifecycle")
	writeFile(filePath, "content")
	const mainDocumentId document.Id = 42

	t.Run("UnrecordedDocument", func(Test *testing.T) {
		//WHEN
		unrecordedDoc, exists := lib.GetActiveDocumentByPath(filePath)
		//THEN
		if unrecordedDoc != (Document{}) || exists {
			Test.Fatal("unrecorded document not rejected")
		}
	})

	t.Run("DocumentOutsideLibrary", func(Test *testing.T) {
		//WHEN
		outsideDoc, exists := lib.GetActiveDocumentByPath(filepath.Join(tempRootDir, "../file_outside"))
		//THEN
		if outsideDoc != (Document{}) || exists {
			t.Fatal("document outside library path not rejected")
		}
	})

	//GIVEN
	recordedDoc, _ := lib.CreateDocument(mainDocumentId)
	lib.SetDocumentPath(recordedDoc, filePath)
	//note: real file not yet read!

	t.Run("QueryFreshDocument", func(Test *testing.T) {
		//WHEN
		queriedDoc, exists := lib.GetActiveDocumentByPath(filePath)
		//THEN
		if queriedDoc != recordedDoc || !exists {
			Test.Fatal("retrieval failed")
		}
	})

	//GIVEN
	lib.UpdateDocumentFromFile(recordedDoc)

	t.Run("QueryTrackedDocument", func(Test *testing.T) {
		//WHEN
		queriedDoc, exists := lib.GetActiveDocumentByPath(filePath)
		//THEN
		if queriedDoc != recordedDoc || !exists {
			Test.Fatal("retrieval failed")
		}
	})

	t.Run("VerifyTrackedDocumentIsActive", func(Test *testing.T) {
		//WHEN
		isObsolete := recordedDoc.IsObsolete()
		//THEN
		if isObsolete {
			Test.Fatal("already obsolete")
		}
	})

	t.Run("VerifyNoObsoleteVersionExistedAtActivePath", func(Test *testing.T) {
		//WHEN
		obsoletes := lib.GetObsoleteDocumentsForPath(filePath)
		//THEN
		if len(obsoletes) > 0 {
			Test.Fatal("obsolete version reported to exist already")
		}
	})

	//GIVEN
	lib.MarkDocumentAsObsolete(recordedDoc)

	t.Run("DeclaredDocumentObsolete", func(Test *testing.T) {
		//WHEN
		isObsolete := recordedDoc.IsObsolete()
		queriedObsoleteDocument, existsAsActive := lib.GetActiveDocumentByPath(filePath)
		obsoletes := lib.GetObsoleteDocumentsForPath(filePath)
		//THEN
		if !isObsolete {
			Test.Fatal("not obsolete")
		}
		if existsAsActive || queriedObsoleteDocument != (Document{}) {
			Test.Fatal("still reported as existing / found although obsoleted")
		}
		if len(obsoletes) == 0 {
			Test.Fatal("obsolete version not known to exist for path")
		}
	})

	//GIVEN
	os.Remove(filePath)

	t.Run("VerifyRemovedDocumentStillObsolete", func(Test *testing.T) {
		//WHEN
		isObsolete := recordedDoc.IsObsolete()
		queriedObsoleteDocument, existsAsActive := lib.GetActiveDocumentByPath(filePath)
		obsoletes := lib.GetObsoleteDocumentsForPath(filePath)
		//THEN
		if !isObsolete {
			Test.Fatal("removed document is not obsolete")
		}
		if existsAsActive || queriedObsoleteDocument != (Document{}) {
			Test.Fatal("still reported as existing / found although obsoleted and removed")
		}
		if len(obsoletes) == 0 {
			Test.Fatal("obsolete version not known to exist for removed path")
		}
	})

	//GIVEN
	writeFile(filePath, "different")
	differentDoc, _ := lib.CreateDocument(999)
	lib.SetDocumentPath(differentDoc, filePath)
	lib.UpdateDocumentFromFile(differentDoc)

	t.Run("VerifyCoexistenceOfReplacementAndObsoletePath", func(Test *testing.T) {
		//WHEN
		oldOneIsObsolete := recordedDoc.IsObsolete()
		newOneIsObsolete := differentDoc.IsObsolete()
		queriedDocument, activeOneExists := lib.GetActiveDocumentByPath(filePath)
		obsoletes := lib.GetObsoleteDocumentsForPath(filePath)
		//THEN
		if !oldOneIsObsolete {
			Test.Fatal("old document not obsolete")
		}
		if newOneIsObsolete {
			Test.Fatal("new document is obsolete")
		}
		if !activeOneExists || queriedDocument != differentDoc {
			Test.Fatal("different new document not found")
		}
		if len(obsoletes) == 0 {
			Test.Fatal("obsolete version not known to exist for path which also has active")
		}
	})

	//GIVEN
	lib.ForgetDocument(recordedDoc)

	t.Run("VerifyPurgeOfObsoleteVersion", func(Test *testing.T) {
		//WHEN
		queriedDocument, activeOneStillExists := lib.GetActiveDocumentByPath(filePath)
		obsoletes := lib.GetObsoleteDocumentsForPath(filePath)
		//THEN
		if !activeOneStillExists || queriedDocument != differentDoc {
			Test.Fatal("new document not found")
		}
		if len(obsoletes) > 0 {
			Test.Fatal("obsolete version not forgotten")
		}
	})
}

func TestVisitRecordsAndPrint(t *testing.T) {
	//GIVEN
	lib := NewLibrary()
	imaginaryRoot := "/imaginary"
	lib.SetRoot(imaginaryRoot)
	id932229, anchoredPathA := document.Id(1), "A.932229.ndoc"
	id97322X, anchoredPathB := document.Id(13), "B.97322X.ndoc"
	id94722N, anchoredPathC := document.Id(42), "C.94722N.ndoc"

	//WHEN
	docA, _ := lib.CreateDocument(id932229)
	docB, _ := lib.CreateDocument(id97322X)
	docC, _ := lib.CreateDocument(id94722N)
	lib.SetDocumentPath(docA, filepath.Join(imaginaryRoot, anchoredPathA))
	lib.SetDocumentPath(docB, filepath.Join(imaginaryRoot, anchoredPathB))
	lib.SetDocumentPath(docC, filepath.Join(imaginaryRoot, anchoredPathC))
	lib.MarkDocumentAsObsolete(docB)
	lib.ForgetDocument(docC)

	//THEN
	var recordPrintout strings.Builder
	lib.VisitAllRecords(func(doc Document) {
		recordPrintout.WriteString(doc.String())
		recordPrintout.WriteRune('\n')
	})
	if !strings.Contains(recordPrintout.String(), anchoredPathA) ||
		!strings.Contains(recordPrintout.String(), anchoredPathB) ||
		strings.Contains(recordPrintout.String(), anchoredPathC) { //C must not be contained because it should have been forgotten
		t.Fatal("record printout unexpected:\n" + recordPrintout.String())
	}
}

func TestNameStandardization(t *testing.T) {
	//GIVEN
	const mainDocumentId = 42
	ndocid.EncodeUint64(mainDocumentId)
	tempDir, lib := setupLibraryInTemp(t)
	doc, _ := lib.CreateDocument(mainDocumentId)
	filePath := filepath.Join(tempDir, "archive.tar.gz")
	writeFile(filePath, "dummy")
	lib.SetDocumentPath(doc, filePath)
	lib.UpdateDocumentFromFile(doc)

	//WHEN
	newFilename, err, _ := doc.RenameToStandardNameFormat(false)

	//THEN
	if err != nil {
		t.Fatal("got unexpected error: ", err)
	}
	if newFilename != "archive.tar.gz.94722N.ndoc.gz" {
		t.Fatal("got unexpected new filename: ", newFilename)
	}
	if _, err := os.Stat(filePath); err == nil || !os.IsNotExist(err) {
		t.Fatal("file still has old name or is inaccessible")
	}
	newPath := filepath.Join(tempDir, newFilename)
	if _, err := os.Stat(newPath); err != nil {
		t.Fatal("file not found at expected new path", newPath)
	}
	if doc.AnchoredPath() != newFilename {
		t.Fatal("document not renamed in library database")
	}
	if lib.CheckFilePath(newPath, false).Status() != Tracked {
		t.Fatal("status of new path not as expected")
	}
}

func TestBlockAddingSystemFiles(t *testing.T) {
	//GIVEN
	tempDir, lib := setupLibraryInTemp(t)
	doc, _ := lib.CreateDocument(42)
	//databasePath := filepath.Join(tempDir, "doccurator.db")
	locatorPath := filepath.Join(tempDir, "fake_dir", LocatorFileName)

	t.Run("AddLocatorFile", func(Test *testing.T) {
		//WHEN
		err := lib.SetDocumentPath(doc, locatorPath)
		//THEN
		if err == nil {
			Test.Fatal("adding locator not prevented")
		}
	})
}
