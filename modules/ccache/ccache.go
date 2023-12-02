// SPDX-License-Identifier: GPL-3.0-or-later

package ccache

import (
	_ "embed"
	"time"

	"github.com/netdata/go.d.plugin/agent/module"

	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

//go:embed "config_schema.json"
var configSchema string

const (
	precision = 1000
)

func init() {
	module.Register("ccache", module.Creator{
		JobConfigSchema: configSchema,
		Defaults: module.Defaults{
			Priority: 69696, // copied from the python collector
		},
		Create: func() module.Module { return New() },
	})
}

func New() *Ccache {
	return &Ccache{
		Config: Config{
			Timeout: time.Second * 2,
		},
		charts: charts.Copy(),
	}
}

type Config struct {
	Timeout time.Duration `yaml:"timeout"`
}

type Ccache struct {
	module.Base
	Config `yaml:",inline"`

	charts *module.Charts
}

func (c *Ccache) Init() bool {
	return true
}

func (c *Ccache) Check() bool {
	return len(c.Collect()) > 0
}

func (c *Ccache) Charts() *module.Charts {
	return c.charts
}

func (c *Ccache) Collect() map[string]int64 {
	cmd := exec.Command("ccache", "--print-stats")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error creating StdoutPipe for Cmd", err)
		return nil
	}

	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting Cmd", err)
		return nil
	}

	// Create a scanner to read the output line by line
	scanner := bufio.NewScanner(stdout)

	// Create a map to store the stats
	stats := make(map[string]int64)

	// Parse the output
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 2 {
			// Convert string to uint64
			value, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				fmt.Printf("Error parsing uint64 from string '%s': %s\n", parts[1], err)
				continue
			}
			stats[parts[0]] = value
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from scanner", err)
		return nil
	}

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		fmt.Println("Cmd returned error", err)
		return nil
	}

	mx := make(map[string]int64)
	mx["local_storage_hit"] = stats["local_storage_hit"]
	mx["local_storage_miss"] = stats["local_storage_miss"]

	var total_local_storage_ops = stats["local_storage_hit"] + stats["local_storage_miss"]
	mx["local_storage_hit_percentage"] = (precision * 100 * stats["local_storage_hit"]) / total_local_storage_ops
	mx["local_storage_miss_percentage"] = (precision * 100 * stats["local_storage_miss"]) / total_local_storage_ops

	mx["cache_size"] = stats["cache_size_kibibyte"] * 1024
	mx["files_in_cache"] = stats["files_in_cache"]
	return mx
}

func (c *Ccache) Cleanup() {
}

// GVD: charts.go

const (
	prioCcache = module.Priority + iota
	prioCcacheLocalStorage
	prioCcacheLocalStoragePercentage
	prioCcacheCacheSize
	prioCcacheFilesInCache
)

var charts = module.Charts{
	local_storage.Copy(),
	local_storage_percentage.Copy(),
	cache_size.Copy(),
	files_in_cache.Copy(),
}

var local_storage = module.Chart{
	ID:       "local_storage",
	Title:    "Local Storage Hits/Misses",
	Units:    "count",
	Fam:      "ccache",
	Ctx:      "ccache.local_storage",
	Priority: prioCcacheLocalStorage,
	Type:     module.Stacked,
	Dims: module.Dims{
		{ID: "local_storage_hit", Name: "hits"},
		{ID: "local_storage_miss", Name: "misses"},
	},
}

var local_storage_percentage = module.Chart{
	ID:       "local_storage_percentage",
	Title:    "Local Storage Hits/Misses Percentage",
	Units:    "percentage",
	Fam:      "ccache",
	Ctx:      "ccache.local_storage_percentage",
	Priority: prioCcacheLocalStoragePercentage,
	Type:     module.Stacked,
	Dims: module.Dims{
		{ID: "local_storage_hit_percentage", Name: "hit", Div: precision},
		{ID: "local_storage_miss_percentage", Name: "miss", Div: precision},
	},
}

var cache_size = module.Chart{
	ID:       "cache_size",
	Title:    "Cache size",
	Units:    "bytes",
	Fam:      "ccache",
	Ctx:      "ccache.cache_size",
	Priority: prioCcacheCacheSize,
	Type:     module.Line,
	Dims: module.Dims{
		{ID: "cache_size", Name: "size"},
	},
}

var files_in_cache = module.Chart{
	ID:       "files_incache",
	Title:    "Files in cache",
	Units:    "count",
	Fam:      "ccache",
	Ctx:      "ccache.files_in_cache",
	Priority: prioCcacheFilesInCache,
	Type:     module.Line,
	Dims: module.Dims{
		{ID: "files_in_cache", Name: "files"},
	},
}
