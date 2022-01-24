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

	doc.UpdateFromFileOnStorage(libRootDir)

	if doc.Changed() == unchangedPlaceholder {
		t.Fatal("change timestamp not updated by update from file")
	}
	if doc.Recorded() != initialRecorded {
		t.Fatal("recorded timestamp changed later")
	}

	doc.(*document).changed = unchangedPlaceholder

	doc.UpdateFromFileOnStorage(libRootDir)

	if doc.Changed() != unchangedPlaceholder {
		t.Fatal("change timestamp updated without file being changed")
	}
	if doc.Recorded() != initialRecorded {
		t.Fatal("recorded timestamp changed later")
	}

	os.WriteFile(sourceFilePath, []byte("BBB"), fs.ModePerm)
	doc.(*document).changed = unchangedPlaceholder

	doc.UpdateFromFileOnStorage(libRootDir)

	if doc.Changed() == unchangedPlaceholder {
		t.Fatal("change timestamp not updated by change to file")
	}
	if doc.Recorded() != initialRecorded {
		t.Fatal("recorded timestamp changed later")
	}

	doc.(*document).changed = unchangedPlaceholder
	doc.SetRemoved()

	if doc.Changed() == unchangedPlaceholder {
		t.Fatal("change timestamp not updated by obsolete marker")
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
	defer func() {
		os.RemoveAll(libRootDir)
	}()
	const sourceFileName = "verifile"
	sourceFilePath := filepath.Join(libRootDir, sourceFileName)

	doc := NewDocument(42)
	doc.SetPath(sourceFileName)

	_, err = doc.UpdateFromFileOnStorage(libRootDir)
	if err == nil {
		t.Fatal("error expected if file not found")
	}

	os.WriteFile(sourceFilePath, []byte("AAA"), 0o333)

	_, err = doc.UpdateFromFileOnStorage(libRootDir)
	if err == nil {
		t.Fatal("error expected if file unreadable")
	}

	os.Chmod(sourceFilePath, 0o777)

	_, err = doc.UpdateFromFileOnStorage(libRootDir)
	if err != nil {
		t.Fatal("error on updating from existing file")
	}

	if doc.CompareToFileOnStorage(libRootDir) != UnmodifiedFile {
		t.Fatal("file not considered unmodified")
	}

	os.Chmod(sourceFilePath, 0o333)

	if doc.CompareToFileOnStorage(libRootDir) != AccessError { //because file cannot be read
		t.Fatal("error expected if file unreadable")
	}

	os.Chmod(sourceFilePath, 0o777)
	os.Chmod(libRootDir, 0o666)

	if doc.CompareToFileOnStorage(libRootDir) != AccessError { //because stat does not work due to directory permissions
		t.Fatal("error expected if file un-stat-able")
	}

	os.Chmod(libRootDir, 0o777)

	correctTimestamp := doc.(*document).localStorage.lastModified
	doc.(*document).localStorage.lastModified--

	if doc.CompareToFileOnStorage(libRootDir) != TouchedFile {
		t.Fatal("file not considered touched")
	}

	doc.(*document).localStorage.lastModified = correctTimestamp
	os.WriteFile(sourceFilePath, []byte("B"), fs.ModePerm)

	if doc.CompareToFileOnStorage(libRootDir) != ModifiedFile {
		t.Fatal("file not considered modified")
	}

	os.WriteFile(sourceFilePath, []byte("CCC"), fs.ModePerm)

	if doc.CompareToFileOnStorage(libRootDir) != ModifiedFile {
		t.Fatal("file not considered modified")
	}

	os.Remove(sourceFilePath)

	if doc.CompareToFileOnStorage(libRootDir) != NoFileFound {
		t.Fatal("deleted file not considered not-found")
	}

	doc.SetRemoved()

	if doc.CompareToFileOnStorage(libRootDir) != NoFileFound {
		t.Fatal("obsolete file not considered not-found")
	}

	os.WriteFile(sourceFilePath, []byte("AAA"), fs.ModePerm) //content of obsoleted record

	if doc.CompareToFileOnStorage(libRootDir) != UnmodifiedFile { //relies on execution of test within one second (i.e. granularity of modification timestamp)
		t.Fatal("zombie file not considered unmodified")
	}

	os.WriteFile(sourceFilePath, []byte("BOGUS"), fs.ModePerm) //content does not match obsoleted record

	if doc.CompareToFileOnStorage(libRootDir) != ModifiedFile {
		t.Fatal("unexpected file not considered modified")
	}

	os.WriteFile(sourceFilePath, []byte("123"), fs.ModePerm) //equal-length content does not match obsoleted record

	if doc.CompareToFileOnStorage(libRootDir) != ModifiedFile {
		t.Fatal("unexpected equal-length file not considered modified")
	}
}
