package schedule

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	logtos "github.com/hkjojo/go-toolkits/log/v2/kratos"
	grdb "github.com/linxGnu/grocksdb"
)

const (
	ColumnFamilyDefault = "default"

	MetricMemTablesSize   = "rocksdb.size-all-mem-tables" // write_buffer_size 总的
	MetricBlockCacheUsage = "rocksdb.block-cache-usage"   // 块缓存

	MetricBackgroundErrors      = "rocksdb.background-errors"
	MetricCurSizeActiveMemTable = "rocksdb.cur-size-active-mem-table"
	MetricCurSizeAllMemTables   = "rocksdb.cur-size-all-mem-tables"
	MetricNumImmutableMemTable  = "rocksdb.num-immutable-mem-table"
	MetricNumLiveVersions       = "rocksdb.num-live-versions"
	MetricEstimateLiveDataSize  = "rocksdb.estimate-live-data-size"

	MetricDelayedWriteRate = "rocksdb.actual-delayed-write-rate"

	MetricPendingCompactionBytes = "rocksdb.estimate-pending-compaction-bytes"
	MetricNumRunningCompactions  = "rocksdb.num-running-compactions"
	MetricNumRunningFlushes      = "rocksdb.num-running-flushes"

	// CF
	MetricEstimateKeyNum   = "rocksdb.estimate-num-keys"
	MetricTotalSSTFileSize = "rocksdb.total-sst-files-size"
	MetricTableReadersMem  = "rocksdb.estimate-table-readers-mem" // 索引和过滤块占用内存（CF）

	MetricCFStats = "rocksdb.cfstats-no-file-histogram"
)

type DBColumnFamilyMonitor struct {
	db        *grdb.DB
	cfHandles map[string]*grdb.ColumnFamilyHandle
	log       *logtos.ActsHelper
}

func NewDBColumnFamilyMonitor(db *grdb.DB, cfHandles map[string]*grdb.ColumnFamilyHandle) *DBColumnFamilyMonitor {
	return &DBColumnFamilyMonitor{
		db:        db,
		cfHandles: cfHandles,
	}
}

func (cm *DBColumnFamilyMonitor) Execute(ctx context.Context, logger *logtos.ActsHelper) error {
	cm.log = logger

	for cfName, handle := range cm.cfHandles {
		if cfName == ColumnFamilyDefault {
			continue
		}
		cm.collectColumnFamilyStats(cfName, handle)
	}

	return nil
}

func (cm *DBColumnFamilyMonitor) collectColumnFamilyStats(cfName string, cfHandle *grdb.ColumnFamilyHandle) {
	numKeys := cm.getPropertySafe(cfHandle, MetricEstimateKeyNum)
	sstSize := cm.getPropertySafe(cfHandle, MetricTotalSSTFileSize)
	readerMem := cm.getPropertySafe(cfHandle, MetricTableReadersMem)
	memTablesSize := cm.getPropertySafe(cfHandle, MetricMemTablesSize)

	sz, err := strconv.ParseUint(sstSize, 10, 64)
	if err != nil {
		cm.log.Errorw(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("failed to parse sst_size, %s", err))
	}

	rm, err := strconv.ParseUint(readerMem, 10, 64)
	if err != nil {
		cm.log.Errorw(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("failed to parse reader_mem, %s", err))
	}

	ms, err := strconv.ParseUint(memTablesSize, 10, 64)
	if err != nil {
		cm.log.Errorw(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("failed to parse %s, %s", MetricMemTablesSize, err))
	}

	cm.log.Infow(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("[cf_stats - %s] key_num: %s, sst_size: %s, "+
		"reader_mem: %s, all_mem_tables: %s", cfName, numKeys, formatBytes(sz), formatBytes(rm), formatBytes(ms)))

	value := cm.getPropertySafe(cfHandle, MetricCFStats)
	startIndex := strings.Index(value, "Uptime")
	lines := strings.Split(value[startIndex:], "\n")
	for _, line := range lines {
		if len(line) > 0 {
			if strings.Contains(line, "AddFile") {
				continue
			}
			cm.log.Infow(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("[cf_stats - %s] %s", cfName, line))
		}
	}
	for i := 0; i < 7; i++ {
		fileNum := cm.getPropertySafe(cfHandle, fmt.Sprintf("rocksdb.num-files-at-level%d", i))
		compRatio := cm.getPropertySafe(cfHandle, fmt.Sprintf("rocksdb.compression-ratio-at-level%d", i))
		if fileNum != "0" {
			cm.log.Infow(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("[cf_stats - %s] level: %d, "+
				"file_num: %s, compress_ratio: %s", cfName, i, fileNum, compRatio))
		}
	}
}

func (cm *DBColumnFamilyMonitor) getPropertySafe(cfHandle *grdb.ColumnFamilyHandle, propName string) string {
	value := cm.db.GetPropertyCF(propName, cfHandle)
	if value == "" {
		return "0"
	}

	return value
}

type RocksDBMonitor struct {
	db  *grdb.DB
	log *logtos.ActsHelper
}

func NewRocksDBMonitor(db *grdb.DB) *RocksDBMonitor {
	return &RocksDBMonitor{
		db: db,
	}
}

func (r *RocksDBMonitor) Execute(ctx context.Context, logger *logtos.ActsHelper) error {
	r.log = logger
	r.collectDBStats()
	return nil
}

func (r *RocksDBMonitor) collectDBStats() {
	// 内存相关指标
	memTablesSize := r.db.GetProperty(MetricMemTablesSize)
	blockCacheUsage := r.db.GetProperty(MetricBlockCacheUsage)

	// 性能相关指标
	backgroundErrors := r.db.GetProperty(MetricBackgroundErrors)
	curSizeActiveMemTable := r.db.GetProperty(MetricCurSizeActiveMemTable)
	curSizeAllMemTables := r.db.GetProperty(MetricCurSizeAllMemTables)
	numImmutableMemTable := r.db.GetProperty(MetricNumImmutableMemTable)
	numLiveVersions := r.db.GetProperty(MetricNumLiveVersions)
	estimateLiveDataSize := r.db.GetProperty(MetricEstimateLiveDataSize)

	// 写入相关指标
	delayedWriteRate := r.db.GetProperty(MetricDelayedWriteRate)

	// 压缩和合并相关指标
	pendingCompactionBytes := r.db.GetProperty(MetricPendingCompactionBytes)
	numRunningCompactions := r.db.GetProperty(MetricNumRunningCompactions)
	numRunningFlushes := r.db.GetProperty(MetricNumRunningFlushes)

	memTbSize, err := strconv.ParseUint(memTablesSize, 10, 64)
	if err != nil {
		r.log.Errorw(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("failed to parse mem_tables_size, %s", err))
	}
	blockCaUsage, err := strconv.ParseUint(blockCacheUsage, 10, 64)
	if err != nil {
		r.log.Errorw(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("failed to parse block_cache_usage, %s", err))
	}

	// 记录内存相关指标
	memoryMsg := fmt.Sprintf("[db_stats] mem_tables_size: %s, block_cache_usage: %s", formatBytes(memTbSize), formatBytes(blockCaUsage))
	r.log.Infow(logtos.ModuleSystem, SourceMonitor, memoryMsg)

	activeMemTable, err := strconv.ParseUint(curSizeActiveMemTable, 10, 64)
	if err != nil {
		r.log.Errorw(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("failed to parse %s, %s", MetricCurSizeActiveMemTable, err))
	}
	allMemTables, err := strconv.ParseUint(curSizeAllMemTables, 10, 64)
	if err != nil {
		r.log.Errorw(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("failed to parse %s, %s", MetricCurSizeAllMemTables, err))
	}
	// 记录性能相关指标
	performanceMsg := fmt.Sprintf("[db_stats] background_errors: %s, cur_size_active_mem_table: %s, "+
		"cur_size_all_mem_tables: %s, num_immutable_mem_table: %s, num_live_versions: %s, "+
		"estimate_live_data_size: %s", backgroundErrors, formatBytes(activeMemTable), formatBytes(allMemTables),
		numImmutableMemTable, numLiveVersions, estimateLiveDataSize)
	r.log.Infow(logtos.ModuleSystem, SourceMonitor, performanceMsg)

	// 记录写入相关指标
	writeMsg := fmt.Sprintf("[db_stats] delayed_write_rate: %s", delayedWriteRate)
	r.log.Infow(logtos.ModuleSystem, SourceMonitor, writeMsg)

	if pendingCompactionBytes != "0" {
		pending, err := strconv.ParseUint(pendingCompactionBytes, 10, 64)
		if err != nil {
			r.log.Errorw(logtos.ModuleSystem, SourceMonitor, fmt.Sprintf("failed to parse pending_compaction_bytes, %s", err))
		}
		pendingCompactionBytes = formatBytes(pending)
	}
	// 记录压缩和合并相关指标
	compactionMsg := fmt.Sprintf("[db_stats] pending_compaction_bytes: %s, "+
		"num_running_compactions: %s, num_running_flushes: %s", pendingCompactionBytes, numRunningCompactions, numRunningFlushes)
	r.log.Infow(logtos.ModuleSystem, SourceMonitor, compactionMsg)
}
