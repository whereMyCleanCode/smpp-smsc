package smsc

import (
	"fmt"
	"strings"
	"time"
)

// GSM-style delivery receipt status tokens (short message text body).
const (
	DeliveryStatDelivered   = "DELIVRD"
	DeliveryStatUndelivered = "UNDELIV"
)

// Default GSM error codes for receipt text err: field.
const (
	DeliveryErrOK            = "000"
	DeliveryErrUndelivered   = "069"
)

// DeliveryReceiptFields holds id/sub/dlvrd/dates/stat/err/text for a GSM delivery receipt.
type DeliveryReceiptFields struct {
	ID         string
	Sub        string
	Dlvrd      string
	SubmitDate string
	DoneDate   string
	Stat       string
	Err        string
	Text       string
}

// FormatDeliveryReceiptString builds the canonical space-separated delivery receipt short message.
func FormatDeliveryReceiptString(r DeliveryReceiptFields) string {
	return fmt.Sprintf(
		"id:%s sub:%s dlvrd:%s submit date:%s done date:%s stat:%s err:%s text:%s",
		r.ID, r.Sub, r.Dlvrd, r.SubmitDate, r.DoneDate, r.Stat, r.Err, r.Text,
	)
}

// FormatReceiptDateTime formats t as yyMMddHHmm (10 digits, UTC) for submit date / done date fields.
func FormatReceiptDateTime(t time.Time) string {
	return t.UTC().Format("0601021504")
}

func segmentCountForReceipt(segments uint8) int {
	if segments == 0 {
		return 1
	}
	return int(segments)
}

func formatSubDlvrd3(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 999 {
		n = 999
	}
	return fmt.Sprintf("%03d", n)
}

// BuildDeliveryReceiptFromPending fills receipt fields using pending segment count and submit time.
// On success, sub and dlvrd match the submitted segment count; on failure, dlvrd is 000.
func BuildDeliveryReceiptFromPending(messageIDStr string, pr PendingRequest, success bool, now time.Time) DeliveryReceiptFields {
	segs := segmentCountForReceipt(pr.SegmentsCount)
	sub := formatSubDlvrd3(segs)
	var dlvrd string
	if success {
		dlvrd = sub
	} else {
		dlvrd = "000"
	}
	stat := DeliveryStatDelivered
	errCode := DeliveryErrOK
	if !success {
		stat = DeliveryStatUndelivered
		errCode = DeliveryErrUndelivered
	}
	return DeliveryReceiptFields{
		ID:         messageIDStr,
		Sub:        sub,
		Dlvrd:      dlvrd,
		SubmitDate: FormatReceiptDateTime(pr.CreatedAt),
		DoneDate:   FormatReceiptDateTime(now),
		Stat:       stat,
		Err:        errCode,
		Text:       "",
	}
}

// BuildDeliveryReceiptFromPendingWithText is like BuildDeliveryReceiptFromPending but sets text: suffix.
// The text is appended as-is; avoid characters that break downstream parsers if unsure.
func BuildDeliveryReceiptFromPendingWithText(messageIDStr string, pr PendingRequest, success bool, now time.Time, text string) DeliveryReceiptFields {
	r := BuildDeliveryReceiptFromPending(messageIDStr, pr, success, now)
	r.Text = strings.TrimSpace(text)
	return r
}
