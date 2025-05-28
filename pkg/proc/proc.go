package proc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/mist/mistconfig"
)

func RunMistServer(ctx context.Context, cli *config.CLI) error {
	myself, err := os.Executable()
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "mistconfig.json")
	defer os.Remove(f.Name())
	if err != nil {
		return err
	}
	conf, err := mistconfig.Generate(cli)
	if err != nil {
		return err
	}
	err = os.WriteFile(f.Name(), conf, 0644)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, myself,
		"MistServer",
		"-c", f.Name(),
		"-i", "127.0.0.1",
		"-p", fmt.Sprintf("%d", cli.MistAdminPort),
	)
	cmd.Env = []string{
		"MIST_NO_PRETTY_LOGGING=true",
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}
	group, ctx := errgroup.WithContext(ctx)
	output := fmt.Println
	for i, pipe := range []io.ReadCloser{stdout, stderr} {
		func(i int, pipe io.ReadCloser) {
			group.Go(func() error {
				reader := bufio.NewReader(pipe)

				for {
					line, isPrefix, err := reader.ReadLine()
					if err != nil {
						if !errors.Is(err, io.EOF) {
							_, _ = output(fmt.Sprintf("reader gave error, ending logging for fd=%d err=%s", i+1, err))
						}
						line, _, err := reader.ReadLine()
						if string(line) != "" {
							_, _ = output(string(line))
						}
						return err
					}
					if isPrefix {
						_, _ = output("warning: preceding line exceeds 64k logging limit and was split")
					}
					if string(line) != "" {
						level, procName, pid, path, streamName, msg, err := ParseMistLog(string(line))
						if err != nil {
							log.Log(ctx, "badly formatted mist log", "message", string(line))
						} else {
							log.Log(ctx, msg,
								"level", level,
								"procName", procName,
								"pid", pid,
								"streamName", streamName,
								"caller", path,
							)
						}
					}
				}
			})
		}(i, pipe)
	}

	group.Go(func() error {
		return cmd.Start()
	})

	return group.Wait()
}

func ParseMistLog(str string) (string, string, string, string, string, string, error) {
	parts := strings.Split(str, "|")
	if len(parts) != 6 {
		return "", "", "", "", "", "", fmt.Errorf("badly formatted mist string")
	}
	return parts[0], parts[1], parts[2], parts[3], parts[4], parts[5], nil
}
