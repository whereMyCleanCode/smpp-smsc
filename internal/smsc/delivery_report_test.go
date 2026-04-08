package smsc

import (
	"strings"
	"testing"
	"time"
)

func TestFormatDeliveryReceiptString(t *testing.T) {
	r := DeliveryReceiptFields{
		ID:         "1234567890",
		Sub:        "001",
		Dlvrd:      "001",
		SubmitDate: "2304212259",
		DoneDate:   "2304212259",
		Stat:       DeliveryStatDelivered,
		Err:        DeliveryErrOK,
		Text:       "",
	}
	got := FormatDeliveryReceiptString(r)
	want := "id:1234567890 sub:001 dlvrd:001 submit date:2304212259 done date:2304212259 stat:DELIVRD err:000 text:"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildDeliveryReceiptFromPending_SuccessSegments(t *testing.T) {
	submit := time.Date(2023, 4, 21, 22, 59, 0, 0, time.UTC)
	now := time.Date(2023, 4, 21, 23, 0, 0, 0, time.UTC)
	pr := PendingRequest{SegmentsCount: 3, CreatedAt: submit}
	r := BuildDeliveryReceiptFromPending("abc", pr, true, now)
	if r.Sub != "003" || r.Dlvrd != "003" {
		t.Fatalf("success multi-segment: got sub=%s dlvrd=%s", r.Sub, r.Dlvrd)
	}
	if r.Stat != DeliveryStatDelivered || r.Err != DeliveryErrOK {
		t.Fatalf("stat/err: %+v", r)
	}
}

func TestBuildDeliveryReceiptFromPending_FailureSegments(t *testing.T) {
	submit := time.Date(2023, 4, 21, 22, 59, 0, 0, time.UTC)
	now := time.Date(2023, 4, 21, 23, 0, 0, 0, time.UTC)
	pr := PendingRequest{SegmentsCount: 3, CreatedAt: submit}
	r := BuildDeliveryReceiptFromPending("abc", pr, false, now)
	if r.Sub != "003" || r.Dlvrd != "000" {
		t.Fatalf("failure: want sub=003 dlvrd=000, got sub=%s dlvrd=%s", r.Sub, r.Dlvrd)
	}
	if r.Stat != DeliveryStatUndelivered || r.Err != DeliveryErrUndelivered {
		t.Fatalf("stat/err: %+v", r)
	}
}

func TestBuildDeliveryReceiptFromPending_ZeroSegmentsMeansOne(t *testing.T) {
	submit := time.Date(2023, 4, 21, 22, 59, 0, 0, time.UTC)
	now := submit
	pr := PendingRequest{SegmentsCount: 0, CreatedAt: submit}
	r := BuildDeliveryReceiptFromPending("x", pr, true, now)
	if r.Sub != "001" || r.Dlvrd != "001" {
		t.Fatalf("segments 0 treated as 1: got sub=%s dlvrd=%s", r.Sub, r.Dlvrd)
	}
}

func TestShouldSendDeliveryReceipt(t *testing.T) {
	tests := []struct {
		name    string
		flags   RegisteredDeliveryFlags
		success bool
		want    bool
	}{
		{"no_receipt_success", NoReceipt, true, false},
		{"no_receipt_fail", NoReceipt, false, false},
		{"both_success", SuccessAndFailureReceipt, true, true},
		{"both_fail", SuccessAndFailureReceipt, false, true},
		{"success_only_ok", SuccessOnlyReceipt, true, true},
		{"success_only_fail", SuccessOnlyReceipt, false, false},
		{"failure_only_ok", FailureOnlyReceipt, true, false},
		{"failure_only_fail", FailureOnlyReceipt, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.flags.ShouldSendDeliveryReceipt(tt.success)
			if got != tt.want {
				t.Fatalf("flags=%v success=%v got %v want %v", tt.flags, tt.success, got, tt.want)
			}
		})
	}
}

func TestFormatReceiptDateTime(t *testing.T) {
	ts := time.Date(2023, 4, 21, 22, 59, 0, 0, time.UTC)
	got := FormatReceiptDateTime(ts)
	if got != "2304212259" {
		t.Fatalf("got %q want 2304212259", got)
	}
}

func TestBuildDeliveryReceiptFromPendingWithText(t *testing.T) {
	submit := time.Date(2023, 4, 21, 22, 59, 0, 0, time.UTC)
	pr := PendingRequest{SegmentsCount: 1, CreatedAt: submit}
	r := BuildDeliveryReceiptFromPendingWithText("id1", pr, true, submit, "Hello world")
	if !strings.HasSuffix(FormatDeliveryReceiptString(r), "text:Hello world") {
		t.Fatalf("full: %s", FormatDeliveryReceiptString(r))
	}
}
