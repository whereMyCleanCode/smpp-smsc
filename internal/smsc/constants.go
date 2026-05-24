package smsc

import (
	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu"
	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu/pdutext"
	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu/pdutlv"
)

const (
	BindTransmitter     uint32 = 0x00000002
	BindTransmitterResp uint32 = 0x80000002
	BindReceiver        uint32 = 0x00000001
	BindReceiverResp    uint32 = 0x80000001
	BindTransceiver     uint32 = 0x00000009
	BindTransceiverResp uint32 = 0x80000009
	Unbind              uint32 = 0x00000006
	UnbindResp          uint32 = 0x80000006
	SubmitSM            uint32 = 0x00000004
	SubmitSMResp        uint32 = 0x80000004
	DeliverSM           uint32 = 0x00000005
	DeliverSMResp       uint32 = 0x80000005
	QuerySM             uint32 = 0x00000003
	QuerySMResp         uint32 = 0x80000003
	ReplaceSM           uint32 = 0x00000007
	ReplaceSMResp       uint32 = 0x80000007
	CancelSM            uint32 = 0x00000008
	CancelSMResp        uint32 = 0x80000008
	EnquireLink         uint32 = 0x00000015
	EnquireLinkResp     uint32 = 0x80000015
	GenericNACK         uint32 = 0x80000000
)

const (
	StatusOK              uint32 = 0x00000000
	StatusInvMsgLen       uint32 = 0x00000001
	StatusInvCmdLen       uint32 = 0x00000002
	StatusInvCmdID        uint32 = 0x00000003
	StatusInvBnd          uint32 = 0x00000004
	StatusAlyBnd          uint32 = 0x00000005
	StatusInvRegDlvFlg    uint32 = 0x00000007
	StatusSysErr          uint32 = 0x00000008
	StatusInvSrcAdr       uint32 = 0x0000000A
	StatusInvDstAdr       uint32 = 0x0000000B
	StatusInvDestFlag     uint32 = 0x00000040
	StatusInvMsgID        uint32 = 0x0000000C
	StatusBindFail        uint32 = 0x0000000D
	StatusInvPaswd        uint32 = 0x0000000E
	StatusInvSysID        uint32 = 0x0000000F
	StatusSubmitFail      uint32 = 0x00000045
	StatusInvDataCoding   uint32 = 0x00000010
	StatusThrottled       uint32 = 0x00000058
	StatusInvOptParamVal  uint32 = 0x000000C4
	StatusCancelSmFailed  uint32 = 0x00000011
	StatusReplaceSmFailed uint32 = 0x00000013
	StatusQuerySmFailed   uint32 = 0x00000067
)

const (
	DataCodingDefault  uint8 = uint8(pdutext.DefaultType)
	DataCodingLatin1   uint8 = uint8(pdutext.Latin1Type)
	DataCodingCyrillic uint8 = uint8(pdutext.ISO88595Type)
	DataCodingUCS2     uint8 = uint8(pdutext.UCS2Type)
)

// ESMClassDeliveryReceipt is deliver_sm carrying a GSM short message delivery receipt (SMPP message type).
const ESMClassDeliveryReceipt uint8 = 0x04

const (
	PDUHeaderSize = pdu.HeaderLen
	MinPDUSize    = pdu.HeaderLen
	MaxPDUSize    = pdu.MaxSize
)

const (
	TagMessagePayload    uint16 = uint16(pdutlv.TagMessagePayload)
	TagSarMsgRefNum      uint16 = uint16(pdutlv.TagSarMsgRefNum)
	TagSarTotalSegments  uint16 = uint16(pdutlv.TagSarTotalSegments)
	TagSarSegmentSeqnum  uint16 = uint16(pdutlv.TagSarSegmentSeqnum)
	TagLanguageIndicator uint16 = uint16(pdutlv.TagLanguageIndicator)
	TagTemplateID        uint16 = 0x0110
)
