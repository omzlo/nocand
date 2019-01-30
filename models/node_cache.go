package models

import (
	"encoding/json"
	"github.com/omzlo/clog"
	"github.com/omzlo/nocand/models/helpers"
	"github.com/omzlo/nocand/models/nocan"
	"os"
	"sort"
	"time"
)

var nodeCache map[string]nocan.NodeId
var reverseNodeCache map[nocan.NodeId]string
var isDirty bool = false
var cacheFile *helpers.FilePath
var delayedSave *time.Timer = nil

type JsonCacheEntry struct {
	Udid   string
	NodeId nocan.NodeId
}

func NodeCacheLoad() error {
	var err error
	var entries []JsonCacheEntry

	cacheFile = helpers.HomeDir().Append(".nocand", "cache")
	if cacheFile == nil {
		return nil
	}

	f, err := os.Open(cacheFile.String())
	defer f.Close()

	if err != nil {
		clog.Debug("Could not open cache file %s: %s", cacheFile, err)
		_, err := os.Create(cacheFile.String())
		if err != nil {
			clog.Warning("Could not create cache file %s: %s", cacheFile, err)
			cacheFile = nil
			return err
		}
		return nil
	}
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&entries)
	if err != nil {
		clog.Warning("Could not read cache file %s: %s", cacheFile, err)
		return err
	}

	for k, v := range entries {
		var udid Udid8
		if err = udid.DecodeString(v.Udid); err != nil {
			clog.Warning("Could not decode cache entry %d in %s: %s", k, cacheFile, err)
			return err
		}
		if other, exists := reverseNodeCache[v.NodeId]; exists {
			clog.Warning("There is already a node %s with id=%d in the cache %s, ignoring node %s with same id", other, v.NodeId, cacheFile, v.Udid)
		} else {
			nodeCache[v.Udid] = v.NodeId
			reverseNodeCache[v.NodeId] = v.Udid
		}
	}

	clog.Info("Loaded node cache file %s with %d entries", cacheFile, len(entries))
	return nil
}

func NodeCacheSave() error {
	var entries []JsonCacheEntry

	if cacheFile == nil || isDirty == false {
		return nil
	}

	for k, v := range nodeCache {
		entries = append(entries, JsonCacheEntry{k, v})
	}

	sort.SliceStable(entries, func(i, j int) bool { return entries[i].NodeId < entries[i].NodeId })

	f, err := os.Create(cacheFile.String())
	defer f.Close()

	if err != nil {
		clog.Debug("Could not create cache file %s: %s", cacheFile, err)
		return err
	}
	encoder := json.NewEncoder(f)
	err = encoder.Encode(entries)
	if err != nil {
		clog.Warning("Could not write cache file %s: %s", cacheFile, err)
		return err
	}

	clog.Info("Saved node cache file %s with %d entries", cacheFile, len(entries))
	return nil
}

func NodeCacheSetEntry(udid Udid8, node_id nocan.NodeId) bool {
	v, exists := nodeCache[udid.String()]
	if exists && v == node_id {
		return false
	}

	if existing_entry, exists := reverseNodeCache[node_id]; exists {
		delete(nodeCache, existing_entry)
	}
	nodeCache[udid.String()] = node_id
	reverseNodeCache[node_id] = udid.String()
	isDirty = true
	if delayedSave == nil {
		delayedSave = time.AfterFunc(1*time.Minute, func() {
			NodeCacheSave()
			delayedSave = nil
		})
	}
	return true
}

func NodeCacheLookup(udid Udid8) nocan.NodeId {
	return nodeCache[udid.String()]
}

func NodeCacheReverseLookup(node_id nocan.NodeId) bool {
	_, ok := reverseNodeCache[node_id]
	return ok
}

func init() {
	nodeCache = make(map[string]nocan.NodeId)
	reverseNodeCache = make(map[nocan.NodeId]string)
}
