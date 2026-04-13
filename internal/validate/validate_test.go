package validate

import (
	"testing"
)

func TestChannelID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid C prefix", input: "C12345678", wantErr: false},
		{name: "valid D prefix", input: "D12345678", wantErr: false},
		{name: "valid G prefix", input: "G12345678", wantErr: false},
		{name: "valid long ID", input: "C1234567890ABC", wantErr: false},
		{name: "valid all letters", input: "CABCDEFGH", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "too short", input: "C1234567", wantErr: true},
		{name: "lowercase", input: "c12345678", wantErr: true},
		{name: "wrong prefix", input: "X12345678", wantErr: true},
		{name: "plaintext name", input: "general", wantErr: true},
		{name: "has hash prefix", input: "#C12345678", wantErr: true},
		{name: "lowercase body", input: "Cabcdefgh", wantErr: true},
		{name: "user prefix U", input: "U12345678", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ChannelID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ChannelID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestUserID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid U prefix", input: "U12345678", wantErr: false},
		{name: "valid W prefix", input: "W12345678", wantErr: false},
		{name: "valid B prefix", input: "B12345678", wantErr: false},
		{name: "valid long ID", input: "U1234567890ABC", wantErr: false},
		{name: "valid all letters", input: "UABCDEFGH", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "too short", input: "U1234567", wantErr: true},
		{name: "lowercase", input: "u12345678", wantErr: true},
		{name: "wrong prefix", input: "X12345678", wantErr: true},
		{name: "plaintext name", input: "johndoe", wantErr: true},
		{name: "has at prefix", input: "@U12345678", wantErr: true},
		{name: "lowercase body", input: "Uabcdefgh", wantErr: true},
		{name: "channel prefix C", input: "C12345678", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UserID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("UserID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid timestamp", input: "1234567890.123456", wantErr: false},
		{name: "valid all zeros", input: "0000000000.000000", wantErr: false},
		{name: "valid all nines", input: "9999999999.999999", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "missing decimal", input: "1234567890123456", wantErr: true},
		{name: "too few decimal places", input: "1234567890.12345", wantErr: true},
		{name: "too many decimal places", input: "1234567890.1234567", wantErr: true},
		{name: "too few integer digits", input: "123456789.123456", wantErr: true},
		{name: "too many integer digits", input: "12345678901.123456", wantErr: true},
		{name: "letters in integer", input: "123456789a.123456", wantErr: true},
		{name: "letters in decimal", input: "1234567890.12345a", wantErr: true},
		{name: "plaintext", input: "yesterday", wantErr: true},
		{name: "double decimal", input: "1234567890.123.456", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Timestamp(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Timestamp(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestTimeout(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid 1s", input: "1s", wantErr: false},
		{name: "valid 30s", input: "30s", wantErr: false},
		{name: "valid 1m", input: "1m", wantErr: false},
		{name: "valid 5m", input: "5m", wantErr: false},
		{name: "valid 4m59s", input: "4m59s", wantErr: false},
		{name: "valid 300s", input: "300s", wantErr: false},
		{name: "valid 500ms", input: "500ms", wantErr: false},
		{name: "valid 1ns", input: "1ns", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "zero", input: "0s", wantErr: true},
		{name: "negative", input: "-1s", wantErr: true},
		{name: "over 5m", input: "5m1s", wantErr: true},
		{name: "over 5m 6m", input: "6m", wantErr: true},
		{name: "over 5m 301s", input: "301s", wantErr: true},
		{name: "invalid format", input: "five", wantErr: true},
		{name: "invalid format number only", input: "30", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Timeout(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Timeout(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestJSONValue(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid object", input: `{"key":"value"}`, wantErr: false},
		{name: "valid array", input: `[1,2,3]`, wantErr: false},
		{name: "valid string", input: `"hello"`, wantErr: false},
		{name: "valid number", input: `42`, wantErr: false},
		{name: "valid boolean", input: `true`, wantErr: false},
		{name: "valid null", input: `null`, wantErr: false},
		{name: "valid nested", input: `{"a":{"b":[1,2]}}`, wantErr: false},
		{name: "empty string", input: "", wantErr: true},
		{name: "plaintext", input: "hello world", wantErr: true},
		{name: "unclosed brace", input: `{"key":"value"`, wantErr: true},
		{name: "trailing comma", input: `{"a":1,}`, wantErr: true},
		{name: "single quotes", input: `{'key':'value'}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := JSONValue(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("JSONValue(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
