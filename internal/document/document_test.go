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

	if doc.Changed() == 0 {
		t.Fatal("fresh document missing change timestamp")
	}
	initialRecorded := doc.Recorded()
	if initialRecorded == 0 {
		t.Fatal("fresh document missing recorded timestamp")
	}
	if initialRecorded != doc.Changed() {
		t.Fatal("fresh document has different change and recorded timestamp")
	}

	doc.(*document).changed = unchangedPlaceholder

	doc.SetPath("dummy")

	if doc.Changed() == unchangedPlaceholder {
		t.Fatal("change timestamp not updated by setting path")
	}
	if doc.Recorded() != initialRecorded {
		t.Fatal("recorded timestamp changed later")
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
	doc.(*document).changed = unchangedPlaceholder

	doc.UpdateFromFile(sourceFilePath)

	if doc.Changed() == unchangedPlaceholder {
		t.Fatal("change timestamp not updated by update from file")
	}
	if doc.Recorded() != initialRecorded {
		t.Fatal("recorded timestamp changed later")
	}

	doc.(*document).changed = unchangedPlaceholder

	doc.UpdateFromFile(sourceFilePath)

	if doc.Changed() != unchangedPlaceholder {
		t.Fatal("change timestamp updated without file being changed")
	}
	if doc.Recorded() != initialRecorded {
		t.Fatal("recorded timestamp changed later")
	}

	os.WriteFile(sourceFilePath, []byte("BBB"), fs.ModePerm)
	doc.(*document).changed = unchangedPlaceholder

	doc.UpdateFromFile(sourceFilePath)

	if doc.Changed() == unchangedPlaceholder {
		t.Fatal("change timestamp not updated by change to file")
	}
	if doc.Recorded() != initialRecorded {
		t.Fatal("recorded timestamp changed later")
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

	correctTimestamp := doc.(*document).localStorage.lastModified
	doc.(*document).localStorage.lastModified--

	if doc.VerifyRecordedFileStatus() != TouchedFile {
		t.Fatal("file not considered touched")
	}

	doc.(*document).localStorage.lastModified = correctTimestamp
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

	doc.SetRemoved()

	if doc.VerifyRecordedFileStatus() != RemovedFile {
		t.Fatal("file not considered removed")
	}

	os.WriteFile(sourceFilePath, []byte("AAA"), fs.ModePerm)

	if doc.VerifyRecordedFileStatus() != ZombieFile {
		t.Fatal("file not considered zombie")
	}
}
