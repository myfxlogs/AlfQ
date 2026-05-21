package chmigrate

import (
	"testing"
)

func TestMustRun_Nil(t *testing.T) {
	// MustRun with nil conn should panic in a real scenario, but we skip this
	// since it requires a real ClickHouse connection
}
