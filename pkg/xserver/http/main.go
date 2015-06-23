package xserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/args"
	"github.com/spinlock/xserver/pkg/xserver/cookies"
	"github.com/spinlock/xserver/pkg/xserver/counts"
	"github.com/spinlock/xserver/pkg/xserver/session"
	"github.com/spinlock/xserver/pkg/xserver/utils"
)

func init() {
	if port := args.HttpPort(); port != 0 {
		go func() {
			http.HandleFunc("/summary", func(w http.ResponseWriter, r *http.Request) {
				const divMB = uint64(1024 * 1024)
				var mem runtime.MemStats
				runtime.ReadMemStats(&mem)
				s := map[string]interface{}{
					"time": map[string]interface{}{
						"current": time.Now().Unix(),
						"boot":    utils.Starttime,
					},
					"heap": map[string]interface{}{
						"alloc":   mem.HeapAlloc / divMB,
						"sys":     mem.HeapSys / divMB,
						"idle":    mem.HeapIdle / divMB,
						"inuse":   mem.HeapInuse / divMB,
						"objects": mem.HeapObjects / divMB,
					},
					"runtime": map[string]interface{}{
						"routines": runtime.NumGoroutine(),
						"nproc":    runtime.GOMAXPROCS(0),
					},
					"build": map[string]interface{}{
						"version": utils.Version,
						"compile": utils.Compile,
					},
					"cookies": cookies.Count(),
					"session": session.Summary(),
					"streams": session.Streams(),
					"counts":  counts.Snapshot(),
				}
				if b, err := json.MarshalIndent(s, "", "    "); err != nil {
					fmt.Fprintf(w, "json: error = '%v'\n", err)
				} else {
					fmt.Fprintf(w, "%s\n", string(b))
				}
			})
			http.HandleFunc("/mapsize", func(w http.ResponseWriter, r *http.Request) {
				if b, err := json.Marshal(session.MapSize()); err != nil {
					fmt.Fprintf(w, "json: error = '%v'\n", err)
				} else {
					fmt.Fprintf(w, "%s\n", string(b))
				}
			})
			http.HandleFunc("/dumpall", func(w http.ResponseWriter, r *http.Request) {
				if b, err := json.MarshalIndent(session.DumpAll(), "", "    "); err != nil {
					fmt.Fprintf(w, "json: error = '%v'\n", err)
				} else {
					fmt.Fprintf(w, "%s\n", string(b))
				}
			})
			http.HandleFunc("/streams", func(w http.ResponseWriter, r *http.Request) {
				if b, err := json.MarshalIndent(session.DumpStreams(), "", "    "); err != nil {
					fmt.Fprintf(w, "json: error = '%v'\n", err)
				} else {
					fmt.Fprintf(w, "%s\n", string(b))
				}
			})
			if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
				utils.Panic(fmt.Sprintf("http.listen = %d, error = '%v'", port, err))
			}
		}()
	}
}
