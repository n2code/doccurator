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
