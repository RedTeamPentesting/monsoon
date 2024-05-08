package producer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/RedTeamPentesting/monsoon/shell"
	"golang.org/x/sync/errgroup"
)

// Exec runs a command and produces each line the command prints.
type Exec struct {
	cmd              string
	shellBaseCommand string
}

// statically ensure that *Exec implements Source
var _ Source = &Exec{}

// NewFile creates a new producer from a reader. If seekable is set to false
// (e.g. for stdin), Yield() returns an error for subsequent runs.
func NewExec(cmd string, shellBaseCommand string) *Exec {
	return &Exec{cmd: cmd, shellBaseCommand: shellBaseCommand}
}

// Yield runs the command and sends all lines printed by it to ch and the number
// of items to the channel count.  Sending stops and ch and count are closed
// when an error occurs or the context is cancelled.
func (e *Exec) Yield(ctx context.Context, ch chan<- string, count chan<- int) (err error) {
	defer close(ch)
	defer close(count)

	commandOutput, commandOutputWriter := io.Pipe()

	var cmd *exec.Cmd
	if e.shellBaseCommand == "" {
		args, err := shell.Split(e.cmd)
		if err != nil {
			return fmt.Errorf("error splitting command %q: %w", e.cmd, err)
		}
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	} else {
		args, err := shell.Split(e.shellBaseCommand)
		if err != nil {
			return fmt.Errorf("error splitting shell base command %q: %w", e.cmd, err)
		}
		cmd = exec.CommandContext(ctx, args[0], append(args[1:], e.cmd)...)
	}
	cmd.Stdout = commandOutputWriter
	cmd.Stderr = os.Stderr

	eg, localCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		err := cmd.Run()

		// close the writer, ignoring any errors
		_ = commandOutputWriter.Close()

		return err
	})

	eg.Go(func() error {
		// io.Copy(os.Stdout, commandOutput)

		num := 0
		sc := bufio.NewScanner(commandOutput)

		for sc.Scan() {
			num++

			select {
			case <-localCtx.Done():
				return nil
			case ch <- sc.Text():
			}
		}

		fmt.Printf("scanner: done, num %v\n", num)

		select {
		case <-localCtx.Done():
		case count <- num:
		}

		fmt.Printf("scanner: done, err %v\n", sc.Err())

		return sc.Err()
	})

	fmt.Printf("exec main, waiting\n")
	err = eg.Wait()
	fmt.Printf("exec main, done, err: %v\n", err)
	return err
}

// allow testing an exec command early before setting up the producer.
func CheckExec(cmd string) error {
	args, err := shell.Split(cmd)
	if err != nil {
		return err
	}

	_, err = exec.LookPath(args[0])
	if err != nil {
		return err
	}

	return nil
}
