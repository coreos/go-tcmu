package scsi

/*
 * Additional values not defined by other headers, they
 * seem a little incomplete.
 *
 * Find codes in the various SCSI specs.
 * Btw sense codes are at www.t10.org/lists/asc-num.txt
 *
 */

/*
 * SCSI Opcodes
 */
const (
	TestUnitReady              = 0x00
	RezeroUnit                 = 0x01
	RequestSense               = 0x03
	FormatUnit                 = 0x04
	ReadBlockLimits            = 0x05
	ReassignBlocks             = 0x07
	InitializeElementStatus    = 0x07
	Read6                      = 0x08
	Write6                     = 0x0a
	Seek6                      = 0x0b
	ReadReverse                = 0x0f
	WriteFilemarks             = 0x10
	Space                      = 0x11
	Inquiry                    = 0x12
	RecoverBufferedData        = 0x14
	ModeSelect                 = 0x15
	Reserve                    = 0x16
	Release                    = 0x17
	Copy                       = 0x18
	Erase                      = 0x19
	ModeSense                  = 0x1a
	StartStop                  = 0x1b
	ReceiveDiagnostic          = 0x1c
	SendDiagnostic             = 0x1d
	AllowMediumRemoval         = 0x1e
	ReadFormatCapacities       = 0x23
	SetWindow                  = 0x24
	ReadCapacity               = 0x25
	Read10                     = 0x28
	Write10                    = 0x2a
	Seek10                     = 0x2b
	PositionToElement          = 0x2b
	WriteVerify                = 0x2e
	Verify                     = 0x2f
	SearchHigh                 = 0x30
	SearchEqual                = 0x31
	SearchLow                  = 0x32
	SetLimits                  = 0x33
	PreFetch                   = 0x34
	ReadPosition               = 0x34
	SynchronizeCache           = 0x35
	LockUnlockCache            = 0x36
	ReadDefectData             = 0x37
	MediumScan                 = 0x38
	Compare                    = 0x39
	CopyVerify                 = 0x3a
	WriteBuffer                = 0x3b
	ReadBuffer                 = 0x3c
	UpdateBlock                = 0x3d
	ReadLong                   = 0x3e
	WriteLong                  = 0x3f
	ChangeDefinition           = 0x40
	WriteSame                  = 0x41
	Unmap                      = 0x42
	ReadToc                    = 0x43
	ReadHeader                 = 0x44
	GetEventStatusNotification = 0x4a
	LogSelect                  = 0x4c
	LogSense                   = 0x4d
	Xdwriteread10              = 0x53
	ModeSelect10               = 0x55
	Reserve10                  = 0x56
	Release10                  = 0x57
	ModeSense10                = 0x5a
	PersistentReserveIn        = 0x5e
	PersistentReserveOut       = 0x5f
	VariableLengthCmd          = 0x7f
	ReportLuns                 = 0xa0
	SecurityProtocolIn         = 0xa2
	MaintenanceIn              = 0xa3
	MaintenanceOut             = 0xa4
	MoveMedium                 = 0xa5
	ExchangeMedium             = 0xa6
	Read12                     = 0xa8
	ServiceActionOut12         = 0xa9
	Write12                    = 0xaa
	ReadMediaSerialNumber      = 0xab /* Obsolete with Spc-2 */
	ServiceActionIn12          = 0xab
	WriteVerify12              = 0xae
	Verify12                   = 0xaf
	SearchHigh12               = 0xb0
	SearchEqual12              = 0xb1
	SearchLow12                = 0xb2
	SecurityProtocolOut        = 0xb5
	ReadElementStatus          = 0xb8
	SendVolumeTag              = 0xb6
	WriteLong2                 = 0xea
	ExtendedCopy               = 0x83
	ReceiveCopyResults         = 0x84
	AccessControlIn            = 0x86
	AccessControlOut           = 0x87
	Read16                     = 0x88
	CompareAndWrite            = 0x89
	Write16                    = 0x8a
	ReadAttribute              = 0x8c
	WriteAttribute             = 0x8d
	WriteVerify16              = 0x8e
	Verify16                   = 0x8f
	SynchronizeCache16         = 0x91
	WriteSame16                = 0x93
	ServiceActionBidirectional = 0x9d
	ServiceActionIn16          = 0x9e
	ServiceActionOut16         = 0x9f
	/* values for service action in */
	SaiReadCapacity16  = 0x10
	SaiGetLbaStatus    = 0x12
	SaiReportReferrals = 0x13
	/* values for VariableLengthCmd service action codes
	 * see spc4r17 Section D.3.5, table D.7 and D.8 */
	VlcSaReceiveCredential = 0x1800
	/* values for maintenance in */
	MiReportIdentifyingInformation           = 0x05
	MiReportTargetPgs                        = 0x0a
	MiReportAliases                          = 0x0b
	MiReportSupportedOperationCodes          = 0x0c
	MiReportSupportedTaskManagementFunctions = 0x0d
	MiReportPriority                         = 0x0e
	MiReportTimestamp                        = 0x0f
	MiManagementProtocolIn                   = 0x10
	/* value for MiReportTargetPgs ext header */
	MiExtHdrParamFmt = 0x20
	/* values for maintenance out */
	MoSetIdentifyingInformation = 0x06
	MoSetTargetPgs              = 0x0a
	MoChangeAliases             = 0x0b
	MoSetPriority               = 0x0e
	MoSetTimestamp              = 0x0f
	MoManagementProtocolOut     = 0x10
	/* values for variable length command */
	Xdread32      = 0x03
	Xdwrite32     = 0x04
	Xpwrite32     = 0x06
	Xdwriteread32 = 0x07
	Read32        = 0x09
	Verify32      = 0x0a
	Write32       = 0x0b
	WriteSame32   = 0x0d
	/*
	 * Service action opcodes
	 */
	ReadCapacity16 = 0x10
)

/*
 *  SCSI Architecture Model (Sam) Status codes. Taken from Sam-3 draft
 *  T10/1561-D Revision 4 Draft dated 7th November 2002.
 */
const (
	SamStatGood                     = 0x00
	SamStatCheckCondition           = 0x02
	SamStatConditionMet             = 0x04
	SamStatBusy                     = 0x08
	SamStatIntermediate             = 0x10
	SamStatIntermediateConditionMet = 0x14
	SamStatReservationConflict      = 0x18
	SamStatCommandTerminated        = 0x22 /* obsolete in Sam-3 */
	SamStatTaskSetFull              = 0x28
	SamStatAcaActive                = 0x30
	SamStatTaskAborted              = 0x40
)

/*
 * Sense codes
 */
const (
	AscReadError                       = 0x1100
	AscParameterListLengthError        = 0x1a00
	AscInternalTargetFailure           = 0x4400
	AscMiscompareDuringVerifyOperation = 0x1d00
	AscInvalidFieldInCdb               = 0x2400
	AscInvalidFieldInParameterList     = 0x2600
)

/*
 * Sense Keys
 */
const (
	SenseNoSense        = 0x00
	SenseRecoveredError = 0x01
	SenseNotReady       = 0x02
	SenseMediumError    = 0x03
	SenseHardwareError  = 0x04
	SenseIllegalRequest = 0x05
	SenseUnitAttention  = 0x06
	SenseDataProtect    = 0x07
	SenseBlankCheck     = 0x08
	SenseCopyAborted    = 0x0a
	SenseAbortedCommand = 0x0b
	SenseVolumeOverflow = 0x0d
	SenseMiscompare     = 0x0e
)
