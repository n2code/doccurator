package library

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	fileNameA := "file_a"
	fileNameB := "file_b"
	filePathA := filepath.Join(libRootDir, fileNameA)
	filePathB := filepath.Join(libRootDir, fileNameB)

	Lib := MakeRuntimeLibrary()
	Lib.SetRoot(libRootDir)
	Lib.ChdirToRoot()

	assertPathCheck := func(actPath string, expPathStatus PathStatus) {
		checkResult, err := Lib.CheckPath(actPath)
		if err != nil && expPathStatus != Unknown {
			t.Fatalf("got error for path check of %s instead of expected %s: %s", actPath, string(expPathStatus), err)
		}
		if checkResult.status != expPathStatus {
			t.Fatalf("status of %s is not %s as expected, got %s", actPath, string(expPathStatus), string(checkResult.status))
		}
	}

	docB, err := Lib.CreateDocument(2)
	if err != nil || docB == (LibraryDocument{}) {
		t.Fatal("creation of B failed")
	}
	docA, err := Lib.CreateDocument(1)
	if err != nil || docA == (LibraryDocument{}) {
		t.Fatal("creation of A failed")
	}

	docNone, err := Lib.CreateDocument(2)
	if err == nil || docNone != (LibraryDocument{}) {
		t.Fatal("creation not rejected as expected")
	}

	assertPathCheck(filePathA, Unknown)

	os.WriteFile(filePathA, []byte("AAA"), fs.ModePerm)
	os.WriteFile(filePathB, []byte("BB"), fs.ModePerm)

	assertPathCheck(filePathA, Untracked)

	Lib.SetDocumentPath(docA, filePathA)
	Lib.SetDocumentPath(docB, filePathB)

	assertPathCheck(filePathA, Modified)

	Lib.UpdateDocumentFromFile(docA)

	assertPathCheck(filePathA, Tracked)
	assertPathCheck(filepath.Join(libRootDir, "file_which_should_not_exist"), Unknown)

	inTheFuture := time.Now().Add(time.Second)
	os.Chtimes(filePathA, inTheFuture, inTheFuture)

	assertPathCheck(filePathA, Touched)

	Lib.UpdateDocumentFromFile(docA)

	assertPathCheck(filePathA, Tracked)

	os.Rename(filePathA, filePathA+".hidden")

	assertPathCheck(filePathA, Missing)

	os.Rename(filePathA+".hidden", filePathA)

	unrecordedDoc, exists := Lib.GetDocumentByPath(filepath.Join(libRootDir, "file_not_on_record"))
	if unrecordedDoc != (LibraryDocument{}) || exists {
		t.Fatal("unrecorded document not rejected")
	}
	outsideDoc, exists := Lib.GetDocumentByPath(filepath.Join(os.TempDir(), "doccinator-test-dummy"))
	if outsideDoc != (LibraryDocument{}) || exists {
		t.Fatal("document outside library path not rejected")
	}
	secondDocA, exists := Lib.GetDocumentByPath(filePathA)
	if secondDocA != docA || !exists {
		t.Fatal("retrieval of A failed")
	}

	os.Rename(filePathA, filePathA+".renamed")

	assertPathCheck(filePathA+".renamed", Moved)

	os.WriteFile(filePathA, []byte("A+"), fs.ModePerm)

	assertPathCheck(filePathA+".renamed", Untracked)

	os.Rename(filePathA+".renamed", filePathA)

	if docA.IsRemoved() {
		t.Fatal("A is already removed")
	}

	Lib.MarkDocumentAsRemoved(docA)

	assertPathCheck(filePathA, Untracked) //zombie file status (real file not removed yet)

	os.Remove(filePathA)

	assertPathCheck(filePathA, Removed)

	if _, exists := Lib.GetDocumentByPath(filePathA); !exists {
		t.Fatal("removal mark on A went too far")
	}
	if !docA.IsRemoved() {
		t.Fatal("A was not marked as removed")
	}

	Lib.ForgetDocument(docA)
	if _, exists := Lib.GetDocumentByPath(filePathA); exists {
		t.Fatal("A not forgotten")
	}

	// assertPathCheck(filePathA, Untracked)

	allRecords := Lib.AllRecordsAsText()
	if !strings.Contains(allRecords, fileNameB) || strings.Contains(allRecords, fileNameA) {
		t.Fatal("record printout unexpected:\n" + allRecords)
	}
}
