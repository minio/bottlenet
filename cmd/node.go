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
	"fmt"
	"sync"

	"github.com/minio/bottlenet/pkg/perf"
)

var (
	peers    []*node
	nodeLock sync.Mutex

	c cluster
)

type nodeType int

const (
	nodeTypeSelf nodeType = iota

	//mesh
	nodeTypeCoordinator
	nodeTypePeer

	//client-server
	nodeTypeClient
	nodeTypeServer
)

type node struct {
	NodeType nodeType
	Addr     string
	Perf     map[string]perf.Perf
}

type clusterType int

const (
	clusterTypeMesh clusterType = iota
	clusterTypeClientServer
)

type cluster struct {
	clusterType clusterType
	node        []node
}

func addPeer(p *node) error {
	if c.clusterType != clusterTypeMesh {
		//panic(fmt.Errorf("cannot add peer in client-server mode"))
		if p.NodeType != nodeTypeClient && p.NodeType != nodeTypeServer {
			return fmt.Errorf("could not admit mesh peer to a client-server cluster")
		}
	} else {
		if p.NodeType != nodeTypePeer {
			return fmt.Errorf("could not admit client-server peer to mesh cluster")
		}
	}

	if p == nil {
		return fmt.Errorf("empty peer")
	}
	if p.Addr == "" {
		return fmt.Errorf("peer addr cannot be empty")
	}

	nodeLock.Lock()

	peers = append(peers, p)

	nodeLock.Unlock()

	return nil
}

func removePeer(p *node) {
	nodeLock.Lock()
	todel := -1
	for i, x := range peers {
		if x == p {
			todel = i
			break
		}
	}
	if todel != -1 {
		newpeers := []*node{}
		newpeers = append(newpeers, peers[:todel]...)
		newpeers = append(newpeers, peers[1+todel:]...)
		peers = newpeers
	}
	nodeLock.Unlock()
}

type view struct {
	nodeCount     int
	avgThroughput string
	maxThroughput string

	nodeRanking []*node
}

var viewLineCount int
