package document

//NOTE: most document functionality is tested in bigger scoped scenario tests, e.g. on library level

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestChangeTimestampUpdating(t *testing.T) {
	const unchangedPlaceholder unixTimestamp = 1

	doc := NewDocument(42)

	if doc.changed == 0 {
		t.Fatal("fresh document missing change timestamp")
	}

	doc.changed = unchangedPlaceholder

	doc.SetPath("dummy")

	if doc.changed == unchangedPlaceholder {
		t.Fatal("change timestamp not updated by setting path")
	}

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
	sourceFilePath := filepath.Join(libRootDir, "dummy")
	os.WriteFile(sourceFilePath, []byte("AAA"), fs.ModePerm)
	doc.changed = unchangedPlaceholder

	doc.UpdateFromFile(sourceFilePath)

	if doc.changed == unchangedPlaceholder {
		t.Fatal("change timestamp not updated by update from file")
	}

	doc.changed = unchangedPlaceholder

	doc.UpdateFromFile(sourceFilePath)

	if doc.changed != unchangedPlaceholder {
		t.Fatal("change timestamp updated without file being changed")
	}

	os.WriteFile(sourceFilePath, []byte("BBB"), fs.ModePerm)
	doc.changed = unchangedPlaceholder

	doc.UpdateFromFile(sourceFilePath)

	if doc.changed == unchangedPlaceholder {
		t.Fatal("change timestamp not updated by change to file")
	}
}

func TestFileStatusVerification(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "doccinator-test-*")
	if err != nil {
		t.Fatal(err)
	}
	libRootDir, err := filepath.Abs(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	os.Chdir(libRootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.RemoveAll(libRootDir)
	}()
	const sourceFileName = "verifile"
	sourceFilePath := filepath.Join(libRootDir, sourceFileName)
	os.WriteFile(sourceFilePath, []byte("AAA"), fs.ModePerm)

	doc := NewDocument(42)
	doc.SetPath(sourceFileName)
	doc.UpdateFromFile(sourceFilePath)

	if doc.VerifyRecordedFileStatus() != UnmodifiedFile {
		t.Fatal("file not considered unmodified")
	}

	correctTimestamp := doc.localStorage.lastModified
	doc.localStorage.lastModified--

	if doc.VerifyRecordedFileStatus() != TouchedFile {
		t.Fatal("file not considered touched")
	}

	doc.localStorage.lastModified = correctTimestamp
	os.WriteFile(sourceFilePath, []byte("B"), fs.ModePerm)

	if doc.VerifyRecordedFileStatus() != ModifiedFile {
		t.Fatal("file not considered modified")
	}

	os.WriteFile(sourceFilePath, []byte("CCC"), fs.ModePerm)

	if doc.VerifyRecordedFileStatus() != ModifiedFile {
		t.Fatal("file not considered modified")
	}

	os.Remove(sourceFilePath)

	if doc.VerifyRecordedFileStatus() != MissingFile {
		t.Fatal("file not considered missing")
	}

	doc.removed = true

	if doc.VerifyRecordedFileStatus() != RemovedFile {
		t.Fatal("file not considered removed")
	}

	os.WriteFile(sourceFilePath, []byte("AAA"), fs.ModePerm)

	if doc.VerifyRecordedFileStatus() != ZombieFile {
		t.Fatal("file not considered zombie")
	}
}
