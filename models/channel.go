package models

import (
	"errors"
	"github.com/omzlo/nocand/models/nocan"
	"sort"
	"sync"
	"time"
)

// Channel
//
//
type Channel struct {
	Id        nocan.ChannelId
	Name      string
	Value     []byte
	UpdatedAt time.Time
}

func (c *Channel) String() string {
	return c.Name
}

func (c *Channel) Touch() {
	c.UpdatedAt = time.Now()
}

func (c *Channel) GetContent() []byte {
	return c.Value
}

func (c *Channel) SetContent(content []byte) bool {
	if len(content) > 64 {
		return false
	}
	c.Value = make([]byte, len(content))
	copy(c.Value, content)
	c.Touch()
	return true
}

// ChannelCollection
//
//
type ChannelCollection struct {
	Mutex  sync.RWMutex
	ById   map[nocan.ChannelId]*Channel
	ByName map[string]*Channel
	TopId  nocan.ChannelId
}

func NewChannelCollection() *ChannelCollection {
	cc := &ChannelCollection{
		ById:   make(map[nocan.ChannelId]*Channel),
		ByName: make(map[string]*Channel),
		TopId:  nocan.ChannelId(0),
	}
	return cc
}

func (cc *ChannelCollection) Each(fn func(*Channel)) {
	cc.Mutex.RLock()
	defer cc.Mutex.RUnlock()

	for _, v := range cc.ById {
		fn(v)
	}
}

func (cc *ChannelCollection) EachOrdered(fn func(*Channel)) {
	cc.Mutex.RLock()
	defer cc.Mutex.RUnlock()

	keys := make([]int, 0)
	for k, _ := range cc.ById {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, k := range keys {
		fn(cc.ById[nocan.ChannelId(k)])
	}
}

func (cc *ChannelCollection) Register(channelName string) (*Channel, error) {
	if len(channelName) == 0 {
		return nil, errors.New("Channel cannot be empty")
	}

	if channel := cc.Lookup(channelName); channel != nil {
		return channel, nil
	}

	cc.Mutex.Lock()
	defer cc.Mutex.Unlock()

	for {
		if cc.TopId < 0 {
			cc.TopId = 0
		}
		if channel, ok := cc.ById[cc.TopId]; !ok {
			channel = &Channel{Id: cc.TopId, Name: channelName, UpdatedAt: time.Now()}
			cc.ById[cc.TopId] = channel
			cc.ByName[channelName] = channel
			cc.TopId++
			return channel, nil
		}
		cc.TopId++
	}
	// normally never reached
	// return nil, errors.New("Maximum numver of channels has been reached")
}

func (cc *ChannelCollection) Unregister(channel *Channel) bool {
	cc.Mutex.Lock()
	defer cc.Mutex.Unlock()

	delete(cc.ByName, channel.Name)
	delete(cc.ById, channel.Id)
	return true
}

func (cc *ChannelCollection) Lookup(channelName string) *Channel {
	cc.Mutex.RLock()
	defer cc.Mutex.RUnlock()

	if channel, ok := cc.ByName[channelName]; ok {
		return channel
	}
	return nil
}

func (cc *ChannelCollection) Find(channelId nocan.ChannelId) *Channel {
	cc.Mutex.RLock()
	defer cc.Mutex.RUnlock()

	if channel, ok := cc.ById[channelId]; ok {
		return channel
	}
	return nil
}
