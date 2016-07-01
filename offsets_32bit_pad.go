// +build arm

package tcmu

// This file works for the build tags above. To port to other architectures,
// check the offsets from C using the included test.c.
// Go should handle the endianness .

const (
	offLenOp         = 0
	offCmdId         = 4
	offKFlags        = 6
	offUFlags        = 7
	entReqRespOff    = 8
	offReqIovCnt     = entReqRespOff + 0
	offReqIovBidiCnt = entReqRespOff + 4
	offReqIovDifCnt  = entReqRespOff + 8
	offReqCdbOff     = entReqRespOff + 16

	iovSize        = 8
	offReqIov0Base = entReqRespOff + 40
	offReqIov0Len  = entReqRespOff + 44

	offRespSCSIStatus = entReqRespOff + 0
	offRespSense      = entReqRespOff + 8
)
