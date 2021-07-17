package internal

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
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
	tmpDir, _ := os.MkdirTemp("", "doccinator-test-*")
	libRootDir, _ := filepath.Abs(tmpDir)
	defer func() {
		os.RemoveAll(libRootDir)
	}()
	filePathA := filepath.Join(libRootDir, "file_a")
	filePathB := filepath.Join(libRootDir, "file_b")

	Lib := MakeLibrary(libRootDir)

	docB, err := Lib.CreateDocument(2)
	if err != nil || docB == nil {
		t.Fatal("creation of B failed")
	}
	docA, err := Lib.CreateDocument(1)
	if err != nil || docA == nil {
		t.Fatal("creation of A failed")
	}

	docNone, err := Lib.CreateDocument(2)
	if err == nil || docNone != nil {
		t.Fatal("creation not rejected as expected")
	}

	Lib.SetDocumentPath(docA, filePathA)
	Lib.SetDocumentPath(docB, filePathB)

	os.WriteFile(filePathA, []byte("AAA"), fs.ModePerm)
	os.WriteFile(filePathB, []byte("BB"), fs.ModePerm)

	Lib.ChdirToRoot()
	docA.UpdateFromFile()

	unrecordedDoc, exists := Lib.GetDocumentByPath(filepath.Join(libRootDir, "file_not_on_record"))
	if unrecordedDoc != nil || exists {
		t.Fatal("unrecorded document not rejected")
	}
	outsideDoc, exists := Lib.GetDocumentByPath(filepath.Join(os.TempDir(), "doccinator-test-dummy"))
	if outsideDoc != nil || exists {
		t.Fatal("document outside library path not rejected")
	}
	secondDocA, exists := Lib.GetDocumentByPath(filePathA)
	if secondDocA != docA || !exists {
		t.Fatal("retrieval of A failed")
	}

	err = Lib.RemoveDocument(docA)
	if _, exists := Lib.GetDocumentByPath(filePathA); err != nil || exists {
		t.Fatal("A not deleted")
	}
	err = Lib.RemoveDocument(docA)
	if err == nil {
		t.Fatal("second attempt to delete A did not fail")
	}
}
