/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package ioutil

import (
	"io"
	"sync"
)

type conTeeReader struct {
	r     io.Reader
	c     int64
	ch    chan []byte
	quit  chan struct{}
	errch chan error
	wg    *sync.WaitGroup
}

func NewConTeeReader(r io.Reader, w io.Writer) *conTeeReader {
	ch := make(chan []byte, 4)
	errch := make(chan error)
	quit := make(chan struct{})
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			// NOTE: quit is not used, it will race against ch so
			// that ch cann't drain the buffer.
			case buf, ok := <-ch:
				if !ok {
					return
				}
				_, err := w.Write(buf)
				if err != nil {
					errch <- err
					return
				}
			}
		}
	}()
	return &conTeeReader{r, 0, ch, quit, errch, wg}
}

func (r *conTeeReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	r.c += int64(n)
	if n > 0 {
		buf := make([]byte, n)
		copy(buf, p[:n])
		select {
		case r.ch <- buf[:n]:
		case <-r.quit:
			if err == nil {
				err = ErrClosed
			}
			return n, err
		case err := <-r.errch:
			// Same as io.TeeReader.
			return n, err
		}
	}
	return
}

func (r *conTeeReader) Close() error {
	close(r.quit)
	close(r.ch)
	r.wg.Wait()
	return nil
}
