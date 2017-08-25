//go:generate goversioninfo -icon=script/windows-installer/git-lfs-logo.ico

package main

import (
	"fmt"
	"os"
	"io/ioutil"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
	"runtime/pprof"

	"github.com/git-lfs/git-lfs/commands"
)

func main() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill)

	var once sync.Once

	done := make(chan struct{})
	wg := new(sync.WaitGroup)
	wg.Add(1)
	run := os.Getpid()
	fmem, err := ioutil.TempFile("/tmp", fmt.Sprintf("mem-%d-", run))
	if err != nil {
		panic(err.Error())
	}
	// ### MEMORY STATS
	go func() {
		defer wg.Done()
		f, err := ioutil.TempFile("/tmp", fmt.Sprintf("memstat-%d-", run))
		if err != nil {
			return
		}
		defer f.Close()
		for {
			select {
			case <-time.After(100 * time.Millisecond):
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				if ms.HeapAlloc > 1024*1024*1024 {
					if err := pprof.WriteHeapProfile(fmem); err != nil {
						panic(err.Error())
					}
					fmem.Close()
					fmt.Fprintf(os.Stderr, "\nTOO MUCH MEMORY\n")
					os.Exit(128)
				}
				fmt.Fprintf(f, "%+v\n", ms.HeapAlloc)
			case <-done:
				return
			}
		}
	}()

	go func() {
		for {
			sig := <-c
			once.Do(commands.Cleanup)
			fmt.Fprintf(os.Stderr, "\nExiting because of %q signal.\n", sig)

			exitCode := 1
			if sysSig, ok := sig.(syscall.Signal); ok {
				exitCode = int(sysSig)
			}
			close(done)
			wg.Wait()
			os.Exit(exitCode + 128)
		}
	}()

	commands.Run()
	once.Do(commands.Cleanup)
}
