package tcmu

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/coreos/go-tcmu/scsi"
	"github.com/prometheus/common/log"
)

// SCSICmdHandler is a simple request/response handler for SCSI commands coming to TCMU.
// A SCSI error is reported as an SCSIResponse with an error bit set, while returning a Go error is for flagrant, process-ending errors (OOM, perhaps).
type SCSICmdHandler interface {
	HandleCommand(cmd *SCSICmd) (SCSIResponse, error)
}

type ReadWriterAtCmdHandler struct {
	RW  ReadWriterAt
	Inq *InquiryInfo
}

// InquiryInfo holds the general vendor information for the emulated SCSI Device. Fields used from this will be padded or trunacted to meet the spec.
type InquiryInfo struct {
	VendorID   string
	ProductID  string
	ProductRev string
}

var defaultInquiry = InquiryInfo{
	VendorID:   "go-tcmu",
	ProductID:  "TCMU Device",
	ProductRev: "0001",
}

func (h ReadWriterAtCmdHandler) HandleCommand(cmd *SCSICmd) (SCSIResponse, error) {
	switch cmd.Command() {
	case scsi.Inquiry:
		if h.Inq == nil {
			h.Inq = &defaultInquiry
		}
		return EmulateInquiry(cmd, h.Inq)
	case scsi.TestUnitReady:
		return EmulateTestUnitReady(cmd)
	case scsi.ServiceActionIn16:
		return EmulateServiceActionIn(cmd)
	case scsi.ModeSense, scsi.ModeSense10:
		return EmulateModeSense(cmd, false)
	case scsi.ModeSelect, scsi.ModeSelect10:
		return EmulateModeSelect(cmd, false)
	case scsi.Read6, scsi.Read10, scsi.Read12, scsi.Read16:
		return EmulateRead(cmd, h.RW)
	case scsi.Write6, scsi.Write10, scsi.Write12, scsi.Write16:
		return EmulateWrite(cmd, h.RW)
	default:
		log.Debugf("Ignore unknown SCSI command 0x%x\n", cmd.Command())
	}
	return cmd.NotHandled(), nil
}

func EmulateInquiry(cmd *SCSICmd, inq *InquiryInfo) (SCSIResponse, error) {
	if (cmd.GetCDB(1) & 0x01) == 0 {
		if cmd.GetCDB(2) == 0x00 {
			return EmulateStdInquiry(cmd, inq)
		}
		return cmd.IllegalRequest(), nil
	}
	return EmulateEvpdInquiry(cmd, inq)
}

func FixedString(s string, length int) []byte {
	p := []byte(s)
	l := len(p)
	if l >= length {
		return p[:length]
	}
	sp := bytes.Repeat([]byte{' '}, length-l)
	return append(p, sp...)
}

func EmulateStdInquiry(cmd *SCSICmd, inq *InquiryInfo) (SCSIResponse, error) {
	buf := make([]byte, 36)
	buf[2] = 0x05 // SPC-3
	buf[3] = 0x02 // response data format
	buf[7] = 0x02 // CmdQue
	vendorID := FixedString(inq.VendorID, 8)
	copy(buf[8:16], vendorID)
	productID := FixedString(inq.ProductID, 16)
	copy(buf[16:32], productID)
	productRev := FixedString(inq.ProductRev, 4)
	copy(buf[32:36], productRev)

	buf[4] = 31 // Set additional length to 31
	_, err := cmd.Write(buf)
	if err != nil {
		return SCSIResponse{}, err
	}
	return cmd.Ok(), nil
}

func EmulateEvpdInquiry(cmd *SCSICmd, inq *InquiryInfo) (SCSIResponse, error) {
	vpdType := cmd.GetCDB(2)
	log.Debugf("SCSI EVPD Inquiry 0x%x\n", vpdType)
	switch vpdType {
	case 0x0: // Supported VPD pages
		// The absolute minimum.
		data := make([]byte, 6)

		// We support 0x00 and 0x83 only
		data[3] = 2
		data[4] = 0x00
		data[5] = 0x83

		cmd.Write(data)
		return cmd.Ok(), nil
	case 0x83: // Device identification
		used := 4
		data := make([]byte, 512)
		data[1] = 0x83
		wwn := []byte("") // TODO(barakmich): Report WWN. See tcmu_get_wwn;

		// 1/3: T10 Vendor id
		ptr := data[used:]
		ptr[0] = 2 // code set: ASCII
		ptr[1] = 1 // identifier: T10 vendor id
		copy(ptr[4:], FixedString(inq.VendorID, 8))
		n := copy(ptr[12:], wwn)
		ptr[3] = byte(8 + n + 1)
		used += int(ptr[3]) + 4

		// 2/3: NAA binary // TODO(barakmich): Emulate given a real WWN

		ptr = data[used:]
		ptr[0] = 1  // code set: binary
		ptr[1] = 3  // identifier: NAA
		ptr[3] = 16 // body length for naa registered extended format

		// Set type 6 and use OpenFabrics IEEE Company ID: 00 14 05
		ptr[4] = 0x60
		ptr[5] = 0x01
		ptr[6] = 0x40
		ptr[7] = 0x50
		next := true
		i := 7
		for _, x := range wwn {
			if i >= 20 {
				break
			}
			v, ok := charToHex(x)
			if !ok {
				continue
			}

			if next {
				next = false
				ptr[i] |= v
				i++
			} else {
				next = true
				ptr[i] = (v << 4)
			}
		}
		used += 20

		// 3/3: Vendor specific
		ptr = data[used:]
		ptr[0] = 2 // code set: ASCII
		ptr[1] = 0 // identifier: vendor-specific

		cfgString := cmd.Device().GetDevConfig()
		n = copy(ptr[4:], []byte(cfgString))
		ptr[3] = byte(n + 1)

		used += n + 1 + 4

		order := binary.BigEndian
		order.PutUint16(data[2:4], uint16(used-4))

		cmd.Write(data[:used])
		return cmd.Ok(), nil
	default:
		return cmd.IllegalRequest(), nil
	}
}

func EmulateTestUnitReady(cmd *SCSICmd) (SCSIResponse, error) {
	return cmd.Ok(), nil
}

func EmulateServiceActionIn(cmd *SCSICmd) (SCSIResponse, error) {
	if cmd.GetCDB(1) == scsi.ReadCapacity16 {
		return EmulateReadCapacity16(cmd)
	}
	return cmd.NotHandled(), nil
}

func EmulateReadCapacity16(cmd *SCSICmd) (SCSIResponse, error) {
	buf := make([]byte, 32)
	order := binary.BigEndian
	// This is in LBAs, and the "index of the last LBA", so minus 1. Friggin spec.
	order.PutUint64(buf[0:8], uint64(cmd.Device().Sizes().VolumeSize/cmd.Device().Sizes().BlockSize)-1)
	// This is in BlockSize
	order.PutUint32(buf[8:12], uint32(cmd.Device().Sizes().BlockSize))
	// All the rest is 0
	cmd.Write(buf)
	return cmd.Ok(), nil
}

func charToHex(c byte) (byte, bool) {
	if c >= '0' && c <= '9' {
		return c - '0', true
	}
	if c >= 'a' && c <= 'f' {
		return c - 'a' + 10, true
	}
	if c >= 'A' && c <= 'F' {
		return c - 'A' + 10, true
	}
	return 0x00, false
}

func CachingModePage(w io.Writer, wce bool) {
	buf := make([]byte, 20)
	buf[0] = 0x08 // caching mode page
	buf[1] = 0x12 // page length (20, forced)
	if wce {
		buf[2] = buf[2] | 0x04
	}
	w.Write(buf)
}

// EmulateModeSense responds to a static Mode Sense command. `wce` enables or diables
// the SCSI "Write Cache Enabled" flag.
func EmulateModeSense(cmd *SCSICmd, wce bool) (SCSIResponse, error) {
	pgs := &bytes.Buffer{}
	outlen := int(cmd.XferLen())

	page := cmd.GetCDB(2)
	if page == 0x3f || page == 0x08 {
		CachingModePage(pgs, wce)
	}
	scsiCmd := cmd.Command()

	dsp := byte(0x10) // Support DPO/FUA

	pgdata := pgs.Bytes()
	var hdr []byte
	if scsiCmd == scsi.ModeSense {
		// MODE_SENSE_6
		hdr = make([]byte, 4)
		hdr[0] = byte(len(pgdata) + 3)
		hdr[1] = 0x00 // Device type
		hdr[2] = dsp
	} else {
		// MODE_SENSE_10
		hdr = make([]byte, 8)
		order := binary.BigEndian
		order.PutUint16(hdr, uint16(len(pgdata)+6))
		hdr[2] = 0x00 // Device type
		hdr[3] = dsp
	}
	data := append(hdr, pgdata...)
	if outlen < len(data) {
		data = data[:outlen]
	}
	cmd.Write(data)
	return cmd.Ok(), nil
}

// EmulateModeSelect checks that the only mode selected is the static one returned from
// EmulateModeSense. `wce` should match the Write Cache Enabled of the EmulateModeSense call.
func EmulateModeSelect(cmd *SCSICmd, wce bool) (SCSIResponse, error) {
	selectTen := (cmd.GetCDB(0) == scsi.ModeSelect10)
	page := cmd.GetCDB(2) & 0x3f
	subpage := cmd.GetCDB(3)
	allocLen := cmd.XferLen()
	hdrLen := 4
	if selectTen {
		hdrLen = 8
	}
	inBuf := make([]byte, 512)
	gotSense := false

	if allocLen == 0 {
		return cmd.Ok(), nil
	}
	n, err := cmd.Read(inBuf)
	if err != nil {
		return SCSIResponse{}, err
	}
	if n >= len(inBuf) {
		return cmd.CheckCondition(scsi.SenseIllegalRequest, scsi.AscParameterListLengthError), nil
	}

	cdbone := cmd.GetCDB(1)
	if cdbone&0x10 == 0 || cdbone&0x01 != 0 {
		return cmd.IllegalRequest(), nil
	}

	pgs := &bytes.Buffer{}
	// TODO(barakmich): select over handlers. Today we have one.
	if page == 0x08 && subpage == 0 {
		CachingModePage(pgs, wce)
		gotSense = true
	}
	if !gotSense {
		return cmd.IllegalRequest(), nil
	}
	b := pgs.Bytes()
	if int(allocLen) < (hdrLen + len(b)) {
		return cmd.CheckCondition(scsi.SenseIllegalRequest, scsi.AscParameterListLengthError), nil
	}
	/* Verify what was selected is identical to what sense returns, since we
	don't support actually setting anything. */
	if !bytes.Equal(inBuf[hdrLen:len(b)], b) {
		log.Errorf("not equal for some reason: %#v %#v", inBuf[hdrLen:len(b)], b)
		return cmd.CheckCondition(scsi.SenseIllegalRequest, scsi.AscInvalidFieldInParameterList), nil
	}
	return cmd.Ok(), nil
}

func EmulateRead(cmd *SCSICmd, r io.ReaderAt) (SCSIResponse, error) {
	offset := cmd.LBA() * uint64(cmd.Device().Sizes().BlockSize)
	length := int(cmd.XferLen() * uint32(cmd.Device().Sizes().BlockSize))
	if cmd.Buf == nil {
		cmd.Buf = make([]byte, length)
	}
	if len(cmd.Buf) < int(length) {
		//realloc
		cmd.Buf = make([]byte, length)
	}
	n, err := r.ReadAt(cmd.Buf[:length], int64(offset))
	if err != nil {
		log.Errorln("read/read failed: error:", err)
		return cmd.MediumError(), nil
	}
	if n < length {
		log.Errorln("read/read failed: unable to copy enough")
		return cmd.MediumError(), nil
	}
	n, err = cmd.Write(cmd.Buf[:length])
	if err != nil {
		log.Errorln("read/write failed: error:", err)
		return cmd.MediumError(), nil
	}
	if n < length {
		log.Errorln("read/write failed: unable to copy enough")
		return cmd.MediumError(), nil
	}
	return cmd.Ok(), nil
}

func EmulateWrite(cmd *SCSICmd, r io.WriterAt) (SCSIResponse, error) {
	offset := cmd.LBA() * uint64(cmd.Device().Sizes().BlockSize)
	length := int(cmd.XferLen() * uint32(cmd.Device().Sizes().BlockSize))
	if cmd.Buf == nil {
		cmd.Buf = make([]byte, length)
	}
	if len(cmd.Buf) < int(length) {
		//realloc
		cmd.Buf = make([]byte, length)
	}
	n, err := cmd.Read(cmd.Buf[:int(length)])
	if err != nil {
		log.Errorln("write/read failed: error:", err)
		return cmd.MediumError(), nil
	}
	if n < length {
		log.Errorln("write/read failed: unable to copy enough")
		return cmd.MediumError(), nil
	}
	n, err = r.WriteAt(cmd.Buf[:length], int64(offset))
	if err != nil {
		log.Errorln("write/write failed: error:", err)
		return cmd.MediumError(), nil
	}
	if n < length {
		log.Errorln("write/write failed: unable to copy enough")
		return cmd.MediumError(), nil
	}
	return cmd.Ok(), nil
}
