# go-tcmu
[![GoDoc](https://godoc.org/github.com/coreos/go-tcmu?status.svg)](https://godoc.org/github.com/coreos/go-tcmu)
---

Go bindings to attach Go `Reader`s and `Writer`s to the Linux kernel via SCSI.

It connects to the [TCM Userspace](https://www.kernel.org/doc/Documentation/target/tcmu-design.txt) kernel API, and provides a loopback device that responds to SCSI commands. This project is based on [open-iscsi/tcmu-runner](https://github.com/open-iscsi/tcmu-runner), but in pure Go.

### Overview

This package creates two types of Handlers (much like `net/http`) for [SCSI block device](https://en.wikipedia.org/wiki/SCSI_command) commands. It wraps the implementation details of the kernel API, and sets up (a) a TCMU SCSI device and connect that to (b) a loopback SCSI target. 

From here, the [Linux IO Target](http://linux-iscsi.org/wiki/Main_Page) kernel stack can expose the SCSI target however it likes. This includes iSCSI, vHost, etc. For further details, see the [LIO wiki](http://linux-iscsi.org/wiki/Main_Page).

### Usage
First, to use this package, you'll need the appropriate kernel modules and configfs mounted

#### Make sure configfs is mounted 

This may already be true on your system, depending on kernel configuration. Many distributions do this by default. Check if it's mounted to `/sys/kernel/config` with

```
mount | grep configfs
```

Which should respond
```
configfs on /sys/kernel/config type configfs (rw,relatime)
```

To mount it explicitly:
```
sudo modprobe configfs
sudo mkdir -p /sys/kernel/config
sudo mount -t configfs none /sys/kernel/config
```

#### Use the TCMU module

Many distros include the module, but few activate it by default.

```
sudo modprobe target_core_user
```


Now that that's settled, there's [tcmufile.go](cmd/tcmufile/tcmufile.go) for a quick example binary that serves an image file under /dev/tcmufile/myfile. 

For creating your custom SCSI targets based on a ReadWriterAt:

```go
handler := &tcmu.SCSIHandler{
        HBA: 30, // Choose a virtual HBA number. 30 is fine.
        LUN: 0,  // The LUN attached to this HBA. Multiple LUNs can work on the same HBA, this differentiates them.
        WWN: tcmu.NaaWWN{
                OUI:      "000000",                      // Or provide your OUI
                VendorID: tcmu.GenerateSerial("foobar"), // Or provide a vendor id/serial number
                // Optional: Provide further information for your WWN
                // VendorIDExt: "0123456789abcdef", 
        },
        VolumeName: "myVolName", // The name of your volume.
        DataSizes: tcmu.DataSizes{
                VolumeSize: 5 * 1024 * 1024, // Size in bytes, eg, 5GiB
                BlockSize:  1024,            // Size of logical blocks, eg, 1K
        },
        DevReady: tcmu.SingleThreadedDevReady(
                tcmu.ReadWriterAtCmdHandler{      // Or replace with your own handler
                        RW: rw,
                }),
}
d, _ := tcmu.OpenTCMUDevice("/dev/myDevDirectory", handler)
defer d.Close()
```
This will create a device named `/dev/myDevDirectory/myVolName` with the mentioned details. It is now ready for formatting and treating like a block device.

If you wish to handle more SCSI commands, you can implement a replacement for the `ReadWriterAtCmdHandler` following the interface:

```go
type SCSICmdHandler interface {
	HandleCommand(cmd *SCSICmd) (SCSIResponse, error)
}
```

If the default functionality was acceptable, the library contains a number of helpful `Emulate` functions that you can call to achieve the basic functionality.
