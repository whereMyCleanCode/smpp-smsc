package smsc

import (
	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu"
	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu/pdutext"
	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu/pdutlv"
)

const (
	BindTransmitter     uint32 = uint32(pdu.BindTransmitterID)
	BindTransmitterResp uint32 = uint32(pdu.BindTransmitterRespID)
	BindReceiver        uint32 = uint32(pdu.BindReceiverID)
	BindReceiverResp    uint32 = uint32(pdu.BindReceiverRespID)
	BindTransceiver     uint32 = uint32(pdu.BindTransceiverID)
	BindTransceiverResp uint32 = uint32(pdu.BindTransceiverRespID)
	Unbind              uint32 = uint32(pdu.UnbindID)
	UnbindResp          uint32 = uint32(pdu.UnbindRespID)
	SubmitSM            uint32 = uint32(pdu.SubmitSMID)
	SubmitSMResp        uint32 = uint32(pdu.SubmitSMRespID)
	DeliverSM           uint32 = uint32(pdu.DeliverSMID)
	DeliverSMResp       uint32 = uint32(pdu.DeliverSMRespID)
	EnquireLink         uint32 = uint32(pdu.EnquireLinkID)
	EnquireLinkResp     uint32 = uint32(pdu.EnquireLinkRespID)
	GenericNACK         uint32 = uint32(pdu.GenericNACKID)
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
	StatusDeliveryFailure uint32 = 0x000000FE
	StatusUnknownErr      uint32 = 0x000000FF
)

const (
	DataCodingDefault  uint8 = uint8(pdutext.DefaultType)
	DataCodingLatin1   uint8 = uint8(pdutext.Latin1Type)
	DataCodingCyrillic uint8 = uint8(pdutext.ISO88595Type)
	DataCodingUCS2     uint8 = uint8(pdutext.UCS2Type)
)

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
