package library

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLibrarySaveAndReload(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "doccurator-test-*")
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
	fileNameC := "file_c"
	libraryFileName := "test.lib"
	filePathA := filepath.Join(libRootDir, fileNameA)
	filePathB := filepath.Join(libRootDir, fileNameB)
	filePathC := filepath.Join(libRootDir, fileNameC)
	libraryFilePath := filepath.Join(libRootDir, libraryFileName)

	Lib := NewLibrary()
	Lib.SetRoot(libRootDir)

	docB, err := Lib.CreateDocument(2)
	if err != nil || docB == (Document{}) {
		t.Fatal("creation of B failed")
	}
	docA, err := Lib.CreateDocument(1)
	if err != nil || docA == (Document{}) {
		t.Fatal("creation of A failed")
	}
	docC, err := Lib.CreateDocument(3)
	if err != nil || docC == (Document{}) {
		t.Fatal("creation of C failed")
	}

	Lib.SetDocumentPath(docA, filePathA)
	Lib.SetDocumentPath(docB, filePathB)
	Lib.SetDocumentPath(docC, filePathC)

	os.WriteFile(filePathA, []byte("AAA"), fs.ModePerm)
	os.WriteFile(filePathB, []byte("BB"), fs.ModePerm)
	os.WriteFile(filePathC, []byte("C"), fs.ModePerm)

	Lib.UpdateDocumentFromFile(docA)
	Lib.UpdateDocumentFromFile(docB)
	Lib.UpdateDocumentFromFile(docC)
	Lib.MarkDocumentAsObsolete(docC)

	Lib.SaveToLocalFile(libraryFilePath, false)

	LoadedLib := NewLibrary()
	LoadedLib.LoadFromLocalFile(libraryFilePath)

	var originalLibRecords strings.Builder
	var loadedLibRecords strings.Builder
	Lib.VisitAllRecords(func(doc Document) {
		originalLibRecords.WriteString(doc.String())
		originalLibRecords.WriteRune('\n')
	})
	LoadedLib.VisitAllRecords(func(doc Document) {
		loadedLibRecords.WriteString(doc.String())
		loadedLibRecords.WriteRune('\n')
	})
	if originalLibRecords.String() != loadedLibRecords.String() {
		t.Fatalf("library not reloaded correctly\nexpected:\n%s\ngot:\n%s", originalLibRecords.String(), loadedLibRecords.String())
	}
}
