package tcmu

import (
	"errors"
	"io"
	"testing"
)

func TestWrite(t *testing.T) {
	var tests = []struct {
		desc  string
		s     *SCSICmd
		wrote int
		err   error
	}{
		{
			desc: "out of buffer space",
			s: &SCSICmd{
				vecs:      [][]byte{{0}, {1}},
				offset:    0,
				vecoffset: 0,
			},
			wrote: 0,
			err:   errors.New("out of buffer scsi cmd buffer space"),
		},
		{
			desc: "write 3 bytes 3x1",
			s: &SCSICmd{
				vecs:      [][]byte{{0}, {1}, {2}},
				offset:    0,
				vecoffset: 0,
			},
			wrote: 3,
		},
		{
			desc: "write 3 bytes 1x3",
			s: &SCSICmd{
				vecs:      [][]byte{{0, 1, 2}},
				offset:    0,
				vecoffset: 0,
			},
			wrote: 3,
		},
	}

	for i, tt := range tests {
		b := []byte{0, 1, 2}
		wrote, err := tt.s.Write(b)
		if err != nil || tt.err != nil {
			if want, got := tt.err, err; want.Error() != got.Error() {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}
			continue
		}
		want, got := tt.wrote, wrote
		if want != got {
			t.Fatalf("[%02d] test %q, unexpected wrote buffer size:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}

}

func TestRead(t *testing.T) {
	var tests = []struct {
		desc string
		s    *SCSICmd
		read int
		err  error
	}{
		{
			desc: "read exceeded vecs size",
			s: &SCSICmd{
				vecs:      [][]byte{{0}, {1}},
				offset:    0,
				vecoffset: 0,
			},
			read: 0,
			err:  io.EOF,
		},
		{
			desc: "read 3 bytes 3x1",
			s: &SCSICmd{
				vecs:      [][]byte{{0}, {1}, {2}},
				offset:    0,
				vecoffset: 0,
			},
			read: 3,
		},
		{
			desc: "read 3 bytes 1x3",
			s: &SCSICmd{
				vecs:      [][]byte{{0, 1, 2}},
				offset:    0,
				vecoffset: 0,
			},
			read: 3,
		},
	}

	for i, tt := range tests {
		b := []byte{0, 1, 2}
		read, err := tt.s.Read(b)
		if err != nil || tt.err != nil {
			if want, got := tt.err, err; want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}
			continue
		}
		want, got := tt.read, read
		if want != got {
			t.Fatalf("[%02d] test %q, unexpected read buffer size:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}

}

type fakeSCSICmdHandler struct {
	SCSICmdHandler
	FakeHandleCommand func(cmd *SCSICmd) (SCSIResponse, error)
}

func (c *fakeSCSICmdHandler) HandleCommand(cmd *SCSICmd) (SCSIResponse, error) {
	return cmd.Ok(), nil
}

func TestDevReady(t *testing.T) {
	var tests = []struct {
		desc    string
		s       SCSICmdHandler
		id      uint16
		threads int
	}{
		{
			desc:    "DevReady test with SingleThreadedDevReady",
			s:       &fakeSCSICmdHandler{},
			id:      1,
			threads: 1,
		},
		{
			desc:    "DevReady test with MultiThreadedDevReady",
			s:       &fakeSCSICmdHandler{},
			id:      1,
			threads: 2,
		},
	}

	for i, tt := range tests {
		var f DevReadyFunc
		if tt.threads > 1 {
			f = MultiThreadedDevReady(tt.s, tt.threads)
		} else {
			f = SingleThreadedDevReady(tt.s)

		}
		cmdChan := make(chan *SCSICmd, 3)
		respChan := make(chan SCSIResponse, 3)
		f(cmdChan, respChan)
		cmd := &SCSICmd{id: tt.id}
		cmdChan <- cmd
		resp := <-respChan
		want, got := tt.id, resp.id
		if want != got {
			t.Fatalf("[%02d] test %q, unexpected command id in response:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
		close(cmdChan)
		close(respChan)
	}
}
