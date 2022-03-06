package doccurator

import (
	"fmt"
	"github.com/n2code/doccurator/internal"
	"github.com/n2code/doccurator/internal/document"
	"github.com/n2code/doccurator/internal/library"
	"github.com/n2code/doccurator/internal/output"
	"path/filepath"
	"strings"
)

func (d *doccurator) PrintRecord(id document.Id) {
	doc, exists := d.appLib.GetDocumentById(id)
	if exists {
		fmt.Fprintln(d.out, doc)
	}
}

func (d *doccurator) PrintAllRecords(excludeRetired bool) {
	fmt.Fprintf(d.extraOut, "Library: %s\n\n\n", d.appLib.GetRoot())
	count := 0
	d.appLib.VisitAllRecords(func(doc library.Document) {
		if doc.IsObsolete() && excludeRetired {
			return
		}
		fmt.Fprintf(d.out, "%s\n\n", doc)
		count++
	})
	if count == 0 {
		fmt.Fprintln(d.extraOut, "<no records>")
	} else {
		fmt.Fprintf(d.extraOut, "\n%d in total\n", count)
	}
}

func (d *doccurator) PrintTree(excludeUnchanged bool) error {
	tree := output.NewVisualFileTree(d.appLib.GetRoot() + " [library root]")

	var pathsWithErrors []*library.CheckedPath
	paths, ok := d.appLib.Scan(d.isLibFilePath, d.optimizedFsAccess) //full scan may optimize performance if allowed to
	for index, checkedPath := range paths {
		prefix := ""
		status := checkedPath.Status()
		if excludeUnchanged && (status == library.Tracked || status == library.Removed) {
			continue
		}
		if status != library.Tracked {
			prefix = fmt.Sprintf("[%s] ", string(status))
		}
		tree.InsertPath(checkedPath.PathRelativeToLibraryRoot(), prefix)
		if status == library.Error {
			pathsWithErrors = append(pathsWithErrors, &paths[index])
		}
	}
	errorCount := len(pathsWithErrors)

	fmt.Fprint(d.out, tree.Render())

	//TODO [FEATURE]: coloring
	if !ok {
		var msg strings.Builder
		fmt.Fprintf(&msg, "%d scanning %s occurred:\n", errorCount, output.Plural(errorCount, "error", "errors"))
		for _, errorPath := range pathsWithErrors {
			fmt.Fprintf(&msg, "@%s: %s\n", errorPath.PathRelativeToLibraryRoot(), errorPath.GetError())
		}
		return fmt.Errorf(msg.String())
	} else {
		return nil
	}
}

func (d *doccurator) PrintStatus(paths []string) error {
	//TODO [FEATURE]: pair up missing+moved and hide missing
	buckets := make(map[library.PathStatus][]string)

	if len(paths) > 0 {
		fmt.Fprintf(d.extraOut, "Status of %d specified %s:\n", len(paths), output.Plural(paths, "path", "paths"))
	}
	fmt.Fprintln(d.out)

	var errorMessages strings.Builder
	errorCount := 0
	hasChanges := false
	explicitQueryForPaths := len(paths) > 0

	processResult := func(result *library.CheckedPath, absolutePath string) {
		switch status := result.Status(); status {
		case library.Error:
			fmt.Fprintf(&errorMessages, "  [E] @%s\n      %s\n", absolutePath, result.GetError())
			errorCount++
		default:
			buckets[status] = append(buckets[status], mustRelFilepathToWorkingDir(absolutePath))
			if status.RepresentsChange() {
				hasChanges = true
			}
		}
	}

	if explicitQueryForPaths {
		for _, path := range paths {
			abs := mustAbsFilepath(path)
			result := d.appLib.CheckFilePath(abs, false) //explicit status query must not sacrifice correctness for performance
			processResult(&result, abs)
		}
	} else {
		results, _ := d.appLib.Scan(d.isLibFilePath, d.optimizedFsAccess) //full scan may optimize performance if allowed to
		for _, result := range results {
			processResult(&result, filepath.Join(d.appLib.GetRoot(), result.PathRelativeToLibraryRoot()))
		}
	}

	//TODO [FEATURE]: coloring

	for _, status := range []library.PathStatus{
		library.Tracked,
		library.Removed,
		library.Obsolete,
		library.Duplicate,
		library.Untracked,
		library.Touched,
		library.Moved,
		library.Modified,
		library.Missing,
		library.Error,
	} {
		bucket := buckets[status]
		if len(bucket) == 0 || (!status.RepresentsChange() && !explicitQueryForPaths) {
			continue //to hide empty buckets and unchanged files [when no explicit paths are queried]
		}
		fmt.Fprintf(d.out, " %s (%d %s)\n", status, len(bucket), output.Plural(bucket, "file", "files"))
		for _, path := range bucket {
			fmt.Fprintf(d.out, "  [%s] %s\n", string(rune(status)), path)
		}
		fmt.Fprintln(d.out)
	}
	if errorCount > 0 {
		fmt.Fprintf(d.out, " %s occurred:\n%s\n", output.Plural(errorCount, "Error", "Errors"), errorMessages.String()) //not on stderr because it was explicitly queried
	} else if hasChanges == false && len(paths) == 0 {
		fmt.Fprint(d.out, " Library files in sync with all records.\n\n")
	}
	return nil
}

func (d *doccurator) SearchByIdPart(part string) (results []SearchResult) {
	partInUpper := strings.ToUpper(part)
	d.appLib.VisitAllRecords(func(doc library.Document) {
		if id := doc.Id(); strings.Contains(id.String(), partInUpper) {
			absolute := filepath.Join(d.appLib.GetRoot(), doc.PathRelativeToLibraryRoot())
			relative := mustRelFilepathToWorkingDir(absolute)
			results = append(results, SearchResult{
				Id:           id,
				RelativePath: relative,
				StatusText:   d.appLib.CheckFilePath(absolute, d.optimizedFsAccess).Status().String()})
		}
	})
	return
}

func (d *doccurator) GetFreeId() document.Id {
	candidate := internal.UnixTimestampNow()
	for candidate > 0 {
		id := document.Id(candidate)
		if _, exists := d.appLib.GetDocumentById(id); !exists {
			return id
		}
		candidate--
	}
	return document.MissingId
}

func (d *doccurator) isLibFilePath(path string) bool {
	return path == d.libFile
}
