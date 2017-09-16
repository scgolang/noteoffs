package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/scgolang/midi"
)

func main() {
	var (
		debug      bool
		deviceName string
		timeout    time.Duration
	)
	flag.BoolVar(&debug, "debug", false, "Debug mode.")
	flag.StringVar(&deviceName, "d", "k-board", "MIDI device name.")
	flag.DurationVar(&timeout, "t", 2*time.Second, "Timeout during which we expect to receive a note off.")
	flag.Parse()

	packets, err := getPacketChan(deviceName)
	if err != nil {
		die(err)
	}
	var (
		ctx   = context.Background()
		notes = map[byte]time.Time{}
		tk    = time.NewTicker(20 * time.Millisecond)
	)
	for {
		select {
		case <-ctx.Done():
			die(ctx.Err())
		case pkt := <-packets:
			if pkt.Err != nil {
				die(pkt.Err)
			}
			if debug {
				fmt.Printf("%#v\n", pkt.Data)
				continue
			}
			switch pkt.Data[0] {
			case 0x90:
				notes[pkt.Data[1]] = time.Now() // Note On
			case 0x80:
				notes[pkt.Data[1]] = time.Time{} // Note Off
			}
		case <-tk.C:
			notes = check(notes, timeout)
		}
	}
}

func check(notes map[byte]time.Time, timeout time.Duration) map[byte]time.Time {
	var (
		m   = map[byte]time.Time{}
		now = time.Now()
	)
	for note, t := range notes {
		if t.IsZero() {
			continue
		}
		if now.Sub(t) < timeout {
			m[note] = t
			continue
		}
		fmt.Fprintf(os.Stderr, "Missing Note Off for %d\n", note)
	}
	return m
}

func die(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}

func getPacketChan(deviceName string) (<-chan midi.Packet, error) {
	devices, err := midi.Devices()
	if err != nil {
		return nil, err
	}
	var device *midi.Device

	for _, d := range devices {
		if strings.Contains(strings.ToLower(d.Name), deviceName) {
			device = d
			break
		}
	}
	if device == nil {
		return nil, errors.New("no device named " + deviceName + " detected")
	}
	device.QueueSize = 16 // Arbitrary channel buffer size.

	if err := device.Open(); err != nil {
		return nil, err
	}
	return device.Packets()
}

// NoteOn represents a MIDI Note On event.
type NoteOn struct {
	At   time.Time
	Note byte
}
