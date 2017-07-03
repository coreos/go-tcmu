package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/coreos/go-tcmu"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	if len(os.Args) != 2 {
		die("not enough arguments")
	}
	filename := os.Args[1]
	f, err := os.OpenFile(filename, os.O_RDWR, 0700)
	if err != nil {
		die("couldn't open: %v", err)
	}
	defer f.Close()
	fi, _ := f.Stat()
	handler := tcmu.BasicSCSIHandler(f)
	handler.VolumeName = fi.Name()
	handler.DataSizes.VolumeSize = fi.Size()
	d, err := tcmu.OpenTCMUDevice("/dev/tcmufile", handler)
	if err != nil {
		die("couldn't tcmu: %v", err)
	}
	defer d.Close()
	fmt.Printf("go-tcmu attached to %s/%s\n", "/dev/tcmufile", fi.Name())

	mainClose := make(chan bool)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	go func() {
		for _ = range signalChan {
			fmt.Println("\nReceived an interrupt, stopping services...")
			close(mainClose)
		}
	}()
	<-mainClose
}

func die(why string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, why+"\n", args...)
	os.Exit(1)
}
