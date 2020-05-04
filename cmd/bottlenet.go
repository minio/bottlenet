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
	"os"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio/pkg/console"
)

var coordinatorMessage = `
Run the following command on each of the other nodes.
  $>_ bottlenet THIS-SERVER-ADDR
`

var clientServerMessage = `
Run the following command on each of the server nodes.
  $>_ bottlenet --server THIS-SERVER-ADDR

Run the following command on each of the client nodes.
  $>_ bottlenet --client THIS-SERVER-ADDR
`

var (
	firstPeer         = false
	selfStartCtx      context.Context
	selfStartCancelFn func()
	testChan          chan struct{}
)

func init() {
	selfStartCtx, selfStartCancelFn = context.WithCancel(context.Background())
	testChan = make(chan struct{})
}

func bottlenet(ctx context.Context) error {
	printBottlenetMessage()
	peers = []*node{
		{
			NodeType: nodeTypeSelf,
			Addr:     getLocalIPs()[0],
		},
	}

	if clientMode {
		peers[0].NodeType = nodeTypeClient
	}
	if serverMode {
		peers[0].NodeType = nodeTypeServer
	}

	go runTestController()

	mux := http.NewServeMux()
	mux.HandleFunc("/join", listenJoin)
	mux.HandleFunc("/start", listenStart)
	return serveBottlenet(ctx, mux)
}

func doStart(ctx context.Context, coordinator string) (map[string][]*node, error) {
	client := newClient()
	perfMap := map[string][]*node{}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("http://%s/%s", coordinator, "start"), nil)
	if err != nil {
		return perfMap, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return perfMap, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return perfMap, err
		}
		return perfMap, fmt.Errorf(string(respBody))
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

func listenStart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	endpointsMap := map[string][]*node{}

	for _, p := range peers {
		if clientMode || serverMode {
			break
		}
		if p.Addr == getLocalIPs()[0] {
			continue
		}
		err := doPerf(ctx, p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	for i, p := range peers {
		remotes := []*node{}
		for j, p := range peers {
			if j >= i && !clientMode && !serverMode {
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	dispatchMap, err := json.MarshalIndent(endpointsMap, "", " ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(dispatchMap)
}

func doJoin(ctx context.Context, coordinator string, connbrk chan error) error {
	client := newClient()

	n := &node{
		NodeType: nodeTypePeer,
		Addr:     getLocalIPs()[0],
	}

	if clientMode {
		n.NodeType = nodeTypeClient
	}
	if serverMode {
		n.NodeType = nodeTypeServer
	}

	nBytes, err := json.Marshal(n)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("http://%s/%s", coordinator, "join"), bytes.NewReader(nBytes))
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	go func() {
		_, err := io.Copy(ioutil.Discard, resp.Body)
		connbrk <- err
	}()

	return nil
}

func listenJoin(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p := &node{}
	if err := json.Unmarshal(body, p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := addPeer(p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.(http.Flusher).Flush()
	testChan <- struct{}{}
	<-r.Context().Done()
	fmt.Println("Peer disconnected. Exiting.")
	os.Exit(1)
	removePeer(p)
}

func runTestController() {
	go func() {
		key := make([]byte, 1)
		os.Stdin.Read(key)
		fmt.Println("running bottlenet tests...")
		runTest()
	}()
	for range testChan {
		if len(testChan) > 0 {
			// only run after all nodes have joined
			continue
		}

		console.RewindLines(viewLineCount)
		fmt.Printf("%d peer(s) detected...press any key to begin tests...\n", len(peers)-1)
		viewLineCount = 1

	}
}

func runTest() {
	selfStartCancelFn()
	<-selfStartCtx.Done()
	selfStartCtx, selfStartCancelFn = context.WithCancel(context.Background())

	resp, err := doStart(selfStartCtx, getLocalIPs()[0])
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	printResults(resp)
	selfStartCancelFn()
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

	exit := 0

	defer func() {
		fmt.Println("Exiting.")
		os.Exit(exit)
	}()

	stackRankMap := map[string]float64{}
	total := float64(0)
	for kout := range results {
		nodeAvgSum := float64(0)
		nodeMax := float64(0)
		for kin, v := range results {
			if kin == kout {
				for _, vi := range v {
					for _, vx := range vi.Perf {
						nodeAvgSum = nodeAvgSum + vx.Throughput.Avg
						nodeMax = func() float64 {
							if vx.Throughput.Max > nodeMax {
								return vx.Throughput.Max
							}
							return nodeMax
						}()
						total = total + vx.Throughput.Avg
					}
				}
				continue
			}
			for _, vi := range v {
				for kx, vx := range vi.Perf {
					if kx == kout {
						nodeAvgSum = nodeAvgSum + vx.Throughput.Avg
						nodeMax = func() float64 {
							if vx.Throughput.Max > nodeMax {
								return vx.Throughput.Max
							}
							return nodeMax
						}()
					}
				}
			}
		}
		stackRankMap[kout] = nodeMax
	}

	stackRankKeys := []string{}

	for i := range stackRankMap {
		stackRankKeys = append(stackRankKeys, i)
	}

	sort.Slice(stackRankKeys, func(i, j int) bool {
		left := stackRankKeys[i]
		right := stackRankKeys[j]
		return stackRankMap[left] < stackRankMap[right]
	})

	max := float64(0)
	avg := float64(0)
	for _, k := range stackRankMap {
		k = k / float64(len(stackRankMap)-1)
		avg = avg + k
		if k > max {
			max = k
		}
	}

	// avg = avg / float64(len(stackRankKeys))
	// fmt.Printf("Slowest nodes in your network:\n")
	// for n, k := range stackRankKeys {
	//     s := stackRankMap[k]
	//     ks := k + strings.Repeat(" ", 21-len(k))
	//     fmt.Printf("%d. %s : %s/s \n", n+1, ks, humanize.IBytes(uint64(s)))
	// }

	resJSON, err := json.MarshalIndent(results, "", " ")
	if err != nil {
		fmt.Println(err)
		exit = 1
		return
	}

	filename := fmt.Sprintf("bottlenet_%s.json", time.Now().Format("20060102150405"))
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Println(err)
		exit = 1
		return
	}
	defer f.Close()

	_, err = f.Write(resJSON)
	if err != nil {
		fmt.Println(err)
		exit = 1
		return
	}
	fmt.Println("Bottlenet results saved to", filename)
}

func printBottlenetMessage() {
	if serverMode || clientMode {
		clientServerMsg := strings.ReplaceAll(clientServerMessage, "THIS-SERVER-ADDR", getLocalIPs()[0])
		fmt.Printf("%s\n", clientServerMsg)
		return
	}
	coordMsg := strings.ReplaceAll(coordinatorMessage, "THIS-SERVER-ADDR", getLocalIPs()[0])
	fmt.Printf("%s\n", coordMsg)
}

func getLocalIPs() []string {
	if address != ":7007" {
		return []string{address}
	}
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
				toRet = append(toRet, fmt.Sprintf("%s:7007", ip.String()))
			}
		}
		return toRet
	}()...)

}
