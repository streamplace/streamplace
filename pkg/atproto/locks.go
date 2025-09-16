package atproto

import "stream.place/streamplace/pkg/statedb"

var handleLocks = statedb.NewNamedLocks()
var pdsLocks = statedb.NewNamedLocks()
