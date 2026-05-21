package storage

import (
	"crypto/rand"
	"encoding/binary"
)

// NewLinkUUID возвращает псевдослучайный положительный идентификатор новой записи.
// Не зависит от хранилища и безопасен при конкурентных запросах.
func NewLinkUUID() (int, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	n := int(binary.BigEndian.Uint32(b[:]) & 0x7fffffff)
	if n == 0 {
		n = 1
	}
	return n, nil
}
