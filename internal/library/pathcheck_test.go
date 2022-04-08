package library

import (
	"fmt"
	"github.com/n2code/doccurator/internal/document"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPathChecking(t *testing.T) {
	type f struct {
		path               string
		contentOnRecord    string //if empty no record will be created, if == "<EMPTY>" empty file is recorded
		contentIsObsolete  bool   //if set obsolete document will refer to content on record + path
		fileContent        string //if empty file will not exist, if == "<EMPTY>" file will be empty
		fileTimeOffset     int    //optional
		expectedStatus     PathStatus
		expectedRefToDocOf uint //1-based index of test configuration
	}

	noFile := ""
	emptyFile := "<EMPTY>"
	noRecord := ""
	emptyRecord := "<EMPTY>"
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
						Test.Errorf("did not get proper reference for %s", subject.path)
					}
					if document.Id(subject.expectedRefToDocOf) != checkResult.referencing.id {
						Test.Errorf("expected reference to ID %s not received, got %s", document.Id(subject.expectedRefToDocOf), checkResult.referencing.id)
					}
				} else {
					if checkResult.referencing.id != document.MissingId {
						Test.Errorf("got reference to ID where none was expected for %s", subject.path)
					}
					if checkResult.referencing.library != nil {
						Test.Errorf("expected library API handle to be empty but got one for %s", subject.path)
					}
				}
			}
		}
		t.Run(name, verification)
	}

	testStatusCombination("OneTracked",
		f{path: "A", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked, expectedRefToDocOf: 1})

	testStatusCombination("AllTracked",
		f{path: "A", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked, expectedRefToDocOf: 1},
		f{path: "B", contentOnRecord: "2", fileContent: "2", expectedStatus: Tracked, expectedRefToDocOf: 2},
		f{path: "C", contentOnRecord: "3", fileContent: "3", expectedStatus: Tracked, expectedRefToDocOf: 3})

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
		f{path: "A", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked, expectedRefToDocOf: 1},
		f{path: "A_CLONE", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Duplicate, expectedRefToDocOf: 1})

	testStatusCombination("MixDuplicateAndInaccessible",
		f{path: "A", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked, expectedRefToDocOf: 1},
		f{path: "X", contentOnRecord: noRecord, fileContent: noFile, expectedStatus: Error},
		f{path: "A_CLONE", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Duplicate, expectedRefToDocOf: 1})

	testStatusCombination("MixTouchedAndDuplicate",
		f{path: "A", contentOnRecord: "1", fileContent: "1", fileTimeOffset: 42, expectedStatus: Touched, expectedRefToDocOf: 1},
		f{path: "A_CLONE", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Duplicate, expectedRefToDocOf: 1})

	testStatusCombination("DuplicateOfModifiedIsUntracked", //because if content is changed the saviour of the old version shall be preserved
		f{path: "A", contentOnRecord: "1", fileContent: "1+", fileTimeOffset: 42, expectedStatus: Modified, expectedRefToDocOf: 1},
		f{path: "B", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Untracked})

	testStatusCombination("Missing",
		f{path: "A", contentOnRecord: "1", fileContent: noFile, expectedStatus: Missing, expectedRefToDocOf: 1})

	testStatusCombination("DuplicateOfMissingIsMoved", //because if original is absent it can be interpreted as a move
		f{path: "OLD", contentOnRecord: "1", fileContent: noFile, expectedStatus: Missing, expectedRefToDocOf: 1},
		f{path: "NEW", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Moved, expectedRefToDocOf: 1})

	testStatusCombination("TrackedCloneBesidesMoved",
		f{path: "COPY", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked, expectedRefToDocOf: 1},
		f{path: "NEW", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Moved, expectedRefToDocOf: 3},
		f{path: "OLD", contentOnRecord: "1", fileContent: noFile, expectedStatus: Missing, expectedRefToDocOf: 3})

	testStatusCombination("MixModifiedAndDuplicate",
		f{path: "A", contentOnRecord: "1", fileContent: "1+", expectedStatus: Modified, expectedRefToDocOf: 1},
		f{path: "B", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked, expectedRefToDocOf: 2},
		f{path: "C", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Duplicate, expectedRefToDocOf: 2}) //duplicate with respect to B

	testStatusCombination("ObsoleteLeftover",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: "1", expectedStatus: Obsolete, expectedRefToDocOf: 1})

	testStatusCombination("ObsoleteLeftoverNextToTrackedClone",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: "1", expectedStatus: Obsolete, expectedRefToDocOf: 1},
		f{path: "A_CLONE", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked, expectedRefToDocOf: 2})

	testStatusCombination("MixObsoleteAndTrackedCloneAndUntracked",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: "1", expectedStatus: Obsolete, expectedRefToDocOf: 1}, //supposed to be deleted and has not changed
		f{path: "A_CLONE", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked, expectedRefToDocOf: 2},
		f{path: "OTHER", contentOnRecord: noRecord, fileContent: "3", expectedStatus: Untracked}) //because it neither matches any record nor any path

	testStatusCombination("NewContentAtObsoletedPath",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: "2", expectedStatus: Untracked}, //different content at obsoleted path
		f{path: "OTHER", contentOnRecord: noRecord, fileContent: "3", expectedStatus: Untracked})                 //because it neither matches any record nor any path

	testStatusCombination("Removed",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expectedStatus: Removed, expectedRefToDocOf: 1})

	testStatusCombination("MixRemovedAndUntracked",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expectedStatus: Removed, expectedRefToDocOf: 1},
		f{path: "OTHER", contentOnRecord: noRecord, fileContent: "3", expectedStatus: Untracked}) //because it neither matches any record nor any path

	testStatusCombination("ObsoleteMatchesPastRecord",
		f{path: "A", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expectedStatus: Removed, expectedRefToDocOf: 1},
		f{path: "UNWANTED", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Obsolete, expectedRefToDocOf: 1}) //path is new but content is obsolete

	testStatusCombination("ObsoleteContentRecreatedNextToActiveModifiedWithObsoleteContentOnRecord",
		f{path: "A", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Obsolete, expectedRefToDocOf: 2},
		f{path: "A_PAST", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expectedStatus: Removed, expectedRefToDocOf: 2},
		f{path: "B", contentOnRecord: "1", fileContent: "2", expectedStatus: Modified, expectedRefToDocOf: 3})

	testStatusCombination("ObsoleteHasPriorityOverDuplicate",
		f{path: "A", contentOnRecord: noRecord, fileContent: "1", expectedStatus: Obsolete, expectedRefToDocOf: 3}, //because the content is present and matching
		f{path: "B", contentOnRecord: "1", fileContent: "1", expectedStatus: Tracked, expectedRefToDocOf: 2},
		f{path: "C", contentOnRecord: "1", contentIsObsolete: true, fileContent: noFile, expectedStatus: Removed, expectedRefToDocOf: 3})

	testStatusCombination("UntrackedEmptyFile",
		f{path: "A", contentOnRecord: noRecord, fileContent: emptyFile, expectedStatus: Untracked})

	testStatusCombination("TrackedEmptyFile",
		f{path: "A", contentOnRecord: emptyRecord, fileContent: emptyFile, expectedStatus: Tracked, expectedRefToDocOf: 1})

	testStatusCombination("TouchedEmptyFile",
		f{path: "A", contentOnRecord: emptyRecord, fileContent: emptyFile, fileTimeOffset: 42, expectedStatus: Touched, expectedRefToDocOf: 1})

	testStatusCombination("ObsoleteEmptyFile",
		f{path: "A", contentOnRecord: emptyRecord, contentIsObsolete: true, fileContent: emptyFile, expectedStatus: Obsolete, expectedRefToDocOf: 1})

	testStatusCombination("MissingEmptyFile",
		f{path: "A", contentOnRecord: emptyRecord, fileContent: noFile, expectedStatus: Missing, expectedRefToDocOf: 1})

	testStatusCombination("MovedEmptyFile",
		f{path: "A_OLD", contentOnRecord: emptyRecord, fileContent: noFile, expectedStatus: Missing, expectedRefToDocOf: 1},
		f{path: "A_NEW", contentOnRecord: noRecord, fileContent: emptyFile, expectedStatus: Moved, expectedRefToDocOf: 1})

	testStatusCombination("RemovedEmptyFile",
		f{path: "A", contentOnRecord: emptyRecord, contentIsObsolete: true, fileContent: noFile, expectedStatus: Removed, expectedRefToDocOf: 1})

	testStatusCombination("TruncatedFile",
		f{path: "A", contentOnRecord: "42", fileContent: emptyFile, expectedStatus: Modified, expectedRefToDocOf: 1})

	testStatusCombination("EmptyFilesAreNoDuplicates",
		f{path: "A", contentOnRecord: emptyRecord, fileContent: emptyFile, expectedStatus: Tracked, expectedRefToDocOf: 1},
		f{path: "A_CLONE1", contentOnRecord: noRecord, fileContent: emptyFile, expectedStatus: Untracked},
		f{path: "A_CLONE2", contentOnRecord: noRecord, fileContent: emptyFile, expectedStatus: Untracked})

	testStatusCombination("EmptyFilesCantBeConsideredObsolete1",
		f{path: "A", contentOnRecord: emptyRecord, contentIsObsolete: true, fileContent: noFile, expectedStatus: Removed, expectedRefToDocOf: 1},
		f{path: "EMPTY", contentOnRecord: noRecord, fileContent: emptyFile, expectedStatus: Untracked})

	testStatusCombination("EmptyFilesCantBeConsideredObsolete2",
		f{path: "A", contentOnRecord: emptyRecord, contentIsObsolete: true, fileContent: emptyFile, expectedStatus: Obsolete, expectedRefToDocOf: 1},
		f{path: "EMPTY", contentOnRecord: noRecord, fileContent: emptyFile, expectedStatus: Untracked})

	testStatusCombination("DetectMovedEmptyFiles",
		f{path: "A", contentOnRecord: emptyRecord, fileContent: emptyFile, expectedStatus: Tracked, expectedRefToDocOf: 1},
		f{path: "B", contentOnRecord: emptyRecord, fileContent: noFile, expectedStatus: Missing, expectedRefToDocOf: 2},
		f{path: "B_ELSEWHERE", contentOnRecord: noRecord, fileContent: emptyFile, expectedStatus: Moved, expectedRefToDocOf: 2})

	testStatusCombination("TruncatedFileAmongEmpties",
		f{path: "A", contentOnRecord: emptyRecord, fileContent: emptyFile, expectedStatus: Tracked, expectedRefToDocOf: 1},
		f{path: "B", contentOnRecord: emptyRecord, contentIsObsolete: true, fileContent: noFile, expectedStatus: Removed, expectedRefToDocOf: 2},
		f{path: "C", contentOnRecord: emptyRecord, contentIsObsolete: true, fileContent: emptyFile, expectedStatus: Obsolete, expectedRefToDocOf: 3},
		f{path: "X", contentOnRecord: "42", fileContent: emptyFile, expectedStatus: Modified, expectedRefToDocOf: 4})

}
