package resolver

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
)

func TestKeyEncoderFuncNil(t *testing.T) {
	t.Parallel()

	var encoder KeyEncoderFunc[int]

	_, err := encoder.Encode(1)
	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/topology/shard/resolver: key encoder function is nil"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestKeyEncoderFunc(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("encode failed")
	encoder := KeyEncoderFunc[int](func(key int) ([]byte, error) {
		if key < 0 {
			return nil, sentinel
		}

		return []byte{byte(key)}, nil
	})

	encoded, err := encoder.Encode(7)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if got, want := encoded, []byte{7}; !bytes.Equal(got, want) {
		t.Fatalf("Encode() = %v, want %v", got, want)
	}

	if _, err := encoder.Encode(-1); !errors.Is(err, sentinel) {
		t.Fatalf("Encode() error = %v, want sentinel", err)
	}
}

func TestStringKeyEncoder(t *testing.T) {
	t.Parallel()

	encoded, err := StringKeyEncoder().Encode("a\x00b")
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	if got, want := string(encoded), "a\x00b"; got != want {
		t.Fatalf("Encode() = %q, want %q", got, want)
	}
}

func TestBytesKeyEncoderReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()

	key := []byte{1, 2, 3}
	encoded, err := BytesKeyEncoder().Encode(key)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	encoded[0] = 9

	if got, want := key[0], byte(1); got != want {
		t.Fatalf("input key changed to %d, want %d", got, want)
	}
}

func TestIntegerKeyEncoders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  func() ([]byte, error)
		want string
	}{
		{
			name: "int64 positive",
			got:  func() ([]byte, error) { return Int64KeyEncoder().Encode(1) },
			want: "0000000000000001",
		},
		{
			name: "int64 negative",
			got:  func() ([]byte, error) { return Int64KeyEncoder().Encode(-1) },
			want: "ffffffffffffffff",
		},
		{
			name: "uint64",
			got:  func() ([]byte, error) { return Uint64KeyEncoder().Encode(0x0102030405060708) },
			want: "0102030405060708",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := test.got()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}

			if got := hex.EncodeToString(encoded); got != test.want {
				t.Fatalf("Encode() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestFixedSizeKeyEncoders(t *testing.T) {
	t.Parallel()

	var key16 [16]byte
	for index := range key16 {
		key16[index] = byte(index)
	}

	encoded16, err := Bytes16KeyEncoder().Encode(key16)
	if err != nil {
		t.Fatalf("Bytes16KeyEncoder.Encode() error = %v", err)
	}
	if got, want := encoded16, key16[:]; !bytes.Equal(got, want) {
		t.Fatalf("Bytes16KeyEncoder.Encode() = %v, want %v", got, want)
	}

	var key32 [32]byte
	for index := range key32 {
		key32[index] = byte(31 - index)
	}

	encoded32, err := Bytes32KeyEncoder().Encode(key32)
	if err != nil {
		t.Fatalf("Bytes32KeyEncoder.Encode() error = %v", err)
	}
	if got, want := encoded32, key32[:]; !bytes.Equal(got, want) {
		t.Fatalf("Bytes32KeyEncoder.Encode() = %v, want %v", got, want)
	}
}
