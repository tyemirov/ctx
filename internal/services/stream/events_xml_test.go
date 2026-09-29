package stream_test

import (
	"encoding/xml"
	"testing"

	"github.com/tyemirov/ctx/internal/services/stream"
	"github.com/tyemirov/ctx/internal/types"
)

func TestEventXMLRoundTrip(testingHandle *testing.T) {
	testCases := []struct {
		name  string
		event stream.Event
	}{
		{name: "start", event: stream.Event{Version: stream.SchemaVersion, Kind: stream.EventKindStart}},
		{name: "tree", event: stream.Event{
			Version: stream.SchemaVersion,
			Kind:    stream.EventKindTree,
			Tree:    &types.TreeOutputNode{Path: "/project", Name: "project", Type: types.NodeTypeDirectory},
		}},
	}
	for _, testCase := range testCases {
		testingHandle.Run(testCase.name, func(testingHandle *testing.T) {
			encoded, marshalError := xml.Marshal(testCase.event)
			if marshalError != nil {
				testingHandle.Fatalf("marshal event: %v", marshalError)
			}
			var decoded stream.Event
			if unmarshalError := xml.Unmarshal(encoded, &decoded); unmarshalError != nil {
				testingHandle.Fatalf("unmarshal event: %v", unmarshalError)
			}
			if decoded.Kind != testCase.event.Kind || decoded.Version != testCase.event.Version {
				testingHandle.Fatalf("event metadata changed: %+v", decoded)
			}
			if testCase.event.Tree != nil && (decoded.Tree == nil || decoded.Tree.Path != testCase.event.Tree.Path) {
				testingHandle.Fatalf("tree payload changed: %+v", decoded.Tree)
			}
		})
	}
}
