// ximport ensures golang.org/x/* packages are tracked in go.mod.
// This file is intentionally minimal; remove once real code imports these packages.
package ximport

import (
	_ "golang.org/x/crypto/argon2"
	_ "golang.org/x/net/http2"
	_ "golang.org/x/net/http2/h2c"
	_ "golang.org/x/sync/errgroup"
)
