package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sort"
	
	"github.com/minio/minio/pkg/console"
	"github.com/dustin/go-humanize"
)

var coordinatorMessage = `
Run the following command on each of the other nodes.
  $ bottlenet THIS-SERVER-IP
`

var mid = `where THIS-SERVER-IP is one of`

var meshNodeMessage = `
Connecting to COORDINATOR-IP:PORT...
`
var (
	firstPeer         = false
	selfStartCtx      context.Context
	selfStartCancelFn func()
)

func init() {
	selfStartCtx, selfStartCancelFn = context.WithCancel(context.Background())
}

func coordinate(ctx context.Context) error {
	printCoordinatorMessage()

	_ = func() {
		firstPeerSpinner := spinner(ctx, "waiting for first peer to join")
		lastPeerSpinner := spinner(ctx, "press enter to start the tests")

		lastPeer := false
		go func() {
			readByte := make([]byte, 1)
			os.Stdin.Read(readByte)
			lastPeer = true
		}()

		for {
			if ctx.Err() != nil {
				return
			}
			if firstPeerSpinner(firstPeer) &&
				lastPeerSpinner(lastPeer) {
				return
			}
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/join", listenJoin(ctx))
	mux.HandleFunc("/start", listenStart(ctx))
	return serveBottlenet(ctx, mux)
}

func doStart(ctx context.Context, coordinator string) (map[string][]*node, error) {
	client := newClient()
	perfMap := map[string][]*node{}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://%s:7007/%s", coordinator, "start"), nil)
	if err != nil {
		return perfMap, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return perfMap, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return perfMap, fmt.Errorf("bottlenet perf test canceled")
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return perfMap, err
	}
	//fmt.Println(string(respBody))

	if err := json.Unmarshal(respBody, &perfMap); err != nil {
		return perfMap, err
	}
	return perfMap, err
}

func listenStart(ctx context.Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		endpointsMap := map[string][]*node{}

		for _, p := range peers {
			if p.Addr == getLocalIPs()[0] {
				continue
			}
			err := doPerf(ctx, p)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
		}

		for i, p := range peers {
			remotes := []*node{}
			for j, p := range peers {
				if j >= i {
					break
				}
				pnew := new(node)
				pnew.Addr = p.Addr
				pnew.Perf = p.Perf
				pnew.NodeType = p.NodeType
				remotes = append(remotes, pnew)
			}
			endpointsMap[p.Addr] = remotes
		}

		for addr, remotes := range endpointsMap {
			if err := doDispatch(ctx, addr, remotes); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
		}

		dispatchMap, err := json.MarshalIndent(endpointsMap, "", " ")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(dispatchMap)
	}
}

func doJoin(ctx context.Context, coordinator string, connbrk chan struct{}) error {
	client := newClient()

	n := &node{
		NodeType: nodeTypePeer,
		Addr:     getLocalIPs()[0],
	}

	nBytes, err := json.Marshal(n)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://%s:7007/%s", coordinator, "join"), bytes.NewReader(nBytes))
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	go func() {
		io.Copy(ioutil.Discard, resp.Body)
		connbrk <- struct{}{}
	}()

	return nil
}

func listenJoin(ctx context.Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		p := &node{}
		if err := json.Unmarshal(body, p); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		if err := addPeer(p); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		go runTest()
		<-r.Context().Done()
		removePeer(p)
		return
	}
}

func runTest() {
	selfStartCancelFn()
	selfStartCtx, selfStartCancelFn = context.WithCancel(context.Background())
	resp, err := doStart(selfStartCtx, getLocalIPs()[0])
	if err != nil {
		if e, ok := err.(*url.Error); ok {
			if e.Unwrap().Error() != context.Canceled.Error() {
				fmt.Println(err)
			}
		} else {
			fmt.Println(err)
		}
		return
	}

	printResults(resp)
}

func printResults(results map[string][]*node) {
	/*
	           10       10       5
	       a  <-->  b  <-->  c  <-->  d
	    10 |      5 |               5 |
	       c        d                 a

	   A simple algorithm to derive the slowest node
	   from the edge speeds is to compute the sum of
	   individual edges directly connected to each edge.

	   For instance, the speed of node a will be
	       s(a->b) + s(a->c) + s(a->d)

	   This algorithm is based on the assumption that incoming
	   and outgoing speed on a specific node are the same.
	*/

	stackRankMap := map[string]float64{}
	total := float64(0)
	for kout := range results {
		nodeAvgSum := float64(0)
		nodeMaxSum := float64(0)
		for kin, v := range results {
			if kin == kout {
				for _, vi := range v {
					for _, vx := range vi.Perf {
						nodeAvgSum = nodeAvgSum + vx.Throughput.Avg
						nodeMaxSum = nodeMaxSum + vx.Throughput.Max
						total = total + vx.Throughput.Avg
					}
				}
				continue
			}
			for _, vi := range v {
				for kx, vx := range vi.Perf {
					if kx == kout {
						nodeAvgSum = nodeAvgSum + vx.Throughput.Avg
						nodeMaxSum = nodeMaxSum + vx.Throughput.Max
					}
				}
			}
		}
		stackRankMap[kout] = nodeAvgSum
	}
	// viewLineCount += 3
	
	stackRankKeys := []string{}

	for i := range stackRankMap {
		stackRankKeys = append(stackRankKeys, i)
	}

	sort.Slice(stackRankKeys, func(i, j int) bool {
		left := stackRankKeys[i]
		right := stackRankKeys[j]
		return stackRankMap[left] < stackRankMap[right]
	})

	console.RewindLines(1)
	viewLineCount -= 1

	max := float64(0)
	avg := float64(0)
	for _, k := range stackRankMap {
		k = k / 3.0
		avg = avg + k
		if k > max {
			max = k
		}
	}
	
	avg = avg / float64(len(stackRankKeys))
	fmt.Printf("Total Throughput : %s/s (max)  %s/s (avg) \n\n", humanize.IBytes(uint64(max)), humanize.IBytes(uint64(avg)))
	
	fmt.Printf("Slowest nodes in your network:\n")
	// viewLineCount += 1
	
	for n, k := range stackRankKeys {
		s := stackRankMap[k]
		ks := k + strings.Repeat(" ", 14-len(k))
		fmt.Printf("%d. %s : %s/s \n", n+1, ks, humanize.IBytes(uint64(s/float64(len(stackRankKeys)))))
		// viewLineCount += 1
		if n == 2 {
			break
		}
	}

loop:
	fmt.Printf("\npress Ctrl + c to exit. press 'y' to rerun... ")
	ans := make([]byte, 2)
	os.Stdin.Read(ans)

	if string(ans[0]) == "y" {
		runTest()
	}
	if string(ans[0]) == "\n" {
		console.RewindLines(1)
	}
	goto loop

	return
}

func printCoordinatorMessage() {
	coordMsg := strings.ReplaceAll(coordinatorMessage, "THIS-SERVER-PORT", fmt.Sprintf("%d", bottlenetPort))
	fmt.Printf("%s\n", strings.ReplaceAll(coordMsg, "THIS-SERVER-IP", getLocalIPs()[0]))
	/*	sb := strings.Builder{}
		sb.WriteString(coordMsg)

		ips := getLocalIPs()

		for _, ip := range ips {
			sb.WriteString("\n    ")
			sb.WriteString(ip)
		}
		fmt.Println(sb.String())
	*/
}

func getLocalIPs() []string {
	ips := []string{}

	interfaceAddrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}

	return append(ips, func() []string {
		toRet := []string{}
		for _, inter := range interfaceAddrs {
			ip, _, _ := net.ParseCIDR(inter.String())
			if !ip.IsLoopback() {
				toRet = append(toRet, ip.String())
			}
		}
		return toRet
	}()...)

}
