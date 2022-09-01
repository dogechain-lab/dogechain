package command

import (
	"log"
	"net/http"
	"time"

	"net/http/pprof"

	"github.com/spf13/cobra"
)

func InitializePprofServer(cmd *cobra.Command) {
	if cmd.Flag(PprofFlag).Changed {
		address := cmd.Flag(PprofAddressFlag).Value.String()

		log.Println("Running pprof server")

		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
		mux.Handle("/debug/pprof/block", pprof.Handler("block"))
		mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))

		pprofSvr := &http.Server{
			Addr:              address,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}

		go func() {
			if err := pprofSvr.ListenAndServe(); err != nil {
				log.Fatalln("Failure in running pprof server", "err", err)
			}
		}()
	}
}
