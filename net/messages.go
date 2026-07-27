package net

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	MagicBytes   uint32 = 0xD9B4BEF9 // LegacyCoin network magic (different from Bitcoin)
	MaxPayload          = 32 * 1024 * 1024 // 32 MB
	HeaderLen           = 24
)

// Message commands
const (
	CmdVersion   = "version"
	CmdVerack    = "verack"
	CmdGetBlocks = "getblocks"
	CmdInv       = "inv"
	CmdGetData   = "getdata"
	CmdBlock     = "block"
	CmdTx        = "tx"
	CmdGetAddr   = "getaddr"
	CmdAddr      = "addr"
	CmdPing      = "ping"
	CmdPong      = "pong"
)

// InvType constants
const (
	InvTypeError uint32 = 0
	InvTypeTx    uint32 = 1
	InvTypeBlock uint32 = 2
)

// MsgHeader is the 24-byte prefix on every P2P message.
type MsgHeader struct {
	Magic    uint32
	Command  [12]byte
	Length   uint32
	Checksum [4]byte
}

// Message is a decoded P2P message.
type Message struct {
	Command string
	Payload []byte
}

// WriteMessage encodes a message to the writer.
func WriteMessage(w io.Writer, cmd string, payload []byte) error {
	hdr := MsgHeader{
		Magic:  MagicBytes,
		Length: uint32(len(payload)),
	}
	copy(hdr.Command[:], cmd)
	copy(hdr.Checksum[:], checksum(payload))

	if err := binary.Write(w, binary.LittleEndian, hdr); err != nil {
		return err
	}
	if len(payload) > 0 {
		_, err := w.Write(payload)
		return err
	}
	return nil
}

// ReadMessage reads and decodes one message from the reader.
func ReadMessage(r io.Reader) (*Message, error) {
	var hdr MsgHeader
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, err
	}
	if hdr.Magic != MagicBytes {
		return nil, fmt.Errorf("wrong magic bytes: %08x", hdr.Magic)
	}
	if hdr.Length > MaxPayload {
		return nil, fmt.Errorf("payload too large: %d", hdr.Length)
	}
	payload := make([]byte, hdr.Length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	// Verify checksum
	got := checksum(payload)
	if !bytes.Equal(got, hdr.Checksum[:]) {
		return nil, errors.New("message checksum mismatch")
	}
	cmd := string(bytes.TrimRight(hdr.Command[:], "\x00"))
	return &Message{Command: cmd, Payload: payload}, nil
}

func checksum(payload []byte) []byte {
	h1 := sha256.Sum256(payload)
	h2 := sha256.Sum256(h1[:])
	return h2[:4]
}

// ── Payload encoders/decoders ─────────────────────────────────────────────────

// VersionPayload is the payload for the "version" message.
type VersionPayload struct {
	Version   int32
	Services  uint64
	Timestamp int64
	AddrRecv  [26]byte
	AddrFrom  [26]byte
	Nonce     uint64
	UserAgent string
	Height    int32
}

func EncodeVersion(v *VersionPayload) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, v.Version)
	binary.Write(&buf, binary.LittleEndian, v.Services)
	binary.Write(&buf, binary.LittleEndian, v.Timestamp)
	buf.Write(v.AddrRecv[:])
	buf.Write(v.AddrFrom[:])
	binary.Write(&buf, binary.LittleEndian, v.Nonce)
	writeVarStr(&buf, v.UserAgent)
	binary.Write(&buf, binary.LittleEndian, v.Height)
	return buf.Bytes()
}

func DecodeVersion(data []byte) (*VersionPayload, error) {
	r := bytes.NewReader(data)
	v := &VersionPayload{}
	var err error
	if err = binary.Read(r, binary.LittleEndian, &v.Version); err != nil {
		return nil, err
	}
	if err = binary.Read(r, binary.LittleEndian, &v.Services); err != nil {
		return nil, err
	}
	if err = binary.Read(r, binary.LittleEndian, &v.Timestamp); err != nil {
		return nil, err
	}
	if _, err = r.Read(v.AddrRecv[:]); err != nil {
		return nil, err
	}
	if _, err = r.Read(v.AddrFrom[:]); err != nil {
		return nil, err
	}
	if err = binary.Read(r, binary.LittleEndian, &v.Nonce); err != nil {
		return nil, err
	}
	v.UserAgent, err = readVarStr(r)
	if err != nil {
		return nil, err
	}
	if err = binary.Read(r, binary.LittleEndian, &v.Height); err != nil {
		return nil, err
	}
	return v, nil
}

// InvVector is a single inventory item.
type InvVector struct {
	Type uint32
	Hash [32]byte
}

func EncodeInv(items []InvVector) []byte {
	var buf bytes.Buffer
	writeVarInt(&buf, uint64(len(items)))
	for _, iv := range items {
		binary.Write(&buf, binary.LittleEndian, iv.Type)
		buf.Write(iv.Hash[:])
	}
	return buf.Bytes()
}

func DecodeInv(data []byte) ([]InvVector, error) {
	r := bytes.NewReader(data)
	count, err := readVarInt(r)
	if err != nil {
		return nil, err
	}
	items := make([]InvVector, count)
	for i := range items {
		if err := binary.Read(r, binary.LittleEndian, &items[i].Type); err != nil {
			return nil, err
		}
		if _, err := r.Read(items[i].Hash[:]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// GetBlocksPayload is the payload for "getblocks".
type GetBlocksPayload struct {
	Version     uint32
	LocatorHashes [][32]byte
	HashStop    [32]byte
}

func EncodeGetBlocks(g *GetBlocksPayload) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, g.Version)
	writeVarInt(&buf, uint64(len(g.LocatorHashes)))
	for _, h := range g.LocatorHashes {
		buf.Write(h[:])
	}
	buf.Write(g.HashStop[:])
	return buf.Bytes()
}

// AddrPayload is a list of peer addresses.
type NetAddr struct {
	Timestamp uint32
	Services  uint64
	IP        [16]byte
	Port      uint16
}

func EncodeAddr(addrs []NetAddr) []byte {
	var buf bytes.Buffer
	writeVarInt(&buf, uint64(len(addrs)))
	for _, a := range addrs {
		binary.Write(&buf, binary.LittleEndian, a.Timestamp)
		binary.Write(&buf, binary.LittleEndian, a.Services)
		buf.Write(a.IP[:])
		binary.Write(&buf, binary.BigEndian, a.Port) // port is big-endian in Bitcoin protocol
	}
	return buf.Bytes()
}

// ── Binary helpers ────────────────────────────────────────────────────────────

func writeVarInt(w *bytes.Buffer, v uint64) {
	switch {
	case v < 0xfd:
		w.WriteByte(byte(v))
	case v <= 0xffff:
		w.WriteByte(0xfd)
		binary.Write(w, binary.LittleEndian, uint16(v))
	case v <= 0xffffffff:
		w.WriteByte(0xfe)
		binary.Write(w, binary.LittleEndian, uint32(v))
	default:
		w.WriteByte(0xff)
		binary.Write(w, binary.LittleEndian, v)
	}
}

func readVarInt(r *bytes.Reader) (uint64, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	switch b {
	case 0xfd:
		var v uint16
		binary.Read(r, binary.LittleEndian, &v)
		return uint64(v), nil
	case 0xfe:
		var v uint32
		binary.Read(r, binary.LittleEndian, &v)
		return uint64(v), nil
	case 0xff:
		var v uint64
		binary.Read(r, binary.LittleEndian, &v)
		return v, nil
	default:
		return uint64(b), nil
	}
}

func writeVarStr(w *bytes.Buffer, s string) {
	writeVarInt(w, uint64(len(s)))
	w.WriteString(s)
}

func readVarStr(r *bytes.Reader) (string, error) {
	n, err := readVarInt(r)
	if err != nil {
		return "", err
	}
	buf := make([]byte, n)
	if _, err := r.Read(buf); err != nil {
		return "", err
	}
	return string(buf), nil
}
