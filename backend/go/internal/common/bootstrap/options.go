package bootstrap

// Option is a functional option for Run.
type Option func(*runCfg)

type runCfg struct {
	skipPG  bool
	skipRDB bool
	skipNATS bool
	skipCH  bool
}

// WithoutPG disables PostgreSQL connection.
func WithoutPG() Option { return func(c *runCfg) { c.skipPG = true } }

// WithoutRedis disables Redis connection.
func WithoutRedis() Option { return func(c *runCfg) { c.skipRDB = true } }

// WithoutNATS disables NATS connection.
func WithoutNATS() Option { return func(c *runCfg) { c.skipNATS = true } }

// WithoutCH disables ClickHouse connection.
func WithoutCH() Option { return func(c *runCfg) { c.skipCH = true } }
