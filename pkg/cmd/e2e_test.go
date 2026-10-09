package cmd

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	urfavecli "github.com/urfave/cli/v3"
	"stream.place/streamplace/pkg/config"
)

func TestE2EExternalStreamFlag(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{name: "fixture by default", args: []string{"e2e"}},
		{name: "external publisher", args: []string{"e2e", "--external-stream"}, want: true},
		{name: "explicit fixture", args: []string{"e2e", "--external-stream=false"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := makeE2eCommand(&config.BuildFlags{})
			cmd.Writer, cmd.ErrWriter = io.Discard, io.Discard
			stop := errors.New("flags parsed")
			cmd.Before = func(_ context.Context, cmd *urfavecli.Command) (context.Context, error) {
				require.Equal(t, tc.want, cmd.Bool("external-stream"))
				return nil, stop
			}
			require.ErrorIs(t, cmd.Run(t.Context(), tc.args), stop)
		})
	}
}
