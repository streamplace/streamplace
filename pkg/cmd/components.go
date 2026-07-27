package cmd

import (
	"context"
	"strings"

	"stream.place/streamplace/pkg/log"
)

// component is one named long-running process that, together with the
// others, makes up the streamplace node. Components are declared as data
// in runMain and all started together by components.run.
type component struct {
	name string
	run  func(ctx context.Context) error
}

type components []component

// add registers a component to be started at run time.
func (cs *components) add(name string, run func(ctx context.Context) error) {
	*cs = append(*cs, component{name: name, run: run})
}

// addIf registers a component only when cond is true, so conditional
// components (TLS servers, gateways, test streams) read as data rather
// than control flow.
func (cs *components) addIf(cond bool, name string, run func(ctx context.Context) error) {
	if cond {
		cs.add(name, run)
	}
}

// run starts every registered component on the supervision group and
// blocks until shutdown. Components start in registration order; the
// first one to return takes the whole node down with it (see
// TimeoutGroup for the exact semantics).
func (cs components) run(ctx context.Context, group *TimeoutGroup) error {
	names := make([]string, 0, len(cs))
	for _, c := range cs {
		names = append(names, c.name)
		group.Go(func() error {
			return c.run(ctx)
		})
	}
	log.Log(ctx, "starting node components", "count", len(names), "components", strings.Join(names, ","))
	return group.Wait()
}
