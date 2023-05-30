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
	"bytes"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func buildTestBuffer(size int) []byte {
	buf_size := rand.Intn(size)
	buf := make([]byte, buf_size)
	rand.Read(buf)
	return buf
}

func TestConTeeReader(t *testing.T) {
	// read some
	src := bytes.NewBufferString("abc")
	dst := bytes.NewBuffer(make([]byte, 0, 3))

	con := NewConTeeReader(src, dst)

	buf := make([]byte, 1)
	n, err := con.Read(buf)
	assert.Equal(t, 1, n)
	assert.NoError(t, err)
	assert.Equal(t, []byte("a"), buf)

	err = con.Close()
	assert.NoError(t, err)

	assert.Equal(t, []byte("a"), dst.Bytes()[:1])

	// read all
	rand.Seed(time.Now().UnixNano())
	for _, test := range []struct {
		data []byte
	}{
		{data: []byte("This is test buffer.")},
		{data: buildTestBuffer(1024)},
		{data: buildTestBuffer(32 * 1024)},
	} {
		src := bytes.NewBuffer(test.data)
		dst := bytes.NewBuffer(make([]byte, 0, len(test.data)))

		con := NewConTeeReader(src, dst)

		buf := make([]byte, len(test.data))
		n, err := con.Read(buf)
		assert.Equal(t, len(test.data), n)
		assert.NoError(t, err)
		assert.Equal(t, []byte(test.data), buf[:n])

		err = con.Close()
		assert.NoError(t, err)

		assert.Equal(t, []byte(test.data), dst.Bytes()[:n])
	}
}
