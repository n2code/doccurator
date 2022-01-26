package library

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/n2code/doccinator/internal/document"
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
	lib := MakeRuntimeLibrary()
	maxId := document.DocumentId(1<<64 - 1)
	regularId := document.DocumentId(1)
	minId := document.DocumentId(0)

	//WHEN
	regularDoc, err := lib.CreateDocument(regularId)
	//THEN
	if err != nil || regularDoc == (LibraryDocument{}) {
		t.Fatal("creation with regular ID failed")
	}

	//WHEN
	minDoc, err := lib.CreateDocument(minId)
	//THEN
	if err != nil || minDoc == (LibraryDocument{}) {
		t.Fatal("creation with min. ID failed")
	}

	//WHEN
	maxDoc, err := lib.CreateDocument(maxId)
	//THEN
	if err != nil || maxDoc == (LibraryDocument{}) {
		t.Fatal("creation with max. ID failed")
	}

	//WHEN
	docNone, err := lib.CreateDocument(regularId)
	//THEN
	if err == nil || docNone != (LibraryDocument{}) {
		t.Fatal("creation not rejected as expected")
	}
}

func setupLibraryInTemp(t *testing.T) (tempRootDir string, library LibraryApi) {
	tempRootDir = t.TempDir()
	library = MakeRuntimeLibrary()
	library.SetRoot(tempRootDir)
	return
}

func writeFile(path string, content string) {
	os.WriteFile(path, []byte(content), fs.ModePerm)
}

func TestDocumentUpdating(t *testing.T) {
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
}

func TestActiveVersusObsoleteOrchestration(t *testing.T) {
	//GIVEN
	tempRootDir, lib := setupLibraryInTemp(t)
	filePath := filepath.Join(tempRootDir, "file_for_lifecycle")
	writeFile(filePath, "content")

	t.Run("UnrecordedDocument", func(Test *testing.T) {
		//WHEN
		unrecordedDoc, exists := lib.GetActiveDocumentByPath(filePath)
		//THEN
		if unrecordedDoc != (LibraryDocument{}) || exists {
			Test.Fatal("unrecorded document not rejected")
		}
	})

	t.Run("DocumentOutsideLibrary", func(Test *testing.T) {
		//WHEN
		outsideDoc, exists := lib.GetActiveDocumentByPath(filepath.Join(tempRootDir, "../file_outside"))
		//THEN
		if outsideDoc != (LibraryDocument{}) || exists {
			t.Fatal("document outside library path not rejected")
		}
	})

	//GIVEN
	recordedDoc, _ := lib.CreateDocument(42)
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
		if existsAsActive || queriedObsoleteDocument != (LibraryDocument{}) {
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
		if existsAsActive || queriedObsoleteDocument != (LibraryDocument{}) {
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
		path              string
		contentOnRecord   string //if empty no record will be created
		contentIsObsolete bool   //if set obsolete document will refer to content on record + path
		fileContent       string //if empty file will not exist
		fileTimeOffset    int    //optional
		expected          PathStatus
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
						fmt.Sprintf(`%s%s (DB: %s, file: %s) -> expect %s`, file.path, obsoleteMarker, dbText, fileText, file.expected))
				}
				Test.Logf("Tested combination of %d path(s):\n   %s", len(files), strings.Join(fileRepresentations, "\n & "))
			}

			//GIVEN
			libRootDir, lib := setupLibraryInTemp(Test)
			fullFilePath := func(file f) string {
				return filepath.Join(libRootDir, filepath.FromSlash(file.path))
			}
			docs := make([]LibraryDocument, len(files), len(files))

			for i, subject := range files {
				if subject.contentOnRecord != "" {
					doc, err := lib.CreateDocument(document.DocumentId(i))
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
				checkResult := lib.CheckFilePath(fullFilePath(subject))
				//THEN
				if checkResult.status != subject.expected {
					Test.Errorf("expected status %s for %s but got %s", subject.expected, subject.path, checkResult.status)
				}
				if checkResult.err != nil && subject.expected != Error {
					Test.Errorf("got unexpected error for %s: %s", subject.path, checkResult.err)
				}
				if subject.expected == Error && checkResult.err == nil {
					Test.Errorf("did not get error for %s", subject.path)
				}
			}
		}
		t.Run(name, verification)
	}

	testStatusCombination("OneTracked",
		f{path: "A", contentOnRecord: "1", fileContent: "1", expected: Tracked})

	testStatusCombination("AllTracked",
		f{path: "A", contentOnRecord: "1", fileContent: "1", expected: Tracked},
		f{path: "B", contentOnRecord: "2", fileContent: "2", expected: Tracked},
		f{path: "C", contentOnRecord: "3", fileContent: "3", expected: Tracked})

	testStatusCombination("AllUntracked",
		f{path: "A", fileContent: "1", expected: Untracked},
		f{path: "B", fileContent: "2", expected: Untracked},
		f{path: "C", fileContent: "3", expected: Untracked})

	testStatusCombination("AllUntrackedWithClone",
		f{path: "A      ", fileContent: "1", expected: Untracked},
		f{path: "A_CLONE", fileContent: "1", expected: Untracked},
		f{path: "B      ", fileContent: "2", expected: Untracked})

	testStatusCombination("SomeModifiedAndUntrackedClone",
		f{path: "A", contentOnRecord: "_", fileContent: "1", expected: Modified},
		f{path: "B", contentOnRecord: "__", fileContent: "2", expected: Modified},
		f{path: "A_CLONE", contentOnRecord: noRecord, fileContent: "1", expected: Untracked})

	testStatusCombination("NonExistingFileInLibrary",
		f{path: "A", contentOnRecord: noRecord, fileContent: noFile, expected: Error})

	testStatusCombination("NonExistingFileOutsideOfLibrary",
		f{path: "../outside", contentOnRecord: noRecord, fileContent: noFile, expected: Error})

	testStatusCombination("DuplicateIsAnUntrackedClone",
		f{path: "A", contentOnRecord: "1", fileContent: "1", expected: Tracked},
		f{path: "A_CLONE", contentOnRecord: noRecord, fileContent: "1", expected: Duplicate})

	testStatusCombination("MixDuplicateAndInaccessible",
		f{path: "A", contentOnRecord: "1", fileContent: "1", expected: Tracked},
		f{path: "X", contentOnRecord: noRecord, fileContent: noFile, expected: Error},
		f{path: "A_CLONE", contentOnRecord: noRecord, fileContent: "1", expected: Duplicate})

	testStatusCombination("MixTouchedAndDuplicate",
		f{path: "A", contentOnRecord: "1", fileContent: "1", fileTimeOffset: 42, expected: Touched},
		f{path: "A_CLONE", contentOnRecord: noRecord, fileContent: "1", expected: Duplicate})

	testStatusCombination("DuplicateOfModifiedIsUntracked", //because if content is changed the saviour of the old version shall be preserved
		f{path: "A", contentOnRecord: "1", fileContent: "1+", fileTimeOffset: 42, expected: Modified},
		f{path: "B", contentOnRecord: noRecord, fileContent: "1", expected: Untracked})

	testStatusCombination("Missing",
		f{path: "A", contentOnRecord: "1", fileContent: noFile, expected: Missing})

	testStatusCombination("DuplicateOfMissingIsMoved", //because if original is absent it can be interpreted as a move
		f{path: "OLD", contentOnRecord: "1", fileContent: noFile, expected: Missing},
		f{path: "NEW", contentOnRecord: noRecord, fileContent: "1", expected: Moved})

	testStatusCombination("TrackedCloneBesidesMoved",
		f{path: "OLD", contentOnRecord: "1", fileContent: noFile, expected: Missing},
		f{path: "NEW", contentOnRecord: noRecord, fileContent: "1", expected: Moved},
		f{path: "COPY", contentOnRecord: "1", fileContent: "1", expected: Tracked})

	testStatusCombination("MixModifiedAndDuplicate",
		f{path: "A", contentOnRecord: "1", fileContent: "1+", expected: Modified},
		f{path: "B", contentOnRecord: "1", fileContent: "1", expected: Tracked},
		f{path: "C", contentOnRecord: noRecord, fileContent: "1", expected: Duplicate}) //duplicate with respect to B

	testStatusCombination("ObsoleteLeftover",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: "1", expected: Obsolete})

	testStatusCombination("ObsoleteLeftoverNextToTrackedClone",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: "1", expected: Obsolete},
		f{path: "A_CLONE", contentOnRecord: "1", fileContent: "1", expected: Tracked})

	testStatusCombination("MixObsoleteAndTrackedCloneAndUntracked",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: "1", expected: Obsolete}, //supposed to be deleted and has not changed
		f{path: "A_CLONE", contentOnRecord: "1", fileContent: "1", expected: Tracked},
		f{path: "OTHER", contentOnRecord: noRecord, fileContent: "3", expected: Untracked}) //because it neither matches any record nor any path

	testStatusCombination("NewContentAtObsoletedPath",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: "2", expected: Untracked}, //different content at obsoleted path
		f{path: "OTHER", contentOnRecord: noRecord, fileContent: "3", expected: Untracked})                 //because it neither matches any record nor any path

	testStatusCombination("Removed",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expected: Removed})

	testStatusCombination("MixRemovedAndUntracked",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expected: Removed},
		f{path: "OTHER", contentOnRecord: noRecord, fileContent: "3", expected: Untracked}) //because it neither matches any record nor any path

	testStatusCombination("ObsoleteMatchesPastRecord",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expected: Removed},
		f{path: "UNWANTED", contentOnRecord: noRecord, fileContent: "1", expected: Obsolete}) //path is new but content is obsolete

	testStatusCombination("ObsoleteContentRecreatedNextToActiveModifiedWithObsoleteContentOnRecord",
		f{path: "A", contentOnRecord: noRecord, fileContent: "1", expected: Obsolete},
		f{path: "A_PAST", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expected: Removed},
		f{path: "B", contentOnRecord: "1", fileContent: "2", expected: Modified})

	testStatusCombination("ObsoleteHasPriorityOverDuplicate",
		f{path: "A", contentOnRecord: noRecord, fileContent: "1", expected: Obsolete}, //because the content is present and matching
		f{path: "B", contentOnRecord: "1", fileContent: "1", expected: Tracked},
		f{path: "C", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expected: Removed})
}

func TestVisitRecordsAndPrint(t *testing.T) {
	//GIVEN
	lib := MakeRuntimeLibrary()
	imaginaryRoot := "/imaginary"
	lib.SetRoot(imaginaryRoot)
	id22222X, relativePathA := document.DocumentId(0), "A.22222X.ndoc"
	id97322X, relativePathB := document.DocumentId(13), "B.97322X.ndoc"
	id94722N, relativePathC := document.DocumentId(42), "C.94722N.ndoc"

	//WHEN
	docA, _ := lib.CreateDocument(id22222X)
	docB, _ := lib.CreateDocument(id97322X)
	docC, _ := lib.CreateDocument(id94722N)
	lib.SetDocumentPath(docA, filepath.Join(imaginaryRoot, relativePathA))
	lib.SetDocumentPath(docB, filepath.Join(imaginaryRoot, relativePathB))
	lib.SetDocumentPath(docC, filepath.Join(imaginaryRoot, relativePathC))
	lib.MarkDocumentAsObsolete(docB)
	lib.ForgetDocument(docC)

	//THEN
	var recordPrintout strings.Builder
	lib.VisitAllRecords(func(doc LibraryDocument) {
		recordPrintout.WriteString(doc.String())
		recordPrintout.WriteRune('\n')
	})
	if !strings.Contains(recordPrintout.String(), relativePathA) ||
		!strings.Contains(recordPrintout.String(), relativePathB) ||
		strings.Contains(recordPrintout.String(), relativePathC) { //C must not be contained because it should have been forgotten
		t.Fatal("record printout unexpected:\n" + recordPrintout.String())
	}
}
