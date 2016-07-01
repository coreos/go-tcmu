package tcmu

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

var byteOrder binary.ByteOrder = binary.LittleEndian

func (d *Device) mbVersion() uint16 {
	return *(*uint16)(unsafe.Pointer(&d.mmap[0]))
}

func (d *Device) mbFlags() uint16 {
	return *(*uint16)(unsafe.Pointer(&d.mmap[2]))
}

func (d *Device) mbCmdrOffset() uint32 {
	return *(*uint32)(unsafe.Pointer(&d.mmap[4]))
}

func (d *Device) mbCmdrSize() uint32 {
	return *(*uint32)(unsafe.Pointer(&d.mmap[8]))
}

func (d *Device) mbCmdHead() uint32 {
	return *(*uint32)(unsafe.Pointer(&d.mmap[12]))
}

func (d *Device) mbCmdTail() uint32 {
	return *(*uint32)(unsafe.Pointer(&d.mmap[64]))
}

func (d *Device) mbSetTail(u uint32) {
	byteOrder.PutUint32(d.mmap[64:], u)
}

/*
enum tcmu_opcode {
  TCMU_OP_PAD = 0,
  TCMU_OP_CMD,
};
*/
type tcmuOpcode int

const (
	tcmuOpPad tcmuOpcode = 0
	tcmuOpCmd            = 1
)

/*

// Only a few opcodes, and length is 8-byte aligned, so use low bits for opcode.
struct tcmu_cmd_entry_hdr {
  __u32 len_op;
  __u16 cmd_id;
  __u8 kflags;
#define TCMU_UFLAG_UNKNOWN_OP 0x1
  __u8 uflags;

} __packed;
*/
func (d *Device) entHdrOp(off int) tcmuOpcode {
	i := int(*(*uint32)(unsafe.Pointer(&d.mmap[off+offLenOp])))
	i = i & 0x7
	return tcmuOpcode(i)
}

func (d *Device) entHdrGetLen(off int) int {
	i := *(*uint32)(unsafe.Pointer(&d.mmap[off+offLenOp]))
	i = i &^ 0x7
	return int(i)
}

func (d *Device) entCmdId(off int) uint16 {
	return *(*uint16)(unsafe.Pointer(&d.mmap[off+offCmdId]))
}
func (d *Device) setEntCmdId(off int, id uint16) {
	*(*uint16)(unsafe.Pointer(&d.mmap[off+offCmdId])) = id
}
func (d *Device) entKflags(off int) uint8 {
	return *(*uint8)(unsafe.Pointer(&d.mmap[off+offKFlags]))
}
func (d *Device) entUflags(off int) uint8 {
	return *(*uint8)(unsafe.Pointer(&d.mmap[off+offUFlags]))
}

func (d *Device) setEntUflagUnknownOp(off int) {
	d.mmap[off+offUFlags] = 0x01
}

/*
#define TCMU_SENSE_BUFFERSIZE 96

struct tcmu_cmd_entry {
	  struct tcmu_cmd_entry_hdr hdr;

		union {
			struct {
				uint32_t iov_cnt; 0
				uint32_t iov_bidi_cnt; 4
				uint32_t iov_dif_cnt; 8
				uint64_t cdb_off; 12
				uint64_t __pad1; 20
				uint64_t __pad2; 28
				struct iovec iov[0];

			} req;
			struct {
				uint8_t scsi_status;
				uint8_t __pad1;
				uint16_t __pad2;
				uint32_t __pad3;
				char sense_buffer[TCMU_SENSE_BUFFERSIZE];

			} rsp;
		};
} __packed;
*/

func (d *Device) entReqIovCnt(off int) uint32 {
	return *(*uint32)(unsafe.Pointer(&d.mmap[off+offReqIovCnt]))
}

func (d *Device) entReqIovBidiCnt(off int) uint32 {
	return *(*uint32)(unsafe.Pointer(&d.mmap[off+offReqIovBidiCnt]))
}

func (d *Device) entReqIovDifCnt(off int) uint32 {
	return *(*uint32)(unsafe.Pointer(&d.mmap[off+offReqIovDifCnt]))
}

func (d *Device) entReqCdbOff(off int) uint64 {
	return *(*uint64)(unsafe.Pointer(&d.mmap[off+offReqCdbOff]))
}

func (d *Device) setEntRespSCSIStatus(off int, status byte) {
	d.mmap[off+offRespSCSIStatus] = status
}

func (d *Device) copyEntRespSenseData(off int, data []byte) {
	buf := d.mmap[off+offRespSense : off+offRespSense+tcmuSenseBufferSize]
	copy(buf, data)
	if len(data) < tcmuSenseBufferSize {
		for i := len(data); i < tcmuSenseBufferSize; i++ {
			buf[i] = 0
		}
	}
}

func (d *Device) entIovecN(off int, idx int) []byte {
	out := syscall.Iovec{}
	p := unsafe.Pointer(&d.mmap[off+offReqIov0Base])
	out = *(*syscall.Iovec)(unsafe.Pointer(uintptr(p) + uintptr(idx)*unsafe.Sizeof(out)))
	moff := *(*int)(unsafe.Pointer(&out.Base))
	return d.mmap[moff : moff+int(out.Len)]
}

func (d *Device) entCdb(off int) []byte {
	cdbStart := int(d.entReqCdbOff(off))
	len := d.cdbLen(cdbStart)
	return d.mmap[cdbStart : cdbStart+len]
}

func (d *Device) cdbLen(cdbStart int) int {
	opcode := d.mmap[cdbStart]
	// See spc-4 4.2.5.1 operation code
	//
	if opcode <= 0x1f {
		return 6
	} else if opcode <= 0x5f {
		return 10
	} else if opcode == 0x7f {
		return int(d.mmap[cdbStart+7]) + 8
	} else if opcode >= 0x80 && opcode <= 0x9f {
		return 16
	} else if opcode >= 0xa0 && opcode <= 0xbf {
		return 12
	} else {
		panic(fmt.Sprintf("what opcode is %x", opcode))
	}
}
