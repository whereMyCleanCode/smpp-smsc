package smsc

import (
	"encoding/binary"
	"fmt"
	"time"
)

type BindingType uint8

const (
	BindingTypeNone BindingType = iota
	BindingTypeTransceiver
	BindingTypeTransmitter
	BindingTypeReceiver
)

func (bt BindingType) String() string {
	switch bt {
	case BindingTypeTransceiver:
		return "transceiver"
	case BindingTypeTransmitter:
		return "transmitter"
	case BindingTypeReceiver:
		return "receiver"
	default:
		return "none"
	}
}

func (bt BindingType) IsReceiver() bool {
	return bt == BindingTypeReceiver || bt == BindingTypeTransceiver
}

func (bt BindingType) IsTransmitter() bool {
	return bt == BindingTypeTransmitter || bt == BindingTypeTransceiver
}

type RegisteredDeliveryFlags uint8

const (
	NoReceipt                RegisteredDeliveryFlags = 0x00
	SuccessAndFailureReceipt RegisteredDeliveryFlags = 0x01
	FailureOnlyReceipt       RegisteredDeliveryFlags = 0x02
	SuccessOnlyReceipt       RegisteredDeliveryFlags = 0x03
)

func (r RegisteredDeliveryFlags) GetReceiptType() RegisteredDeliveryFlags {
	return r & 0x03
}

type SubmitSmParams struct {
	MessageID uint64

	RegisteredDelivery uint8
	ValidityPeriod     uint8
	DataCoding         uint8

	SeqNum uint32

	ServiceType  string
	SourceAddr   string
	DestAddr     string
	ShortMessage []byte
	Text         string
	WithPayload  bool

	Segment *MessageSegment

	TemplateID *uint64

	TLVParams map[uint16][]byte
}

func (p *SubmitSmParams) GetTLVString(tag uint16) (string, bool) {
	v, ok := p.TLVParams[tag]
	if !ok {
		return "", false
	}
	return string(v), true
}

func (p *SubmitSmParams) GetTLVByte(tag uint16) (byte, bool) {
	v, ok := p.TLVParams[tag]
	if !ok || len(v) < 1 {
		return 0, false
	}
	return v[0], true
}

func (p *SubmitSmParams) GetTLVUint16(tag uint16) (uint16, bool) {
	v, ok := p.TLVParams[tag]
	if !ok {
		return 0, false
	}
	switch len(v) {
	case 1:
		return uint16(v[0]), true
	case 2:
		return binary.BigEndian.Uint16(v), true
	default:
		return 0, false
	}
}

func (p *SubmitSmParams) GetTLVUint32(tag uint16) (uint32, bool) {
	v, ok := p.TLVParams[tag]
	if !ok || len(v) < 4 {
		return 0, false
	}
	return binary.BigEndian.Uint32(v), true
}

func (p *SubmitSmParams) GetTLVUint64(tag uint16) (uint64, bool) {
	v, ok := p.TLVParams[tag]
	if !ok || len(v) < 8 {
		return 0, false
	}
	return binary.BigEndian.Uint64(v), true
}

func (p *SubmitSmParams) GetTLVBytes(tag uint16) ([]byte, bool) {
	v, ok := p.TLVParams[tag]
	if !ok {
		return nil, false
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, true
}

type PendingRequest struct {
	SegmentsCount uint8
	CreatedAt     time.Time
}

type MessageSegment struct {
	MessageID      uint64
	SegmentGroupID string
	MessageRefNum  uint8
	SegmentSeqNum  uint8
	SegmentsCount  uint8
	Text           []byte
	Encoding       uint8
	RegisteredAt   time.Time
}

type DeliveryReportResult uint8

const (
	DeliveryReportSent DeliveryReportResult = iota
	DeliveryReportSkippedNoReceipt
	DeliveryReportSkippedSuccessOnly
	DeliveryReportSkippedFailureOnly
	DeliveryReportSkippedSessionClosed
	DeliveryReportSkippedQueueFull
)

func (r DeliveryReportResult) String() string {
	switch r {
	case DeliveryReportSent:
		return "SENT"
	case DeliveryReportSkippedNoReceipt:
		return "SKIPPED_NO_RECEIPT"
	case DeliveryReportSkippedSuccessOnly:
		return "SKIPPED_SUCCESS_ONLY"
	case DeliveryReportSkippedFailureOnly:
		return "SKIPPED_FAILURE_ONLY"
	case DeliveryReportSkippedSessionClosed:
		return "SKIPPED_SESSION_CLOSED"
	case DeliveryReportSkippedQueueFull:
		return "SKIPPED_QUEUE_FULL"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", r)
	}
}

type SmppResponse struct {
	Msg    string
	Status uint32
}
