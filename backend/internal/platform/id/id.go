package id

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"regexp"
	"time"
)

var uuidV7Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var referencePattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[47][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// New returns an RFC 9562 UUIDv7 using UTC Unix milliseconds and cryptographic randomness.
func New() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	millis := uint64(time.Now().UTC().UnixMilli())
	value[0] = byte(millis >> 40)
	value[1] = byte(millis >> 32)
	value[2] = byte(millis >> 24)
	value[3] = byte(millis >> 16)
	value[4] = byte(millis >> 8)
	value[5] = byte(millis)
	value[6] = (value[6] & 0x0f) | 0x70
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(value[0:4]),
		binary.BigEndian.Uint16(value[4:6]),
		binary.BigEndian.Uint16(value[6:8]),
		binary.BigEndian.Uint16(value[8:10]),
		value[10:16],
	)
}

func Valid(value string) bool { return uuidV7Pattern.MatchString(value) }

// ValidReference accepts existing UUIDv4 records as well as current UUIDv7 records.
// New identifiers still use New; identifier format never grants authorization.
func ValidReference(value string) bool { return referencePattern.MatchString(value) }
