package errs_test

import (
	"errors"
	"testing"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/common/errs"
)

func TestNew(t *testing.T) {
	e := errs.New(pb.ErrCode_ERR_UNSPECIFIED, "test")
	if e.Code != pb.ErrCode_ERR_UNSPECIFIED {
		t.Fatal("code mismatch")
	}
	if e.Message != "test" {
		t.Fatal("msg mismatch")
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("cause")
	e := errs.Wrap(pb.ErrCode_ERR_INTERNAL, "wrapped", cause)
	if !errors.Is(e, cause) {
		t.Fatal("unwrap failed")
	}
}

func TestErrorString(t *testing.T) {
	e := errs.New(pb.ErrCode_ERR_INVALID_ARGUMENT, "bad input")
	if e.Error() == "" {
		t.Fatal("empty error string")
	}
	e2 := errs.Wrap(pb.ErrCode_ERR_INTERNAL, "fail", errors.New("boom"))
	if e2.Error() == "" {
		t.Fatal("empty wrapped error string")
	}
}
