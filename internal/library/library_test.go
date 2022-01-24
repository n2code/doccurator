package library

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
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

func TestLibraryApi(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "doccinator-test-*")
	if err != nil {
		t.Fatal(err)
	}
	libRootDir, err := filepath.Abs(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.RemoveAll(libRootDir)
	}()
	fileNameSubject := "file_a1"
	fileNameAlternative := "file_b"
	fileNameCloneSubject := "file_a2" //copy of A
	filePathOfSubject := filepath.Join(libRootDir, fileNameSubject)
	filePathOfUntrackedOther := filepath.Join(libRootDir, fileNameAlternative)
	filePathOfCloneSubject := filepath.Join(libRootDir, fileNameCloneSubject)

	Lib := MakeRuntimeLibrary()
	Lib.SetRoot(libRootDir)

	assertPathCheck := func(actPath string, expPathStatus PathStatus) {
		checkResult := Lib.CheckFilePath(actPath)
		if checkResult.err != nil && expPathStatus != Error {
			t.Log(string(debug.Stack()))
			t.Errorf("got error for path check of %s instead of expected %s: %s", actPath, string(expPathStatus), err)
			return
		}
		if expPathStatus == Error && checkResult.err == nil {
			t.Log(string(debug.Stack()))
			t.Errorf("did not get error for path check of %s", actPath)
			return
		}
		if checkResult.status != expPathStatus {
			t.Log(string(debug.Stack()))
			t.Errorf("status of %s is not %s as expected, got %s", actPath, string(expPathStatus), string(checkResult.status))
		}
	}

	docSecondary, err := Lib.CreateDocument(2)
	if err != nil || docSecondary == (LibraryDocument{}) {
		t.Fatal("creation of secondary failed")
	}
	docPrimary, err := Lib.CreateDocument(1)
	if err != nil || docPrimary == (LibraryDocument{}) {
		t.Fatal("creation of primary failed")
	}

	docNone, err := Lib.CreateDocument(2)
	if err == nil || docNone != (LibraryDocument{}) {
		t.Fatal("creation not rejected as expected")
	}

	assertPathCheck(filePathOfSubject, Error) //real file does not exist yet
	assertPathCheck("/tmp/file_outside_library", Error)

	os.WriteFile(filePathOfSubject, []byte("AAA"), fs.ModePerm)
	os.WriteFile(filePathOfCloneSubject, []byte("AAA"), fs.ModePerm) //same content as A
	os.WriteFile(filePathOfUntrackedOther, []byte("BB"), fs.ModePerm)

	assertPathCheck(filePathOfSubject, Untracked)
	assertPathCheck(filePathOfUntrackedOther, Untracked)
	assertPathCheck(filePathOfCloneSubject, Untracked)

	Lib.SetDocumentPath(docPrimary, filePathOfSubject)
	Lib.SetDocumentPath(docSecondary, filePathOfUntrackedOther)

	assertPathCheck(filePathOfSubject, Modified)
	assertPathCheck(filePathOfUntrackedOther, Modified)
	assertPathCheck(filePathOfCloneSubject, Untracked)

	changed, err := Lib.UpdateDocumentFromFile(docPrimary)
	if err != nil {
		t.Fatal("update reported error")
	}
	if changed == false {
		t.Fatal("update did not report change")
	}

	changed, err = Lib.UpdateDocumentFromFile(docPrimary)
	if err != nil {
		t.Fatal("repeated update reported error")
	}
	if changed == true {
		t.Fatal("repeated update reported change")
	}

	os.Chmod(filePathOfSubject, 0o333)

	assertPathCheck(filePathOfSubject, Error) //read forbidden

	changed, err = Lib.UpdateDocumentFromFile(docPrimary)
	if err == nil {
		t.Fatal("update did not report error")
	}
	if changed == true {
		t.Fatal("update reported change although error occurred")
	}

	os.Chmod(filePathOfSubject, 0o777)

	assertPathCheck(filePathOfSubject, Tracked)
	assertPathCheck(filepath.Join(libRootDir, "file_which_should_not_exist"), Error)
	assertPathCheck(filePathOfCloneSubject, Duplicate)

	inTheFuture := time.Now().Add(time.Second)
	os.Chtimes(filePathOfSubject, inTheFuture, inTheFuture)

	assertPathCheck(filePathOfSubject, Touched)
	assertPathCheck(filePathOfCloneSubject, Duplicate)

	Lib.UpdateDocumentFromFile(docPrimary)

	assertPathCheck(filePathOfSubject, Tracked)

	os.Rename(filePathOfSubject, filePathOfSubject+".hidden")

	assertPathCheck(filePathOfSubject, Missing)

	os.Rename(filePathOfSubject+".hidden", filePathOfSubject)

	assertPathCheck(filePathOfSubject, Tracked)

	unrecordedDoc, exists := Lib.GetActiveDocumentByPath(filepath.Join(libRootDir, "file_not_on_record"))
	if unrecordedDoc != (LibraryDocument{}) || exists {
		t.Fatal("unrecorded document not rejected")
	}
	outsideDoc, exists := Lib.GetActiveDocumentByPath(filepath.Join(os.TempDir(), "doccinator-test-dummy"))
	if outsideDoc != (LibraryDocument{}) || exists {
		t.Fatal("document outside library path not rejected")
	}
	copyOfDocPrimary, exists := Lib.GetActiveDocumentByPath(filePathOfSubject)
	if copyOfDocPrimary != docPrimary || !exists {
		t.Fatal("retrieval of A failed")
	}

	os.Rename(filePathOfSubject, filePathOfSubject+".renamed")

	assertPathCheck(filePathOfSubject, Missing)
	assertPathCheck(filePathOfSubject+".renamed", Moved)

	docPrimaryClone, _ := Lib.CreateDocument(3)
	Lib.SetDocumentPath(docPrimaryClone, filePathOfCloneSubject)
	Lib.UpdateDocumentFromFile(docPrimaryClone)

	assertPathCheck(filePathOfCloneSubject, Tracked)
	assertPathCheck(filePathOfSubject, Missing)
	assertPathCheck(filePathOfSubject+".renamed", Moved)

	os.WriteFile(filePathOfSubject, []byte("A+"), fs.ModePerm)

	assertPathCheck(filePathOfSubject, Modified)
	assertPathCheck(filePathOfSubject+".renamed", Duplicate) //duplicate with respect to clone!
	assertPathCheck(filePathOfCloneSubject, Tracked)

	os.Rename(filePathOfSubject+".renamed", filePathOfSubject)

	assertPathCheck(filePathOfSubject, Tracked)
	assertPathCheck(filePathOfCloneSubject, Tracked)

	if docPrimary.IsObsolete() {
		t.Fatal("primary subject is already obsolete")
	}

	Lib.MarkDocumentAsObsolete(docPrimary)
	os.WriteFile(filePathOfSubject+".backup", []byte("A+"), fs.ModePerm)

	assertPathCheck(filePathOfCloneSubject, Tracked)        //still alive
	assertPathCheck(filePathOfSubject, Obsolete)            //this path is supposed to be deleted and has not changed
	assertPathCheck(filePathOfSubject+".backup", Untracked) //because it neither matches any record nor any path

	os.WriteFile(filePathOfSubject, []byte("NEW"), fs.ModePerm)

	assertPathCheck(filePathOfSubject, Untracked)           //different content at obsoleted path
	assertPathCheck(filePathOfSubject+".backup", Untracked) //because it neither matches any record nor any path

	os.Remove(filePathOfSubject)

	assertPathCheck(filePathOfSubject, Removed)
	assertPathCheck(filePathOfSubject+".backup", Untracked) //because it neither matches any record nor any path

	Lib.MarkDocumentAsObsolete(docPrimaryClone)

	assertPathCheck(filePathOfSubject+".backup", Untracked) //because it neither matches any record nor any path
	assertPathCheck(filePathOfCloneSubject, Obsolete)

	os.Remove(filePathOfCloneSubject)

	assertPathCheck(filePathOfCloneSubject, Removed)

	if _, exists := Lib.GetActiveDocumentByPath(filePathOfSubject); exists {
		t.Fatal("still reported as existing although it has been obsoleted")
	}
	if !Lib.ObsoleteDocumentExistsForPath(filePathOfSubject) {
		t.Fatal("not reported as known path of some obsolete document")
	}
	if !docPrimary.IsObsolete() {
		t.Fatal("not marked as obsolete")
	}

	Lib.ForgetDocument(docPrimary)
	if _, exists := Lib.GetActiveDocumentByPath(filePathOfSubject); exists {
		t.Fatal("forgotten document still known as active")
	}
	if Lib.ObsoleteDocumentExistsForPath(filePathOfSubject) {
		t.Fatal("forgotten document still known as obsolete")
	}

	os.WriteFile(filePathOfSubject, []byte("reborn A"), fs.ModePerm)

	assertPathCheck(filePathOfSubject, Untracked)

	var recordPrintout strings.Builder
	Lib.VisitAllRecords(func(doc document.DocumentApi) {
		recordPrintout.WriteString(doc.String())
		recordPrintout.WriteRune('\n')
	})
	if !strings.Contains(recordPrintout.String(), fileNameAlternative) || strings.Contains(recordPrintout.String(), fileNameSubject) {
		t.Fatal("record printout unexpected:\n" + recordPrintout.String())
	}
}
