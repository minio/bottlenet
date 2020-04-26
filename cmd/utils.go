package cmd

import (
	"context"
	"fmt"
	"os"
	"time"
	"io"

	"github.com/fatih/color"
	"github.com/minio/minio/pkg/console"
)

const (
	dot   = "●"
	check = "✔"
)

func infoText(s string) string {
	console.SetColor("INFO", color.New(color.FgGreen, color.Bold))
	return console.Colorize("INFO", s)
}

func greenText(s string) string {
	console.SetColor("GREEN", color.New(color.FgGreen))
	return console.Colorize("GREEN", s)
}

func warnText(s string) string {
	console.SetColor("WARN", color.New(color.FgRed, color.Bold))
	return console.Colorize("WARN", s)
}

func spinner(ctx context.Context, msg string) func(bool) bool {
	spinners := []string{"/", "|", "\\", "--", "|"}

	startSpinner := func(s string) func() {
		printText := func(t string, sp string, rewind int) {
			console.RewindLines(rewind)

			dot := infoText(dot)
			t = fmt.Sprintf("%s ...", t)
			t = greenText(t)
			sp = infoText(sp)
			toPrint := fmt.Sprintf("%s %s %s ", dot, t, sp)
			fmt.Printf("%s\n", toPrint)
		}
		i := 0
		sp := func() string {
			i = i + 1
			i = i % len(spinners)
			return spinners[i]
		}

		done := make(chan bool)
		doneToggle := false
		innerCtx, cancel := context.WithCancel(ctx)
		go func() {
			printText(s, sp(), 0)
			for {
				<-time.After(500 * time.Millisecond) //2 fps
				if innerCtx.Err() != nil {
					printText(s, check, 1)
					done <- true
					return
				}
				printText(s, sp(), 1)
			}
		}()
		return func() {
			if !doneToggle {
				cancel()
				<-done
				os.Stdout.Sync()
				doneToggle = true
			}
		}
	}

	return func(resource string) func(bool) bool {
		var spinStopper func()
		done := false

		// if globalJSON {
		// 	return func(bool) bool {
		// 		return true
		// 	}
		// }

		return func(cond bool) bool {
			if done {
				return done
			}
			if spinStopper == nil {
				spinStopper = startSpinner(resource)
			}
			if cond {
				spinStopper()
				done = true
			}
			return done
		}
	}(msg)
}

type networkOverloadedErr struct{}

var networkOverloaded networkOverloadedErr

func (n networkOverloadedErr) Error() string {
        return "network overloaded"
}

type progressReader struct {
        r            io.Reader
        progressChan chan int64
}

func (p *progressReader) Read(b []byte) (int, error) {
        n, err := p.r.Read(b)
        if err != nil && err != io.EOF {
                return n, err
        }
        p.progressChan <- int64(n)
        return n, err
}
