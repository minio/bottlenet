/*
 * Bottlenet (C) 2020 MinIO, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/minio/minio/pkg/console"
)

var coordinatorMessage = `
Run the following command on each of the other nodes.
  $ bottlenet THIS-SERVER-IP
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
	viewLineCount--

	max := float64(0)
	avg := float64(0)
	for _, k := range stackRankMap {
		k = k / float64(len(stackRankMap))
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
}

func printCoordinatorMessage() {
	coordMsg := strings.ReplaceAll(coordinatorMessage, "THIS-SERVER-PORT", fmt.Sprintf("%d", bottlenetPort))
	fmt.Printf("%s\n", strings.ReplaceAll(coordMsg, "THIS-SERVER-IP", getLocalIPs()[0]))
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
