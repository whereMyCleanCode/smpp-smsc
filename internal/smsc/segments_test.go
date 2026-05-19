package smsc

import (
	"testing"
	"time"
)

func TestSegmentsManagerReassemblesMessage(t *testing.T) {
	idGen := &stubIDGenerator{}
	mgr := NewSegmentsManager(newTestLogger(), time.Minute, idGen, 10)

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

	messageID, status, text, complete, dlrRequested, err := mgr.AddSegment(&seg2)
	if err != nil || status != StatusOK || complete || text != "" {
		t.Fatalf("unexpected first add result: id=%d status=%d complete=%v text=%q err=%v", messageID, status, complete, text, err)
	}
	if dlrRequested {
		t.Fatalf("unexpected dlr request on first segment")
	}

	messageID2, status, text, complete, dlrRequested, err := mgr.AddSegment(&seg1)
	if err != nil || status != StatusOK || complete || text != "" {
		t.Fatalf("unexpected second add result: id=%d status=%d complete=%v text=%q err=%v", messageID2, status, complete, text, err)
	}
	if dlrRequested {
		t.Fatalf("unexpected dlr request on second segment")
	}

	messageID3, status, text, complete, dlrRequested, err := mgr.AddSegment(&seg3)
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
	if dlrRequested {
		t.Fatalf("unexpected dlr request for this test case")
	}
	if messageID == 0 || messageID2 != messageID || messageID3 != messageID {
		t.Fatalf("message id mismatch: first=%d second=%d third=%d", messageID, messageID2, messageID3)
	}
}

func TestSegmentsManagerReassemblesMessageTwelveSegments(t *testing.T) {
	const n = 12
	idGen := &stubIDGenerator{}
	mgr := NewSegmentsManager(newTestLogger(), time.Minute, idGen, n)

	base := &MessageSegment{
		SegmentGroupID: "group-12",
		SegmentsCount:  n,
		Encoding:       DataCodingDefault,
		RegisteredAt:   time.Now(),
	}

	var want string
	for i := 1; i <= n; i++ {
		want += string(rune('A' + i - 1))
	}

	for i := 1; i <= n; i++ {
		seg := *base
		seg.SegmentSeqNum = uint8(i)
		seg.Text = []byte{byte('A' + i - 1)}
		_, status, text, complete, _, err := mgr.AddSegment(&seg)
		if err != nil {
			t.Fatalf("segment %d add error: %v", i, err)
		}
		if status != StatusOK {
			t.Fatalf("segment %d status=%d", i, status)
		}
		if i < n && complete {
			t.Fatalf("segment %d should not be complete yet", i)
		}
		if i < n && text != "" {
			t.Fatalf("segment %d unexpected text %q", i, text)
		}
		if i == n {
			if !complete {
				t.Fatalf("last segment should complete")
			}
			if text != want {
				t.Fatalf("reassembled text: got %q want %q", text, want)
			}
		}
	}
}
