package document

//NOTE: most document functionality is tested in bigger scoped scenario tests, e.g. on library level

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
	doc.DeclareObsolete()

	if doc.Changed() == unchangedPlaceholder {
		t.Fatal("change timestamp not updated by obsolete marker")
	}
}

func TestFileStatusVerification(t *testing.T) {
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

	if doc.CompareToFileOnStorage(libRootDir, false) != UnmodifiedFile {
		t.Fatal("file not considered unmodified")
	}

	os.Chmod(sourceFilePath, 0o333)

	if doc.CompareToFileOnStorage(libRootDir, false) != FileAccessError { //because file cannot be read
		t.Fatal("error expected if file unreadable")
	}

	os.Chmod(sourceFilePath, 0o777)
	os.Chmod(libRootDir, 0o666)

	if doc.CompareToFileOnStorage(libRootDir, false) != FileAccessError { //because stat does not work due to directory permissions
		t.Fatal("error expected if file un-stat-able")
	}

	os.Chmod(libRootDir, 0o777)

	correctTimestamp := doc.(*document).localStorage.lastModified
	doc.(*document).localStorage.lastModified--

	if doc.CompareToFileOnStorage(libRootDir, false) != TouchedFile {
		t.Fatal("file not considered touched")
	}

	doc.(*document).localStorage.lastModified = correctTimestamp
	os.WriteFile(sourceFilePath, []byte("B"), fs.ModePerm)

	if doc.CompareToFileOnStorage(libRootDir, false) != ModifiedFile {
		t.Fatal("file not considered modified")
	}

	os.WriteFile(sourceFilePath, []byte("CCC"), fs.ModePerm)

	if doc.CompareToFileOnStorage(libRootDir, false) != ModifiedFile {
		t.Fatal("file not considered modified")
	}

	os.Remove(sourceFilePath)

	if doc.CompareToFileOnStorage(libRootDir, false) != NoFileFound {
		t.Fatal("deleted file not considered not-found")
	}

	doc.DeclareObsolete()

	if doc.CompareToFileOnStorage(libRootDir, false) != NoFileFound {
		t.Fatal("obsolete file not considered not-found")
	}

	os.WriteFile(sourceFilePath, []byte("AAA"), fs.ModePerm) //content of obsoleted record

	if doc.CompareToFileOnStorage(libRootDir, false) != UnmodifiedFile { //relies on execution of test within one second (i.e. granularity of modification timestamp)
		t.Fatal("zombie file not considered unmodified")
	}

	os.WriteFile(sourceFilePath, []byte("BOGUS"), fs.ModePerm) //content does not match obsoleted record

	if doc.CompareToFileOnStorage(libRootDir, false) != ModifiedFile {
		t.Fatal("unexpected file not considered modified")
	}

	os.WriteFile(sourceFilePath, []byte("123"), fs.ModePerm) //equal-length content does not match obsoleted record

	if doc.CompareToFileOnStorage(libRootDir, false) != ModifiedFile {
		t.Fatal("unexpected equal-length file not considered modified")
	}
}

func TestStandardizingFilenames(t *testing.T) {
	someId := Id(42)
	assertRepeatedlyStandardizableAndReversible := func(filename string, expectedAfterStandardization string) {
		cut := NewDocument(someId)
		cut.SetPath("fake_dir/" + filename)
		act, err := cut.StandardizedFilename()

		//single standardization
		switch {
		case err != nil:
			t.Error("standardization of", filename, "yields unexpected error: ", err)
			return
		case act != expectedAfterStandardization:
			t.Error("standardization of", filename, "yields", act, "but expectation is", expectedAfterStandardization)
			return
		}

		//repeated standardization
		actRepeated, err := cut.StandardizedFilename()
		switch {
		case err != nil:
			t.Error("repeated standardization of standardized name ", act, "yields unexpected error: ", err)
		case actRepeated != act:
			t.Error("repeated standardization of standardized name ", act, "yields", actRepeated, "but expectation is no change")
		}

		//reverse
		originalExtractor := regexp.MustCompile(`(.*)\.[^.]+\.ndoc(?:\.[^.]*)?`)
		if matches := originalExtractor.FindStringSubmatch(act); matches[1] != filename {
			t.Error("standardized name ", act, "could not be reversed to", filename, "(got", matches[1], "instead)")
		}
	}

	assertRepeatedlyStandardizableAndReversible("name.ext", "name.ext."+someId.String()+".ndoc.ext")
	assertRepeatedlyStandardizableAndReversible("name_only", "name_only."+someId.String()+".ndoc")
	assertRepeatedlyStandardizableAndReversible(".ext_only", ".ext_only."+someId.String()+".ndoc.ext_only")
	assertRepeatedlyStandardizableAndReversible("name.ext1.ext2", "name.ext1.ext2."+someId.String()+".ndoc.ext2")
	assertRepeatedlyStandardizableAndReversible("name.", "name.."+someId.String()+".ndoc.")

	//verify detection of non-matching IDs in filenames
	correctID := Id(777)
	differentID := Id(13)
	irregularDoc := NewDocument(correctID)
	irregularDoc.SetPath("fake_dir/problematic.ext." + differentID.String() + ".ndoc.ext")
	if _, err := irregularDoc.StandardizedFilename(); err == nil {
		t.Error("already-standardized filename not recognized to have unexpected ID: ", err)
	}
}
