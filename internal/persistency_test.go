package internal

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestLibrarySaveAndReload(t *testing.T) {
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
	libraryFileName := "test.lib"
	filePathA := filepath.Join(libRootDir, fileNameA)
	filePathB := filepath.Join(libRootDir, fileNameB)
	libraryFilePath := filepath.Join(libRootDir, libraryFileName)

	Lib := MakeRuntimeLibrary()
	Lib.SetRoot(libRootDir)

	docB, err := Lib.CreateDocument(2)
	if err != nil || docB == nil {
		t.Fatal("creation of B failed")
	}
	docA, err := Lib.CreateDocument(1)
	if err != nil || docA == nil {
		t.Fatal("creation of A failed")
	}

	Lib.SetDocumentPath(docA, filePathA)
	Lib.SetDocumentPath(docB, filePathB)

	os.WriteFile(filePathA, []byte("AAA"), fs.ModePerm)
	os.WriteFile(filePathB, []byte("BB"), fs.ModePerm)

	Lib.ChdirToRoot()
	docA.UpdateFromFile()
	docB.UpdateFromFile()

	Lib.SaveToLocalFile(libraryFilePath, false)

	LoadedLib := MakeRuntimeLibrary()
	LoadedLib.LoadFromLocalFile(libraryFilePath)

	if Lib.AllRecordsAsText() != LoadedLib.AllRecordsAsText() {
		t.Fatalf("library not reloaded correctly\nexpected:\n%s\ngot:\n%s", Lib.AllRecordsAsText(), LoadedLib.AllRecordsAsText())
	}
}
