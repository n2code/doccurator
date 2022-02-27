package library

import (
	"fmt"
	"github.com/n2code/ndocid"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/n2code/doccurator/internal/document"
)

func TestGetPathRelativeToLibraryRoot(t *testing.T) {
	lib := library{rootPath: "/dummy/root"}

	assertRelPath := func(full string, expRel string) {
		actRel, inLibrary := lib.getPathRelativeToLibraryRoot(full)
		if inLibrary != true {
			t.Error("expected detection to determine that path", full, "is inside library")
		}
		if actRel != expRel {
			t.Error("expected", expRel, "but got", actRel)
		}
	}

	assertNotInLib := func(full string) {
		actRel, inLibrary := lib.getPathRelativeToLibraryRoot(full)
		if inLibrary != false {
			t.Error("expected detection to determine that path", full, "is outside library")
		}
		if actRel != "" {
			t.Error("expected path", full, "to be empty on error but got", actRel)
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
		obsoleteExists := lib.ObsoleteDocumentExistsForPath(filePath)
		//THEN
		if obsoleteExists {
			Test.Fatal("obsolete version reported to exist already")
		}
	})

	//GIVEN
	lib.MarkDocumentAsObsolete(recordedDoc)

	t.Run("DeclaredDocumentObsolete", func(Test *testing.T) {
		//WHEN
		isObsolete := recordedDoc.IsObsolete()
		queriedObsoleteDocument, existsAsActive := lib.GetActiveDocumentByPath(filePath)
		obsoleteExists := lib.ObsoleteDocumentExistsForPath(filePath)
		//THEN
		if !isObsolete {
			Test.Fatal("not obsolete")
		}
		if existsAsActive || queriedObsoleteDocument != (Document{}) {
			Test.Fatal("still reported as existing / found although obsoleted")
		}
		if !obsoleteExists {
			Test.Fatal("obsolete version not known to exist for path")
		}
	})

	//GIVEN
	os.Remove(filePath)

	t.Run("VerifyRemovedDocumentStillObsolete", func(Test *testing.T) {
		//WHEN
		isObsolete := recordedDoc.IsObsolete()
		queriedObsoleteDocument, existsAsActive := lib.GetActiveDocumentByPath(filePath)
		obsoleteExists := lib.ObsoleteDocumentExistsForPath(filePath)
		//THEN
		if !isObsolete {
			Test.Fatal("removed document is not obsolete")
		}
		if existsAsActive || queriedObsoleteDocument != (Document{}) {
			Test.Fatal("still reported as existing / found although obsoleted and removed")
		}
		if !obsoleteExists {
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
		obsoleteExists := lib.ObsoleteDocumentExistsForPath(filePath)
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
		if !obsoleteExists {
			Test.Fatal("obsolete version not known to exist for path which also has active")
		}
	})

	//GIVEN
	lib.ForgetDocument(recordedDoc)

	t.Run("VerifyPurgeOfObsoleteVersion", func(Test *testing.T) {
		//WHEN
		queriedDocument, activeOneStillExists := lib.GetActiveDocumentByPath(filePath)
		obsoleteExists := lib.ObsoleteDocumentExistsForPath(filePath)
		//THEN
		if !activeOneStillExists || queriedDocument != differentDoc {
			Test.Fatal("new document not found")
		}
		if obsoleteExists {
			Test.Fatal("obsolete version not forgotten")
		}
	})
}

func TestPathChecking(t *testing.T) {
	type f struct {
		path               string
		contentOnRecord    string //if empty no record will be created
		contentIsObsolete  bool   //if set obsolete document will refer to content on record + path
		fileContent        string //if empty file will not exist
		fileTimeOffset     int    //optional
		expectedStatus     PathStatus
		expectedRefToDocOf uint //1-based index of test configuration
	}

	noFile := ""
	noRecord := ""
	testStatusCombination := func(name string, files ...f) {
		verification := func(Test *testing.T) {
			{
				fileRepresentations := make([]string, 0, len(files))
				for _, file := range files {
					obsoleteMarker := map[bool]string{true: "[obsolete]"}[file.contentIsObsolete]
					dbText := map[bool]string{true: fmt.Sprintf(`"%s"`, file.contentOnRecord), false: "<no rec>"}[file.contentOnRecord != ""]
					fileText := map[bool]string{true: fmt.Sprintf(`"%s"`, file.fileContent), false: "<none>"}[file.fileContent != ""]
					fileRepresentations = append(fileRepresentations,
						fmt.Sprintf(`%s%s (DB: %s, file: %s) -> expect %s`, file.path, obsoleteMarker, dbText, fileText, file.expectedStatus))
				}
				Test.Logf("Tested combination of %d path(s):\n   %s", len(files), strings.Join(fileRepresentations, "\n & "))
			}

			//GIVEN
			libRootDir, lib := setupLibraryInTemp(Test)
			fullFilePath := func(file f) string {
				return filepath.Join(libRootDir, filepath.FromSlash(file.path))
			}
			docs := make([]Document, len(files), len(files))

			for i, subject := range files {
				if subject.contentOnRecord != "" {
					doc, err := lib.CreateDocument(document.Id(i + 1)) //first test config entry has ID (1), second has (2), etc.
					if err != nil {
						Test.Fatalf("creation of document for %s failed", subject.path)
					}
					docs[i] = doc
				}
			}
			baseTime := time.Now().Local()
			for i, subject := range files {
				if subject.path == "" || (subject.contentIsObsolete && subject.contentOnRecord == "") {
					Test.Fatal("test data error")
				}
				if subject.contentOnRecord != "" {
					doc := docs[i]
					err := lib.SetDocumentPath(doc, fullFilePath(subject))
					if err != nil {
						Test.Fatalf("setting path %s for document %s failed", subject.path, doc.id)
					}
					writeFile(fullFilePath(subject), subject.contentOnRecord)
					os.Chtimes(fullFilePath(subject), baseTime, baseTime)
					lib.UpdateDocumentFromFile(doc)
					if subject.contentIsObsolete {
						lib.MarkDocumentAsObsolete(doc)
					}
					if subject.fileContent == "" {
						os.Remove(fullFilePath(subject))
					}
				}
				if subject.fileContent != "" {
					writeFile(fullFilePath(subject), subject.fileContent)
					offsetTime := baseTime.Add(time.Second * time.Duration(subject.fileTimeOffset))
					os.Chtimes(fullFilePath(subject), offsetTime, offsetTime)
				}
			}

			//WHEN
			for _, subject := range files {
				checkResult := lib.CheckFilePath(fullFilePath(subject), false)
				//THEN
				if checkResult.status != subject.expectedStatus {
					Test.Errorf("expected status %s for %s but got %s", subject.expectedStatus, subject.path, checkResult.status)
				}
				if checkResult.err != nil && subject.expectedStatus != Error {
					Test.Errorf("got unexpected error for %s: %s", subject.path, checkResult.err)
				}
				if subject.expectedStatus == Error && checkResult.err == nil {
					Test.Errorf("did not get error for %s", subject.path)
				}
				if subject.expectedRefToDocOf != 0 {
					if checkResult.referencing.id == document.MissingId || checkResult.referencing.library == nil {
						Test.Errorf("did not get proper reference")
					}
					if document.Id(subject.expectedRefToDocOf) != checkResult.referencing.id {
						Test.Errorf("expected reference to ID %s not received, got %s", document.Id(subject.expectedRefToDocOf), checkResult.referencing.id)
					}
				} else {
					if checkResult.referencing.id != document.MissingId {
						Test.Errorf("got reference to ID where none was expected")
					}
					if checkResult.referencing.library != nil {
						Test.Errorf("expected reference to be empty but got library API handle")
					}
				}
			}
		}
		t.Run(name, verification)
	}

	testStatusCombination("OneTracked",
		f{path: "A", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked})

	testStatusCombination("AllTracked",
		f{path: "A", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked},
		f{path: "B", contentOnRecord: "2", fileContent: "2", expectedStatus: Tracked},
		f{path: "C", contentOnRecord: "3", fileContent: "3", expectedStatus: Tracked})

	testStatusCombination("AllUntracked",
		f{path: "A", fileContent: "1", expectedStatus: Untracked},
		f{path: "B", fileContent: "2", expectedStatus: Untracked},
		f{path: "C", fileContent: "3", expectedStatus: Untracked})

	testStatusCombination("AllUntrackedWithClone",
		f{path: "A      ", fileContent: "1", expectedStatus: Untracked},
		f{path: "A_CLONE", fileContent: "1", expectedStatus: Untracked},
		f{path: "B      ", fileContent: "2", expectedStatus: Untracked})

	testStatusCombination("SomeModifiedAndUntrackedClone",
		f{path: "A", contentOnRecord: "_", fileContent: "1", expectedStatus: Modified, expectedRefToDocOf: 1},
		f{path: "B", contentOnRecord: "__", fileContent: "2", expectedStatus: Modified, expectedRefToDocOf: 2},
		f{path: "A_CLONE", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Untracked})

	testStatusCombination("NonExistingFileInLibrary",
		f{path: "A", contentOnRecord: noRecord, fileContent: noFile, expectedStatus: Error})

	testStatusCombination("NonExistingFileOutsideOfLibrary",
		f{path: "../outside", contentOnRecord: noRecord, fileContent: noFile, expectedStatus: Error})

	testStatusCombination("DuplicateIsAnUntrackedClone",
		f{path: "A", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked},
		f{path: "A_CLONE", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Duplicate})

	testStatusCombination("MixDuplicateAndInaccessible",
		f{path: "A", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked},
		f{path: "X", contentOnRecord: noRecord, fileContent: noFile, expectedStatus: Error},
		f{path: "A_CLONE", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Duplicate})

	testStatusCombination("MixTouchedAndDuplicate",
		f{path: "A", contentOnRecord: "1", fileContent: "1", fileTimeOffset: 42, expectedStatus: Touched, expectedRefToDocOf: 1},
		f{path: "A_CLONE", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Duplicate})

	testStatusCombination("DuplicateOfModifiedIsUntracked", //because if content is changed the saviour of the old version shall be preserved
		f{path: "A", contentOnRecord: "1", fileContent: "1+", fileTimeOffset: 42, expectedStatus: Modified, expectedRefToDocOf: 1},
		f{path: "B", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Untracked})

	testStatusCombination("Missing",
		f{path: "A", contentOnRecord: "1", fileContent: noFile, expectedStatus: Missing})

	testStatusCombination("DuplicateOfMissingIsMoved", //because if original is absent it can be interpreted as a move
		f{path: "OLD", contentOnRecord: "1", fileContent: noFile, expectedStatus: Missing},
		f{path: "NEW", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Moved, expectedRefToDocOf: 1})

	testStatusCombination("TrackedCloneBesidesMoved",
		f{path: "COPY", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked},
		f{path: "NEW", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Moved, expectedRefToDocOf: 3},
		f{path: "OLD", contentOnRecord: "1", fileContent: noFile, expectedStatus: Missing})

	testStatusCombination("MixModifiedAndDuplicate",
		f{path: "A", contentOnRecord: "1", fileContent: "1+", expectedStatus: Modified, expectedRefToDocOf: 1},
		f{path: "B", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked},
		f{path: "C", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Duplicate}) //duplicate with respect to B

	testStatusCombination("ObsoleteLeftover",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: "1", expectedStatus: Obsolete})

	testStatusCombination("ObsoleteLeftoverNextToTrackedClone",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: "1", expectedStatus: Obsolete},
		f{path: "A_CLONE", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked})

	testStatusCombination("MixObsoleteAndTrackedCloneAndUntracked",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: "1", expectedStatus: Obsolete}, //supposed to be deleted and has not changed
		f{path: "A_CLONE", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked},
		f{path: "OTHER", contentOnRecord: noRecord, fileContent: "3", expectedStatus: Untracked}) //because it neither matches any record nor any path

	testStatusCombination("NewContentAtObsoletedPath",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: "2", expectedStatus: Untracked}, //different content at obsoleted path
		f{path: "OTHER", contentOnRecord: noRecord, fileContent: "3", expectedStatus: Untracked})                 //because it neither matches any record nor any path

	testStatusCombination("Removed",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expectedStatus: Removed})

	testStatusCombination("MixRemovedAndUntracked",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expectedStatus: Removed},
		f{path: "OTHER", contentOnRecord: noRecord, fileContent: "3", expectedStatus: Untracked}) //because it neither matches any record nor any path

	testStatusCombination("ObsoleteMatchesPastRecord",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expectedStatus: Removed},
		f{path: "UNWANTED", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Obsolete}) //path is new but content is obsolete

	testStatusCombination("ObsoleteContentRecreatedNextToActiveModifiedWithObsoleteContentOnRecord",
		f{path: "A", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Obsolete},
		f{path: "A_PAST", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expectedStatus: Removed},
		f{path: "B", contentOnRecord: "1", fileContent: "2", expectedStatus: Modified, expectedRefToDocOf: 3})

	testStatusCombination("ObsoleteHasPriorityOverDuplicate",
		f{path: "A", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Obsolete}, //because the content is present and matching
		f{path: "B", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked},
		f{path: "C", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expectedStatus: Removed})
}

func TestVisitRecordsAndPrint(t *testing.T) {
	//GIVEN
	lib := NewLibrary()
	imaginaryRoot := "/imaginary"
	lib.SetRoot(imaginaryRoot)
	id932229, relativePathA := document.Id(1), "A.932229.ndoc"
	id97322X, relativePathB := document.Id(13), "B.97322X.ndoc"
	id94722N, relativePathC := document.Id(42), "C.94722N.ndoc"

	//WHEN
	docA, _ := lib.CreateDocument(id932229)
	docB, _ := lib.CreateDocument(id97322X)
	docC, _ := lib.CreateDocument(id94722N)
	lib.SetDocumentPath(docA, filepath.Join(imaginaryRoot, relativePathA))
	lib.SetDocumentPath(docB, filepath.Join(imaginaryRoot, relativePathB))
	lib.SetDocumentPath(docC, filepath.Join(imaginaryRoot, relativePathC))
	lib.MarkDocumentAsObsolete(docB)
	lib.ForgetDocument(docC)

	//THEN
	var recordPrintout strings.Builder
	lib.VisitAllRecords(func(doc Document) {
		recordPrintout.WriteString(doc.String())
		recordPrintout.WriteRune('\n')
	})
	if !strings.Contains(recordPrintout.String(), relativePathA) ||
		!strings.Contains(recordPrintout.String(), relativePathB) ||
		strings.Contains(recordPrintout.String(), relativePathC) { //C must not be contained because it should have been forgotten
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
	newFilename, err, _ := doc.RenameToStandardNameFormat()

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
	if doc.PathRelativeToLibraryRoot() != newFilename {
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
