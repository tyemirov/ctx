package stream

import (
	"encoding/xml"
	"time"

	"github.com/tyemirov/ctx/internal/types"
)

const SchemaVersion = 1

type EventKind string

const (
	EventKindStart        EventKind = "start"
	EventKindDirectory    EventKind = "directory"
	EventKindFile         EventKind = "file"
	EventKindContentChunk EventKind = "content_chunk"
	EventKindSummary      EventKind = "summary"
	EventKindWarning      EventKind = "warning"
	EventKindError        EventKind = "error"
	EventKindTree         EventKind = "tree"
	EventKindDone         EventKind = "done"
)

type DirectoryPhase string

const (
	DirectoryEnter DirectoryPhase = "enter"
	DirectoryLeave DirectoryPhase = "leave"
)

type Event struct {
	XMLName   xml.Name  `json:"-" xml:"event"`
	Version   int       `json:"version" xml:"version,attr"`
	Kind      EventKind `json:"kind" xml:"kind,attr"`
	Command   string    `json:"command,omitempty" xml:"command,attr,omitempty"`
	Path      string    `json:"path,omitempty" xml:"path,attr,omitempty"`
	EmittedAt time.Time `json:"emittedAt,omitempty" xml:"emittedAt,attr,omitempty"`

	Directory *DirectoryEvent       `json:"directory,omitempty" xml:"directory,omitempty"`
	File      *FileEvent            `json:"file,omitempty" xml:"file,omitempty"`
	Chunk     *ChunkEvent           `json:"chunk,omitempty" xml:"chunk,omitempty"`
	Summary   *SummaryEvent         `json:"summary,omitempty" xml:"summary,omitempty"`
	Message   *LogEvent             `json:"message,omitempty" xml:"message,omitempty"`
	Err       *ErrorEvent           `json:"error,omitempty" xml:"error,omitempty"`
	Tree      *types.TreeOutputNode `json:"tree,omitempty" xml:"tree>node,omitempty"`
}

type DirectoryEvent struct {
	Phase        DirectoryPhase `json:"phase" xml:"phase,attr"`
	Path         string         `json:"path" xml:"path,attr"`
	Name         string         `json:"name,omitempty" xml:"name,attr,omitempty"`
	Depth        int            `json:"depth,omitempty" xml:"depth,attr,omitempty"`
	LastModified string         `json:"lastModified,omitempty" xml:"lastModified,attr,omitempty"`
	Summary      *SummaryEvent  `json:"summary,omitempty" xml:"summary,omitempty"`
}

type FileEvent struct {
	Path          string                     `json:"path" xml:"path,attr"`
	Name          string                     `json:"name" xml:"name,attr"`
	Depth         int                        `json:"depth,omitempty" xml:"depth,attr,omitempty"`
	SizeBytes     int64                      `json:"sizeBytes,omitempty" xml:"sizeBytes,attr,omitempty"`
	LastModified  string                     `json:"lastModified,omitempty" xml:"lastModified,attr,omitempty"`
	MimeType      string                     `json:"mimeType,omitempty" xml:"mimeType,attr,omitempty"`
	IsBinary      bool                       `json:"isBinary" xml:"isBinary,attr"`
	Tokens        int                        `json:"tokens,omitempty" xml:"tokens,attr,omitempty"`
	Model         string                     `json:"model,omitempty" xml:"model,attr,omitempty"`
	Type          string                     `json:"type" xml:"type,attr"`
	Documentation []types.DocumentationEntry `json:"documentation,omitempty" xml:"documentation>entry,omitempty"`
}

type ChunkEvent struct {
	Path     string `json:"path" xml:"path,attr"`
	Index    int    `json:"index" xml:"index,attr"`
	Data     string `json:"data,omitempty" xml:"data,omitempty"`
	Encoding string `json:"encoding,omitempty" xml:"encoding,attr,omitempty"`
	IsFinal  bool   `json:"isFinal" xml:"isFinal,attr"`
}

type SummaryEvent struct {
	Files  int    `json:"files" xml:"files,attr"`
	Bytes  int64  `json:"bytes" xml:"bytes,attr"`
	Tokens int    `json:"tokens,omitempty" xml:"tokens,attr,omitempty"`
	Model  string `json:"model,omitempty" xml:"model,attr,omitempty"`
}

type LogEvent struct {
	Level   string `json:"level,omitempty" xml:"level,attr,omitempty"`
	Message string `json:"message" xml:",chardata"`
}

type ErrorEvent struct {
	Message string `json:"message" xml:",chardata"`
}
