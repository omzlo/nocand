package models

import (
	"encoding/hex"
	"errors"
	"fmt"
	"pannetrat.com/nocand/models/nocan"
	"sync"
	"time"
)

/******/

// Udid8
//
//
type Udid8 [8]byte

var NullUdid8 = Udid8{0,0,0,0,0,0,0,0}

func CreateUdid8(b []byte) Udid8 {
	var udid Udid8
	copy(udid[:], b[:8])
	return udid
}

func (id Udid8) String() string {
	retval := ""

	for i := 0; i < len(id); i++ {
		if i > 0 {
			retval += ":"
		}
		retval += hex.EncodeToString(id[i : i+1])
	}
	return retval
}

func (id Udid8) DecodeString(s string) error {
	src := []byte(s)

	if len(id) < 8 {
		return errors.New("Insufficient space to store node uidAttr")
	}

	for i := 0; i < len(s); i += 3 {
		if _, err := hex.Decode(id[i/3:i/3+1], src[i:i+2]); err != nil {
			return err
		}
		if i > 0 && src[i-1] != ':' {
			return errors.New("expected ':' in hex identifier")
		}
	}
	return nil
}

// NodeState
//
//
type NodeState uint

const (
	NodeStateUnknown NodeState = iota
	NodeStateConnecting
	NodeStateConnected
	NodeStateBootloader
	NodeStateRunning
	NodeStateProgramming
	NodeStateUnresponsive
	NodeStateCount
)

var NodeStateStrings = [NodeStateCount]string{
	"unknown",
	"connecting",
	"connected",
	"bootloader",
	"running",
	"programming",
	"unresponsive",
}

const DEFAULT_EXPECT_TIMEOUT = 3 * time.Second

func (ns NodeState) String() string {
	if int(ns) >= len(NodeStateStrings) {
		return "?"
	}
	return NodeStateStrings[ns]
}

// Node
//
//
type Node struct {
	State    NodeState
	Id       nocan.NodeId
	Udid     Udid8
	LastSeen time.Time
}

func NewNode(id nocan.NodeId, udid Udid8) *Node {
	return &Node{State: NodeStateUnknown, Udid: udid, Id: id}
}

func (n *Node) Touch() {
	n.LastSeen = time.Now()
}

func (n *Node) String() string {
	return fmt.Sprintf("N%d (%s)", n.Id, n.Udid)
}

// NodeCollection
//
//
type NodeCollection struct {
	Mutex sync.RWMutex
	Nodes [128]*Node
	Udids map[Udid8]*Node
}

func NewNodeCollection() *NodeCollection {
	return &NodeCollection{Udids: make(map[Udid8]*Node)}
}

func (nc *NodeCollection) Find(id nocan.NodeId) *Node {
	nc.Mutex.RLock()
	defer nc.Mutex.RUnlock()

	if id < 0 {
		return nil
	}
	return nc.Nodes[id]
}

func (nc *NodeCollection) Lookup(udid Udid8) *Node {
	nc.Mutex.RLock()
	defer nc.Mutex.RUnlock()

	if node, ok := nc.Udids[udid]; ok {
		return node
	}
	return nil
}

func (nc *NodeCollection) Each(cb func(node *Node)) {
	nc.Mutex.RLock()
	defer nc.Mutex.RUnlock()

	for _, n := range nc.Nodes {
		if n != nil {
			cb(n)
		}
	}
}

func (nc *NodeCollection) Register(udid Udid8) (*Node, error) {
	if node := nc.Lookup(udid); node != nil {
		//node.MessageQueue = make(chan *nocan.Message, 16)
		node.State = NodeStateConnecting
		return node, nil
	}

	nc.Mutex.Lock()
	defer nc.Mutex.Unlock()

	for i := 1; i < 128; i++ {
		if nc.Nodes[i] == nil {
			//node := &Node{State: NodeStateUnknown, Udid: udid, MessageQueue: make(chan *nocan.Message, 16), Id: nocan.NodeId(i)}
			node := NewNode(nocan.NodeId(i), udid)
			nc.Nodes[i] = node
			nc.Udids[udid] = node
			node.Touch()
			return node, nil
		}
	}

	return nil, errors.New("Maximum number of nodes has been reached.")
}

func (nc *NodeCollection) Unregister(node *Node) bool {
	nc.Mutex.Lock()
	defer nc.Mutex.Unlock()

	delete(nc.Udids, node.Udid)
	nc.Nodes[node.Id] = nil
	return true
}

/***
func (n *Node) SendMessage(m *nocan.Message) error {
	var frame can.Frame
	var pos uint8

	clog.DebugX("** Sending %s **", m)
	pos = 0
	for {
		frame.CanId = m.CanId | can.CANID_MASK_EXTENDED
		if pos == 0 {
			frame.CanId |= nocan.NOCANID_MASK_FIRST
		}
		if m.Dlc-pos > 8 {
			frame.Dlc = 8
		} else {
			frame.Dlc = m.Dlc - pos
			frame.CanId |= nocan.NOCANID_MASK_LAST
		}
		copy(frame.Data[:], m.Data[pos:pos+frame.Dlc])
		if err := rpi.DriverSendCanFrame(&frame); err != nil {
			return err
		}
		pos += frame.Dlc
		if pos >= m.Dlc {
			break
		}
	}
	return nil
}


func (n *Node) ReceiveMessage() *nocan.Message {
	msg, more := <-n.MessageQueue
	if !more {
		return nil
	}
	n.Touch()
	return msg
}

func (n *Node) ExpectSystemMessage(function nocan.MessageType) (*nocan.Message, error) {
	ticker := time.NewTicker(DEFAULT_EXPECT_TIMEOUT)
	defer ticker.Stop()

	select {
	case msg := <-n.MessageQueue:
		if !msg.IsSystemMessage() {
			return msg, errors.New("Expected System message")
		}
		fn, _ := msg.SystemFunctionParam()
		if fn != function {
			return msg, fmt.Errorf("Expected system fumction %d, got %d instead", function, fn)
		}
		return msg, nil

	case <-ticker.C:
		return nil, errors.New("Timeout")
	}
}

func (n *Node) SendSystemMessage(function nocan.MessageType, param uint8, data []byte) error {
	msg := nocan.NewSystemMessage(n.Id, function, param, data)
	return n.SendMessage(msg)
}

func (n *Node) Publish(channel nocan.ChannelId, data []byte) error {
	msg := nocan.NewPublishMessage(n.Id, channel, data)
	return n.SendMessage(msg)
}
***/
