package smsc

import (
	"testing"
)

func TestEncodeMessageRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		dc   uint8
		text string
	}{
		{"gsm7_ascii", DataCodingDefault, "hello"},
		{"ucs2", DataCodingUCS2, "Привет"},
		{"latin1", DataCodingLatin1, "café"},
		{"cyrillic8859", DataCodingCyrillic, "Привет"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc, err := EncodeMessage(tc.text, tc.dc)
			if err != nil {
				t.Fatalf("EncodeMessage: %v", err)
			}
			got, err := DecodeMessage(enc, tc.dc)
			if err != nil {
				t.Fatalf("DecodeMessage: %v", err)
			}
			if got != tc.text {
				t.Fatalf("roundtrip: got %q want %q", got, tc.text)
			}
		})
	}
}

func TestEncodeMessageLatin1Unmappable(t *testing.T) {
	_, err := EncodeMessage("Привет", DataCodingLatin1)
	if err == nil {
		t.Fatalf("expected error for unmappable runes")
	}
}
