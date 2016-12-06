package tcmu

import (
	"fmt"

	"github.com/coreos/go-tcmu/scsi"
	"github.com/prometheus/common/log"
	"golang.org/x/sys/unix"
)

const (
	tcmuSenseBufferSize = 96
)

func (d *Device) beginPoll() {
	// Entry point for the goroutine.
	go d.recvResponse()
	buf := make([]byte, 4)
	for {
		var n int
		var err error
		n, err = unix.Read(d.uioFd, buf)
		if n == -1 && err != nil {
			log.Errorf("error poll reading: %s", err)
			break
		}
		for {
			cmd, err := d.getNextCommand()
			if err != nil {
				log.Errorf("error getting next command: %s", err)
				break
			}
			if cmd == nil {
				break
			}
			d.cmdChan <- cmd
		}
	}
	close(d.cmdChan)
}

func (d *Device) recvResponse() {
	var n int
	buf := make([]byte, 4)
	for resp := range d.respChan {
		err := d.completeCommand(resp)
		if err != nil {
			log.Errorf("error completing command: %s", err)
			return
		}
		/* Tell the fd there's something new */
		n, err = unix.Write(d.uioFd, buf)
		if n == -1 && err != nil {
			log.Errorf("error poll writing: %s", err)
			return
		}
	}
}

func (d *Device) completeCommand(resp SCSIResponse) error {
	off := d.tailEntryOff()
	for d.entHdrOp(off) != tcmuOpCmd {
		d.mbSetTail((d.mbCmdTail() + uint32(d.entHdrGetLen(off))) % d.mbCmdrSize())
		off = d.tailEntryOff()
	}
	if d.entCmdId(off) != resp.id {
		d.setEntCmdId(off, resp.id)
	}
	d.setEntRespSCSIStatus(off, resp.status)
	if resp.status != scsi.SamStatGood {
		d.copyEntRespSenseData(off, resp.senseBuffer)
	}
	d.mbSetTail((d.mbCmdTail() + uint32(d.entHdrGetLen(off))) % d.mbCmdrSize())
	return nil
}

func (d *Device) getNextCommand() (*SCSICmd, error) {
	//d.debugPrintMb()
	//fmt.Printf("nextEntryOff: %d\n", d.nextEntryOff())
	//fmt.Printf("headEntryOff: %d\n", d.headEntryOff())
	for d.nextEntryOff() != d.headEntryOff() {
		off := d.nextEntryOff()
		if d.entHdrOp(off) == tcmuOpPad {
			d.cmdTail = (d.cmdTail + uint32(d.entHdrGetLen(off))) % d.mbCmdrSize()
		} else if d.entHdrOp(off) == tcmuOpCmd {
			//d.printEnt(off)
			out := &SCSICmd{
				id:     d.entCmdId(off),
				device: d,
			}
			out.cdb = d.entCdb(off)
			vecs := int(d.entReqIovCnt(off))
			out.vecs = make([][]byte, vecs)
			for i := 0; i < vecs; i++ {
				v := d.entIovecN(off, i)
				out.vecs[i] = v
			}
			d.cmdTail = (d.cmdTail + uint32(d.entHdrGetLen(off))) % d.mbCmdrSize()
			return out, nil
		} else {
			panic(fmt.Sprintf("unsupported command from tcmu? %d", d.entHdrOp(off)))
		}
	}
	return nil, nil
}

func (d *Device) printEnt(off int) {
	for i, x := range d.mmap[off : off+d.entHdrGetLen(off)] {
		fmt.Printf("0x%02x ", x)
		if i%16 == 15 {
			fmt.Printf("\n")
		}
	}
}

func (d *Device) nextEntryOff() int {
	return int(d.cmdTail + d.mbCmdrOffset())
}

func (d *Device) headEntryOff() int {
	return int(d.mbCmdHead() + d.mbCmdrOffset())
}

func (d *Device) tailEntryOff() int {
	return int(d.mbCmdTail() + d.mbCmdrOffset())
}
