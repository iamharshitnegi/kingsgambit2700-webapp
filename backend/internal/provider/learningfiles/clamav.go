package learningfiles

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

type Scanner struct{ Address string }

func (s Scanner) Scan(ctx context.Context, data []byte) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", s.Address)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err = conn.SetDeadline(deadline); err != nil {
			return false, err
		}
	}
	if _, err = io.WriteString(conn, "zINSTREAM\x00"); err != nil {
		return false, err
	}
	for len(data) > 0 {
		chunk := data[:min(32<<10, len(data))]
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(chunk)))
		if _, err = conn.Write(size[:]); err != nil {
			return false, err
		}
		if _, err = conn.Write(chunk); err != nil {
			return false, err
		}
		data = data[len(chunk):]
	}
	if _, err = conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return false, err
	}
	result, err := bufio.NewReader(io.LimitReader(conn, 4096)).ReadString('\x00')
	if err != nil {
		return false, err
	}
	result = strings.TrimSpace(strings.TrimSuffix(result, "\x00"))
	if strings.HasSuffix(result, ": OK") {
		return true, nil
	}
	if strings.HasSuffix(result, " FOUND") {
		return false, nil
	}
	return false, fmt.Errorf("malware scanner did not complete the scan")
}
