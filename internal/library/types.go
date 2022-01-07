package library

import (
	. "github.com/n2code/doccinator/internal/document"
)

type library struct {
	documents    map[DocumentId]DocumentApi
	relPathIndex map[string]DocumentApi
	rootPath     string //absolute, system-native path
}

type orderedDocuments []DocumentApi
type docsByRecordedAndId orderedDocuments
