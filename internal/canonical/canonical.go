package canonical

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
)

func Bytes(value any) ([]byte, error) {
	return json.Marshal(value)
}

func MustBytes(value any) []byte {
	bytes, err := Bytes(value)
	if err != nil {
		panic(err)
	}
	return bytes
}

func Hash(value any) string {
	return HashBytes(MustBytes(value))
}

func HashBytes(bytes []byte) string {
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func WriteJSON(writer io.Writer, value any) error {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if _, err := writer.Write(bytes); err != nil {
		return err
	}
	_, err = writer.Write([]byte("\n"))
	return err
}
