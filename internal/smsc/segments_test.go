package smsc

import (
	"testing"
	"time"
)

func TestSegmentsManagerReassemblesMessage(t *testing.T) {
	idGen := &stubIDGenerator{}
	mgr := NewSegmentsManager(newTestLogger(), time.Minute, idGen)

	base := &MessageSegment{
		SegmentGroupID: "group-A",
		SegmentsCount:  3,
		Encoding:       DataCodingDefault,
		RegisteredAt:   time.Now(),
	}

	seg2 := *base
	seg2.SegmentSeqNum = 2
	seg2.Text = []byte("World")

	seg1 := *base
	seg1.SegmentSeqNum = 1
	seg1.Text = []byte("Hello ")

	seg3 := *base
	seg3.SegmentSeqNum = 3
	seg3.Text = []byte("!")

	messageID, status, text, complete, err := mgr.AddSegment(&seg2)
	if err != nil || status != StatusOK || complete || text != "" {
		t.Fatalf("unexpected first add result: id=%d status=%d complete=%v text=%q err=%v", messageID, status, complete, text, err)
	}

	messageID2, status, text, complete, err := mgr.AddSegment(&seg1)
	if err != nil || status != StatusOK || complete || text != "" {
		t.Fatalf("unexpected second add result: id=%d status=%d complete=%v text=%q err=%v", messageID2, status, complete, text, err)
	}

	messageID3, status, text, complete, err := mgr.AddSegment(&seg3)
	if err != nil {
		t.Fatalf("third add error: %v", err)
	}
	if status != StatusOK {
		t.Fatalf("third add status=%d", status)
	}
	if !complete {
		t.Fatalf("message should be complete")
	}
	if text != "Hello World!" {
		t.Fatalf("unexpected reassembled text: %q", text)
	}
	if messageID == 0 || messageID2 != messageID || messageID3 != messageID {
		t.Fatalf("message id mismatch: first=%d second=%d third=%d", messageID, messageID2, messageID3)
	}
}
