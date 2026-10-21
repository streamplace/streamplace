package atproto

import "stream.place/streamplace/pkg/statedb"

var handleLocks = statedb.NewNamedLocks()
var pdsLocks = statedb.NewNamedLocks()

// identityLocks serializes the resolve-then-write of an identity refresh per
// account, so that two refreshers -- the sweep's head check and a firehose
// #identity event, say -- cannot interleave and leave the older resolution
// as the last one written.
var identityLocks = statedb.NewNamedLocks()
