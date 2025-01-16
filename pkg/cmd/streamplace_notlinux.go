//go:build !linux

package cmd

import "stream.place/streamplace/pkg/config"

func Start(build *config.BuildFlags) error {
	return start(build, []jobFunc{})
}
