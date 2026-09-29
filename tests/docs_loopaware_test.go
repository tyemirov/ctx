package tests

import (
	"os"
	"strings"
	"testing"
)

const loopAwareTrafficPixel = "https://loopaware.mprlab.com/pixel.js?site_id=c85b81af-5856-47b2-b77e-df45ffc26462"

func TestPublicDocumentationUsesProductionLoopAwareSite(t *testing.T) {
	documentContent, readError := os.ReadFile("../docs/index.html")
	if readError != nil {
		t.Fatalf("read public documentation: %v", readError)
	}

	if occurrenceCount := strings.Count(string(documentContent), loopAwareTrafficPixel); occurrenceCount != 1 {
		t.Fatalf("LoopAware traffic pixel occurrence count = %d, want 1", occurrenceCount)
	}
}
