package progress

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"
)

const (
	labelLength = 40
	termWidth = 80
	updateInterval = time.Second / 10
)

var clearBuf = bytes.Repeat([]byte{' '}, termWidth)

func init() {
	clearBuf[0] = '\r'
	clearBuf[termWidth-1] = '\r'
}

type ProgressReader struct {
	output io.Writer
	reader io.Reader
	total  int64
	read   chan int
	label  string
}

func NewProgressReader(label string, r io.Reader, size int64) io.ReadCloser {
	pr := &ProgressReader{
		output: os.Stderr,
		reader: r,
		total:  size,
		read:   make(chan int),
		label:  label,
	}

	go pr.update()
	return pr
}

func (pr *ProgressReader) Read(b []byte) (int, error) {
	n, err := pr.reader.Read(b)
	pr.read <- n
	return n, err
}

func (pr *ProgressReader) Close() error {
	pr.clearProgress()
	close(pr.read)
	return nil
}

func (pr *ProgressReader) clearProgress() {
	pr.output.Write(clearBuf)
}

func (pr *ProgressReader) printProgress(n int64) {
	percent := float64(n*100) / float64(pr.total)
	formatStr := fmt.Sprintf("\r%%-%dv %%7.2f%%%%", labelLength)
	fmt.Fprintf(pr.output, formatStr, pr.label[:labelLength], percent)
}

func (pr *ProgressReader) update() {
	t := time.NewTicker(updateInterval)
	defer t.Stop()
	var read int64

	for {
		select {
		case n, ok := <-pr.read:
			if !ok {
				return
			}
			read += int64(n)
		case <-t.C:
			pr.printProgress(read)
		}
	}
}
