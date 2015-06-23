package args

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/utils"
)

var args struct {
	ncpu     int
	parallel int
	udp      struct {
		listen []uint16
	}
	rpc struct {
		listen uint16
		remote struct {
			ip   string
			port uint16
		}
	}
	heartbeat int
	manage    int
	retrans   []int
	http      uint16
	apps      []string
	debug     bool
}

func init() {
	var ncpu, parallel, manage, heartbeat int
	var rtmfp, listen, remote, http, apps, retrans string
	var debug bool

	flag.IntVar(&ncpu, "ncpu", 1, "maximum number of CPUs, in [1, 1024]")
	flag.IntVar(&parallel, "parallel", 32, "number of parallel worker-routins per connection, in [1, 1024]")
	flag.StringVar(&rtmfp, "rtmfp", "1935", "rtmfp ports list, for example, '1935,1936,1937'")
	flag.StringVar(&listen, "listen", "", "rpc listen port")
	flag.StringVar(&remote, "remote", "", "rpc remote port")
	flag.IntVar(&manage, "manage", 500, "session management interval, in [100, 10000] milliseconds")
	flag.StringVar(&retrans, "retrans", "500,500,1000,1500,1500,2500,3000,4000,5000,7500,10000,15000", "retransmission intervals, in [100, 30000] milliseconds")
	flag.StringVar(&http, "http", "", "default http port")
	flag.StringVar(&apps, "apps", "", "application names, separated by comma")
	flag.IntVar(&heartbeat, "heartbeat", 60, "keep alive message from server, in [1, 60] seconds")
	flag.BoolVar(&debug, "debug", false, "send log to stdio")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	defer func() {
		if x := recover(); x != nil {
			fmt.Fprintf(os.Stderr, "parse argument(s) failed:\n")
			fmt.Fprintf(os.Stderr, "        %s\n", x)
			os.Exit(1)
		}
	}()

	args.debug = debug

	if ncpu < 1 {
		utils.Panic(fmt.Sprintf("invalid ncpu = %d", ncpu))
	} else {
		args.ncpu = ncpu
	}

	if parallel < 1 || parallel > 1024 {
		utils.Panic(fmt.Sprintf("invalid parallel = %d", parallel))
	} else {
		args.parallel = parallel
	}

	if ports, err := parsePorts(rtmfp); err != nil {
		utils.Panic(fmt.Sprintf("invalid rtmfp = '%s', error = '%v'", rtmfp, err))
	} else if len(ports) == 0 {
		utils.Panic(fmt.Sprintf("invalid rtmfp = '%s'", rtmfp))
	} else {
		args.udp.listen = ports
	}

	if listen = trimSpace(listen); len(listen) == 0 {
		args.rpc.listen = 0
	} else if port, err := parsePort(listen); err != nil {
		utils.Panic(fmt.Sprintf("invalid listen = '%s', error = '%v'", listen, err))
	} else {
		args.rpc.listen = port
	}

	if remote = trimSpace(remote); len(remote) == 0 {
		args.rpc.remote.port = 0
	} else if ip, port, err := parseAddr(remote); err != nil {
		utils.Panic(fmt.Sprintf("invalid remote = '%s', error = '%v'", remote, err))
	} else {
		args.rpc.remote.ip, args.rpc.remote.port = ip, port
	}

	if manage < 100 || manage > 10000 {
		utils.Panic(fmt.Sprintf("invalid manage = %d", manage))
	} else {
		args.manage = manage
	}

	if heartbeat < 1 || heartbeat > 60 {
		utils.Panic(fmt.Sprintf("invalid heartbeat = %d", heartbeat))
	} else {
		args.heartbeat = heartbeat
	}

	if values, err := parseInts(retrans); err != nil {
		utils.Panic(fmt.Sprintf("invalid retrans = '%s', error = '%v'", retrans, err))
	} else if len(values) == 0 {
		utils.Panic(fmt.Sprintf("invalid retrans = '%s'", retrans))
	} else {
		for _, v := range values {
			if v < 100 || v > 30000 {
				utils.Panic(fmt.Sprintf("invalid retrans = '%s'", retrans))
			}
		}
		args.retrans = values
	}

	if http = trimSpace(http); len(http) == 0 {
		args.http = 0
	} else if port, err := parsePort(http); err != nil {
		utils.Panic(fmt.Sprintf("invalid http = '%s', error = '%v'", http, err))
	} else {
		args.http = port
	}

	if apps = trimSpace(apps); len(apps) == 0 {
		args.apps = []string{}
	} else {
		set := make(map[string]string)
		for _, s := range strings.Split(apps, ",") {
			if app := trimSpace(s); len(app) != 0 {
				set[app] = app
			}
		}
		for app, _ := range set {
			args.apps = append(args.apps, app)
		}
	}

	if loc, err := time.LoadLocation("Asia/Shanghai"); err != nil {
		log.Printf("[location]: set location failed, error = '%v'\n", err)
	} else {
		log.Printf("[location]: set location = '%v'\n", loc)
	}
	log.Printf("[argument]: %+v", args)

	runtime.GOMAXPROCS(args.ncpu)
}

func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

func parseAddr(s string) (string, uint16, error) {
	if idx := strings.Index(s, ":"); idx > 0 {
		if port, err := parsePort(s[idx+1:]); err == nil {
			return s[:idx], port, nil
		}
	}
	return "", 0, errors.New("bad ip address")
}

func parsePort(s string) (uint16, error) {
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		if p := uint16(v); int64(p) == v && p != 0 {
			return p, nil
		}
	}
	return 0, errors.New("bad port number")
}

func parsePorts(s string) ([]uint16, error) {
	ps := make([]uint16, 0)
	for _, x := range strings.Split(s, ",") {
		if v := trimSpace(x); len(v) != 0 {
			if p, err := parsePort(v); err != nil {
				return nil, err
			} else {
				ps = append(ps, p)
			}
		}
	}
	return ps, nil
}

func parseInt(s string) (int, error) {
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		if u := uint(v); int64(u) == v {
			return int(u), nil
		}
	}
	return 0, errors.New("bad int value")
}

func parseInts(s string) ([]int, error) {
	is := make([]int, 0)
	for _, x := range strings.Split(s, ",") {
		if v := trimSpace(x); len(v) != 0 {
			if i, err := parseInt(v); err != nil {
				return nil, err
			} else {
				is = append(is, i)
			}
		}
	}
	return is, nil
}

func Parallel() int {
	return args.parallel
}

func Manage() int {
	return args.manage
}

func Retrans() []int {
	return args.retrans
}

func HttpPort() uint16 {
	return args.http
}

func Heartbeat() int {
	return args.heartbeat
}

func UdpListenPorts() []uint16 {
	return args.udp.listen
}

func RpcListenPort() uint16 {
	return args.rpc.listen
}

func RpcRemote() (string, uint16) {
	return args.rpc.remote.ip, args.rpc.remote.port
}

func IsDebug() bool {
	return args.debug
}

func IsAuthorizedApp(app string) bool {
	for i := 0; i < len(args.apps); i++ {
		if app == args.apps[i] {
			return true
		}
	}
	return false
}
