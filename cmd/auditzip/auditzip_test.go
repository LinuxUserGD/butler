package auditzip_test

import (
	"testing"

	"github.com/LinuxUserGD/butler/cmd/auditzip"
	"github.com/itchio/headway/state"
	"github.com/itchio/wharf/wtest"

	_ "github.com/LinuxUserGD/boar/lzmasupport"
)

func TestProtoZip(t *testing.T) {
	consumer := &state.Consumer{
		OnMessage: func(level string, message string) {
			t.Logf("%s %s", level, message)
		},
	}

	upstream := true
	wtest.Must(t, auditzip.Do(consumer, "./testdata/proto.zip", upstream))

	upstream = false
	wtest.Must(t, auditzip.Do(consumer, "./testdata/proto.zip", upstream))
	wtest.Must(t, auditzip.Do(consumer, "./testdata/proto-with-lzma.zip", upstream))
}
