// tcmu is a package that connects to the TCM in Userspace kernel module, a part of the LIO stack. It provides the
// ability to emulate a SCSI storage device in pure Go.
package tcmu

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/prometheus/common/log"
	"github.com/sirupsen/logrus"
)

const (
	configDirFmt = "/sys/kernel/config/target/core/user_%d"
	scsiDir      = "/sys/kernel/config/target/loopback"
)

type Device struct {
	scsi    *SCSIHandler
	devPath string

	hbaDir     string
	deviceName string

	uioFd    int
	mapsize  uint64
	mmap     []byte
	cmdChan  chan *SCSICmd
	respChan chan SCSIResponse
	cmdTail  uint32

	toClean map[string]bool
}

// WWN provides two WWNs, one for the device itself and one for the loopback
// device created for the kernel.
type WWN interface {
	DeviceID() string
	NexusID() string
}

func (d *Device) GetDevConfig() string {
	return fmt.Sprintf("go-tcmu//%s", d.scsi.VolumeName)
}

func (d *Device) Sizes() DataSizes {
	return d.scsi.DataSizes
}

// OpenTCMUDevice creates the virtual device based on the details in the SCSIHandler, eventually creating a device under devPath (eg, "/dev") with the file name scsi.VolumeName.
// The returned Device represents the open device connection to the kernel, and must be closed.
func OpenTCMUDevice(devPath string, scsi *SCSIHandler) (*Device, error) {
	d := &Device{
		scsi:    scsi,
		devPath: devPath,
		uioFd:   -1,
		hbaDir:  fmt.Sprintf(configDirFmt, scsi.HBA),
		toClean: make(map[string]bool),
	}
	if err := d.preEnableTcmu(); err != nil {
		return d, err
	}
	if err := d.start(); err != nil {
		return d, err
	}

	return d, d.postEnableTcmu()
}

func (d *Device) Close() error {
	err := d.teardown()
	if err != nil {
		return err
	}
	if d.uioFd != -1 {
		unix.Close(d.uioFd)
	}
	return nil
}

func (d *Device) preEnableTcmu() error {
	err := d.writeLines(path.Join(d.hbaDir, d.scsi.VolumeName, "control"), []string{
		fmt.Sprintf("dev_size=%d", d.scsi.DataSizes.VolumeSize),
		fmt.Sprintf("dev_config=%s", d.GetDevConfig()),
		fmt.Sprintf("hw_block_size=%d", d.scsi.DataSizes.BlockSize),
		"async=1",
	})
	if err != nil {
		return err
	}

	return d.writeLines(path.Join(d.hbaDir, d.scsi.VolumeName, "enable"), []string{
		"1",
	})
}

func (d *Device) getSCSIPrefixAndWnn() (string, string) {
	return path.Join(scsiDir, d.scsi.WWN.DeviceID(), "tpgt_1"), d.scsi.WWN.NexusID()
}

func (d *Device) getLunPath(prefix string) string {
	return path.Join(prefix, "lun", fmt.Sprintf("lun_%d", d.scsi.LUN))
}

func (d *Device) postEnableTcmu() error {
	prefix, nexusWnn := d.getSCSIPrefixAndWnn()

	err := d.writeLines(path.Join(prefix, "nexus"), []string{
		nexusWnn,
	})
	if err != nil {
		return err
	}

	lunPath := d.getLunPath(prefix)
	logrus.Debugf("Creating directory: %s", lunPath)
	if err := os.MkdirAll(lunPath, 0755); err != nil && !os.IsExist(err) {
		return err
	} else if err == nil {
		d.toClean[lunPath] = true
		d.toClean[path.Join(lunPath, d.scsi.VolumeName)] = true
	}

	logrus.Debugf("Linking: %s => %s", path.Join(lunPath, d.scsi.VolumeName), path.Join(d.hbaDir, d.scsi.VolumeName))
	if err := os.Symlink(path.Join(d.hbaDir, d.scsi.VolumeName), path.Join(lunPath, d.scsi.VolumeName)); err != nil {
		return err
	}
	d.toClean[path.Join(d.hbaDir, d.scsi.VolumeName)] = true

	return d.createDevEntry()
}

func (d *Device) createDevEntry() error {
	if err := os.MkdirAll(d.devPath, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	dev := filepath.Join(d.devPath, d.scsi.VolumeName)

	if _, err := os.Stat(dev); err == nil {
		return fmt.Errorf("Device %s already exists, can not create", dev)
	}
	d.toClean[dev] = true

	tgt, _ := d.getSCSIPrefixAndWnn()

	address, err := ioutil.ReadFile(path.Join(tgt, "address"))
	if err != nil {
		return err
	}

	found := false
	matches := []string{}
	path := fmt.Sprintf("/sys/bus/scsi/devices/%s*/block/*/dev", strings.TrimSpace(string(address)))
	for i := 0; i < 30; i++ {
		var err error
		matches, err = filepath.Glob(path)
		if len(matches) > 0 && err == nil {
			found = true
			break
		}

		logrus.Debugf("Waiting for %s", path)
		time.Sleep(1 * time.Second)
	}

	if !found {
		return fmt.Errorf("Failed to find %s", path)
	}

	if len(matches) == 0 {
		return fmt.Errorf("Failed to find %s", path)
	}

	if len(matches) > 1 {
		return fmt.Errorf("Too many matches for %s, found %d", path, len(matches))
	}

	majorMinor, err := ioutil.ReadFile(matches[0])
	if err != nil {
		return err
	}

	parts := strings.Split(strings.TrimSpace(string(majorMinor)), ":")
	if len(parts) != 2 {
		return fmt.Errorf("Invalid major:minor string %s", string(majorMinor))
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}

	logrus.Debugf("Creating device %s %d:%d", dev, major, minor)
	return mknod(dev, major, minor)
}

func mknod(device string, major, minor int) error {
	var fileMode os.FileMode = 0600
	fileMode |= syscall.S_IFBLK
	dev := int((major << 8) | (minor & 0xff) | ((minor & 0xfff00) << 12))

	return syscall.Mknod(device, uint32(fileMode), dev)
}

func (d *Device) writeLines(target string, lines []string) error {
	dir := path.Dir(target)
	if stat, err := os.Stat(dir); os.IsNotExist(err) {
		logrus.Debugf("Creating directory: %s", dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		d.toClean[dir] = true
	} else if !stat.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	for _, line := range lines {
		content := []byte(line + "\n")
		logrus.Debugf("Setting %s: %s", target, line)
		if err := ioutil.WriteFile(target, content, 0755); err != nil {
			logrus.Errorf("Failed to write %s to %s: %v", line, target, err)
			return err
		}
	}

	return nil
}

func (d *Device) start() (err error) {
	err = d.findDevice()
	if err != nil {
		return
	}
	d.cmdChan = make(chan *SCSICmd, 5)
	d.respChan = make(chan SCSIResponse, 5)
	go d.beginPoll()
	d.scsi.DevReady(d.cmdChan, d.respChan)
	return
}

func (d *Device) findDevice() error {
	err := filepath.Walk("/dev", func(path string, i os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if i.IsDir() && path != "/dev" {
			return filepath.SkipDir
		}
		if !strings.HasPrefix(i.Name(), "uio") {
			return nil
		}
		sysfile := fmt.Sprintf("/sys/class/uio/%s/name", i.Name())
		bytes, err := ioutil.ReadFile(sysfile)
		if err != nil {
			return err
		}
		split := strings.SplitN(strings.TrimRight(string(bytes), "\n"), "/", 4)
		if split[0] != "tcm-user" {
			// Not a TCM device
			log.Debugf("%s is not a tcm-user device", i.Name())
			return nil
		}
		if split[3] != d.GetDevConfig() {
			// Not a TCM device
			log.Debugf("%s is not our tcm-user device", i.Name())
			return nil
		}
		err = d.openDevice(split[1], split[2], i.Name())
		if err != nil {
			return err
		}
		return filepath.SkipDir
	})
	if err == filepath.SkipDir {
		return nil
	}
	return err
}

func (d *Device) openDevice(user string, vol string, uio string) error {
	var err error
	d.deviceName = vol
	//d.uioFd, err = syscall.Open(fmt.Sprintf("/dev/%s", uio), syscall.O_RDWR|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0600)
	d.uioFd, err = syscall.Open(fmt.Sprintf("/dev/%s", uio), syscall.O_RDWR|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	bytes, err := ioutil.ReadFile(fmt.Sprintf("/sys/class/uio/%s/maps/map0/size", uio))
	if err != nil {
		return err
	}
	d.mapsize, err = strconv.ParseUint(strings.TrimRight(string(bytes), "\n"), 0, 64)
	if err != nil {
		return err
	}
	d.mmap, err = syscall.Mmap(d.uioFd, 0, int(d.mapsize), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	d.cmdTail = d.mbCmdTail()
	d.debugPrintMb()
	return err
}

func (d *Device) debugPrintMb() {
	log.Debugf("Got a TCMU mailbox, version: %d\n", d.mbVersion())
	log.Debugf("mapsize: %d\n", d.mapsize)
	log.Debugf("mbFlags: %d\n", d.mbFlags())
	log.Debugf("mbCmdrOffset: %d\n", d.mbCmdrOffset())
	log.Debugf("mbCmdrSize: %d\n", d.mbCmdrSize())
	log.Debugf("mbCmdHead: %d\n", d.mbCmdHead())
	log.Debugf("mbCmdTail: %d\n", d.mbCmdTail())
}

func (d *Device) teardown() error {
	dev := filepath.Join(d.devPath, d.scsi.VolumeName)
	tpgtPath, _ := d.getSCSIPrefixAndWnn()
	lunPath := d.getLunPath(tpgtPath)

	/*
		We're removing:
		/sys/kernel/config/target/loopback/naa.<id>/tpgt_1/lun/lun_0/<volume name>
		/sys/kernel/config/target/loopback/naa.<id>/tpgt_1/lun/lun_0
		/sys/kernel/config/target/loopback/naa.<id>/tpgt_1
		/sys/kernel/config/target/loopback/naa.<id>
		/sys/kernel/config/target/core/user_42/<volume name>
	*/
	pathsToRemove := []string{
		path.Join(lunPath, d.scsi.VolumeName),
		lunPath,
		tpgtPath,
		path.Dir(tpgtPath),
		path.Join(d.hbaDir, d.scsi.VolumeName),
	}

	for _, p := range pathsToRemove {
		if k, _ := d.toClean[p]; k {
			err := remove(p)
			if err != nil {
				logrus.Errorf("Failed to remove: %v", err)
			}
		}
	}

	// Should be cleaned up automatically, but if it isn't remove it
	if _, err := os.Stat(dev); err == nil {
		if k, _ := d.toClean[dev]; k {
			err := remove(dev)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func removeAsync(path string, done chan<- error) {
	logrus.Debugf("Removing: %s", path)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logrus.Errorf("Unable to remove: %v", path)
		done <- err
	}
	logrus.Debugf("Removed: %s", path)
	done <- nil
}

func remove(path string) error {
	done := make(chan error)
	go removeAsync(path, done)
	select {
	case err := <-done:
		return err
	case <-time.After(30 * time.Second):
		return fmt.Errorf("Timeout trying to delete %s.", path)
	}
}
