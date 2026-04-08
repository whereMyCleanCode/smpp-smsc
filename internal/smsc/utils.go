package smsc

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu/pdutext"
	"golang.org/x/text/encoding/charmap"
)

var (
	GSM7BitCharset = map[rune]bool{
		'@': true, '£': true, '$': true, '¥': true, 'è': true, 'é': true, 'ù': true, 'ì': true,
		'ò': true, 'Ç': true, '\n': true, 'Ø': true, 'ø': true, '\r': true, 'Å': true, 'å': true,
		'Δ': true, '_': true, 'Φ': true, 'Γ': true, 'Λ': true, 'Ω': true, 'Π': true, 'Ψ': true,
		'Σ': true, 'Θ': true, 'Ξ': true, '\x1b': true, 'Æ': true, 'æ': true, 'ß': true, 'É': true,
		' ': true, '!': true, '"': true, '#': true, '¤': true, '%': true, '&': true, '\'': true,
		'(': true, ')': true, '*': true, '+': true, ',': true, '-': true, '.': true, '/': true,
		'0': true, '1': true, '2': true, '3': true, '4': true, '5': true, '6': true, '7': true,
		'8': true, '9': true, ':': true, ';': true, '<': true, '=': true, '>': true, '?': true,
		'¡': true, 'A': true, 'B': true, 'C': true, 'D': true, 'E': true, 'F': true, 'G': true,
		'H': true, 'I': true, 'J': true, 'K': true, 'L': true, 'M': true, 'N': true, 'O': true,
		'P': true, 'Q': true, 'R': true, 'S': true, 'T': true, 'U': true, 'V': true, 'W': true,
		'X': true, 'Y': true, 'Z': true, 'Ä': true, 'Ö': true, 'Ñ': true, 'Ü': true, '§': true,
		'¿': true, 'a': true, 'b': true, 'c': true, 'd': true, 'e': true, 'f': true, 'g': true,
		'h': true, 'i': true, 'j': true, 'k': true, 'l': true, 'm': true, 'n': true, 'o': true,
		'p': true, 'q': true, 'r': true, 's': true, 't': true, 'u': true, 'v': true, 'w': true,
		'x': true, 'y': true, 'z': true, 'ä': true, 'ö': true, 'ñ': true, 'ü': true, 'à': true,
	}
	GSM7BitCharsetExt = map[rune]bool{
		'^': true, '{': true, '}': true, '\\': true, '[': true, '~': true, ']': true, '|': true, '€': true,
	}
	ussdRegex = regexp.MustCompile(`^\*[0-9*#]+#$`)
)

func IsGSM7Bit(s string) bool {
	for _, c := range s {
		if !GSM7BitCharset[c] {
			return false
		}
	}
	return true
}

func IsBinary(s []byte) bool {
	for _, b := range s {
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			return true
		}
	}
	return false
}

func IsUSSD(s string) bool {
	return ussdRegex.MatchString(s)
}

func DetectEncoding(message []byte) (uint8, int) {
	if !IsBinary(message) && IsGSM7Bit(string(message)) {
		return DataCodingDefault, 160
	}
	return DataCodingUCS2, 70
}

func DecodeMessage(message []byte, dataCoding uint8) (string, error) {
	if len(message) == 0 {
		return "", nil
	}

	if len(message) > 1 {
		udhLen := int(message[0])
		if udhLen > 0 && udhLen < len(message) {
			if (udhLen == 5 && message[1] == 0 && message[2] == 3) ||
				(udhLen == 6 && message[1] == 8 && message[2] == 4) {
				message = message[udhLen+1:]
			}
		}
	}

	if len(message) == 0 {
		return "", nil
	}

	if dataCoding == DataCodingDefault && guessUCS2(message) {
		dataCoding = DataCodingUCS2
	}

	switch dataCoding {
	case DataCodingDefault:
		unpacked := true
		for _, b := range message {
			if b > 127 {
				unpacked = false
				break
			}
		}
		if unpacked {
			return string(pdutext.GSM7(message).Decode()), nil
		}
		return string(pdutext.GSM7Packed(message).Decode()), nil
	case DataCodingUCS2:
		if len(message)%2 != 0 {
			return string(message), nil
		}
		buf := bytes.NewReader(message)
		chars := make([]uint16, len(message)/2)
		for i := 0; i < len(chars); i++ {
			if err := binary.Read(buf, binary.BigEndian, &chars[i]); err != nil {
				return string(utf16.Decode(chars[:i])), nil
			}
		}
		return string(utf16.Decode(chars)), nil
	case DataCodingLatin1:
		decoded, err := charmap.Windows1252.NewDecoder().Bytes(message)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	case DataCodingCyrillic:
		decoded, err := charmap.ISO8859_5.NewDecoder().Bytes(message)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	default:
		return string(message), nil
	}
}

func guessUCS2(message []byte) bool {
	if len(message)%2 != 0 {
		return false
	}
	count := 0
	pairs := len(message) / 2
	for i := 0; i < len(message); i += 2 {
		if message[i] <= 0x1F {
			count++
		}
	}
	return count*2 > pairs
}

func ParseTLV(data []byte, offset int) (map[uint16][]byte, error) {
	if offset >= len(data) {
		return nil, nil
	}
	result := make(map[uint16][]byte)
	for offset < len(data) {
		if offset+4 > len(data) {
			break
		}
		tag := binary.BigEndian.Uint16(data[offset : offset+2])
		l := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		offset += 4
		if offset+l > len(data) {
			break
		}
		result[tag] = append([]byte(nil), data[offset:offset+l]...)
		offset += l
	}
	return result, nil
}

func ParseUDH(data []byte) (int, int, int, error) {
	if len(data) < 6 {
		return 0, 0, 0, fmt.Errorf("UDH too short")
	}

	headerLen := int(data[0])
	if len(data) < headerLen+1 {
		return 0, 0, 0, fmt.Errorf("invalid UDH length")
	}

	var messageRef, totalSegments, segmentNumber int
	if data[0] == 5 && data[1] == 0 && data[2] == 3 {
		messageRef = int(data[3])
		totalSegments = int(data[4])
		segmentNumber = int(data[5])
	} else if data[0] == 6 && data[1] == 8 && data[2] == 4 {
		messageRef = (int(data[3]) << 8) | int(data[4])
		totalSegments = int(data[5])
		segmentNumber = int(data[6])
	} else {
		return 0, 0, 0, fmt.Errorf("unsupported UDH format")
	}

	if segmentNumber == 0 || segmentNumber > totalSegments {
		return 0, 0, 0, fmt.Errorf("invalid segment number")
	}

	return messageRef, totalSegments, segmentNumber, nil
}

func CreateUDH(messageRef, totalSegments, segmentNumber int) []byte {
	return []byte{
		5, 0, 3,
		byte(messageRef & 0xFF),
		byte(totalSegments & 0xFF),
		byte(segmentNumber & 0xFF),
	}
}

func CommandIDToString(commandID uint32) string {
	switch commandID {
	case BindTransmitter:
		return "bind_transmitter"
	case BindTransmitterResp:
		return "bind_transmitter_resp"
	case BindReceiver:
		return "bind_receiver"
	case BindReceiverResp:
		return "bind_receiver_resp"
	case BindTransceiver:
		return "bind_transceiver"
	case BindTransceiverResp:
		return "bind_transceiver_resp"
	case Unbind:
		return "unbind"
	case UnbindResp:
		return "unbind_resp"
	case SubmitSM:
		return "submit_sm"
	case SubmitSMResp:
		return "submit_sm_resp"
	case DeliverSM:
		return "deliver_sm"
	case DeliverSMResp:
		return "deliver_sm_resp"
	case EnquireLink:
		return "enquire_link"
	case EnquireLinkResp:
		return "enquire_link_resp"
	case GenericNACK:
		return "generic_nack"
	default:
		return fmt.Sprintf("unknown_0x%08x", commandID)
	}
}

func CommandStatusToString(status uint32) string {
	switch status {
	case StatusOK:
		return "ESME_ROK (ok)"
	case StatusInvMsgLen:
		return "ESME_RINVMSGLEN"
	case StatusInvCmdLen:
		return "ESME_RINVCMDLEN"
	case StatusInvCmdID:
		return "ESME_RINVCMDID"
	case StatusInvBnd:
		return "ESME_RINVBNDSTS"
	case StatusAlyBnd:
		return "ESME_RALYBND"
	case StatusThrottled:
		return "ESME_RTHROTTLED"
	default:
		return fmt.Sprintf("ESME_RUNKNOWNERR (0x%08x)", status)
	}
}

func HexDump(data []byte) string {
	return hex.Dump(data)
}

func FormatPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) == 0 {
		return ""
	}
	re := regexp.MustCompile(`[^0-9+]`)
	phone = re.ReplaceAllString(phone, "")
	if strings.HasPrefix(phone, "+") {
		return phone[1:]
	}
	if len(phone) == 11 && strings.HasPrefix(phone, "8") {
		return "7" + phone[1:]
	}
	return phone
}

func IsNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func BuildMessageIDFromClock(clock uint64, counter uint32) string {
	return fmt.Sprintf("%d%08d", clock, counter)
}

func ParseMessageID(messageID string) (uint64, uint32, error) {
	if len(messageID) <= 8 {
		return 0, 0, fmt.Errorf("invalid message ID format")
	}
	clockStr := messageID[:len(messageID)-8]
	counterStr := messageID[len(messageID)-8:]
	clock, err := strconv.ParseUint(clockStr, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	counter, err := strconv.ParseUint(counterStr, 10, 32)
	if err != nil {
		return 0, 0, err
	}
	return clock, uint32(counter), nil
}

func ReadCString(r io.Reader) (string, error) {
	var result bytes.Buffer
	for {
		var b [1]byte
		if _, err := r.Read(b[:]); err != nil {
			return "", err
		}
		if b[0] == 0 {
			break
		}
		result.WriteByte(b[0])
	}
	return result.String(), nil
}
