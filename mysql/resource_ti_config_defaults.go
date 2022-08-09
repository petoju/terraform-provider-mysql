package mysql

type defaultConfig struct {
	Pd   PdConfigurationKeys
	TiKv TiKvConfigurationKeys
}

type PdConfigurationKeys struct {
	// The cluster version
	ClusterVersion    string                `json:"cluster-version" default:"IGNOREONDESTROY#"`
	Log               pdLogKeys             `json:"log"`
	Schedule          pdScheduleKeys        `json:"schedule"`
	Replication       PdReplicationKeys     `json:"replication"`
	PdServer          PdServerKeys          `json:"pd-server"`
	PdReplicationMode PdReplicationModeKeys `json:"replication-mode"`
}

type pdLogKeys struct {
	// The log level
	Level string `json:"level" default:"info"`
}

type pdScheduleKeys struct {
	// Controls the size limit of Region Merge (in MiB)
	MaxMergeRegionSize int64 `json:"max-merge-region-size" default:"20"`
	// Specifies the maximum numbers of the Region Merge keys
	MaxMergeRegionKeys int64 `json:"max-merge-region-keys" default:"200000"`
	// Determines the frequency at which replicaChecker checks the health state of a Region
	PatrolRegionInterval string `json:"patrol-region-interval" default:"10ms"`
	// Determines the time interval of performing split and merge operations on the same Region
	SplitMergeInterval string `json:"split-merge-interval" default:"1h"`
	// Determines the maximum number of snapshots that a single store can send or receive at the same time
	MaxSnapshotCount int64 `json:"max-snapshot-count" default:"64"`
	// Determines the maximum number of pending peers in a single store
	MaxPendingPeerCount int64 `json:"max-pending-peer-count" default:"64"`
	// The downtime after which PD judges that the disconnected store can not be recovered
	MaxStoreDownTime string `json:"max-store-down-time" default:"30m"`
	// Determines the policy of Leader scheduling
	LeaderSchedulePolicy string `json:"leader-schedule-policy" default:"IGNOREONDESTROY#"`
	// The number of Leader scheduling tasks performed at the same time
	LeaderScheduleLimit int64 `json:"leader-schedule-limit" default:"4"`
	// The number of Region scheduling tasks performed at the same time
	RegionScheduleLimit int64 `json:"region-schedule-limit" default:"2048"`
	// The number of Replica scheduling tasks performed at the same time
	ReplicaScheduleLimit int64 `json:"replica-schedule-limit" default:"64"`
	// The number of the Region Merge scheduling tasks performed at the same time
	MergeScheduleLimit int64 `json:"merge-schedule-limit" default:"8"`
	// The number of hot Region scheduling tasks performed at the same time
	HotRegionScheduleLimit int64 `json:"hot-region-schedule-limit" default:"4"`
	// Determines the threshold at which a Region is considered a hot spot
	HotRegionCacheHitsThreshold int64 `json:"hot-region-cache-hits-threshold" default:"3"`
	// The threshold ratio below which the capacity of the store is sufficient
	HighSpaceRatio float64 `json:"high-space-ratio" default:"0.7"`
	// The threshold ratio above which the capacity of the store is insufficient
	LowSpaceRatio float64 `json:"low-space-ratio" default:"0.8"`
	// Controls the balance buffer size
	TolerantSizeRatio int64 `json:"tolerant-size-ratio" default:"0"`
	// Determines whether to enable the feature that automatically removes DownReplica
	EnableRemoveDownReplica bool `json:"enable-remove-down-replica" default:"false"`
	// Determines whether to enable the feature that migrates OfflineReplica
	EnableReplaceOfflineReplica bool `json:"enable-replace-offline-replica" default:"false"`
	// Determines whether to enable the feature that automatically supplements replicas
	EnableMakeUpReplica bool `json:"enable-make-up-replica" default:"false"`
	// Determines whether to enable the feature that removes extra replicas
	EnableRemoveExtraReplica bool `json:"enable-remove-extra-replica" default:"false"`
	// Determines whether to enable isolation level check
	EnableLocationReplacement bool `json:"enable-location-replacement" default:"false"`
	// Determines whether to enable cross-table merge
	EnableCrossTableMerge bool `json:"enable-cross-table-merge" default:"true"`
	// Enables one-way merge, which only allows merging with the next adjacent Region
	EnableOneWayMerge bool `json:"enable-one-way-merge" default:"false"`
}

type PdReplicationKeys struct {
	// Sets the maximum number of replicas
	MaxReplicas int64 `json:"max-replicas" default:"3"`
	// The topology information of a TiKV cluster
	LocationLabels string `json:"location-labels" default:"[]"`
	// Enables Placement Rules
	EnablePlacementRules bool `json:"enable-placement-rules" default:"false"`
	// Enables the label check
	StrictlyMatchLabel bool `json:"strictly-match-label" default:"false"`
}

type PdServerKeys struct {
	// Enables independent Region storage
	UseRegionStorage bool `json:"use-region-storage" default:"false"`
	// Sets the maximum interval of resetting timestamp (BR)
	MaxGapResetTs string `json:"max-gap-reset-ts" default:"IGNOREONDESTROY#"`
	// Sets the cluster key type
	KeyType string `json:"key-type" default:"IGNOREONDESTROY#"`
	// Sets the storage address of the cluster metrics
	MetricStorage string `json:"metric-storage" default:"IGNOREONDESTROY#"`
	// Sets the dashboard address
	DashboardAddress string `json:"dashboard-address" default:"IGNOREONDESTROY#"`
}

type PdReplicationModeKeys struct {
	// Sets the backup mode
	ReplicationMode string `json:"replication-mode" default:"IGNOREDESTROY"`
}

type TiKvConfigurationKeys struct {
	Raftstore      tikvRaftstoreKeys      `json:"raftstore"`
	Coprocessor    tikvCoprocessorKeys    `json:"coprocessor"`
	PessimisticTxn tikvPessimisticTxnKeys `json:"pessimistic-txn"`
	Readpool       tikvReadpoolKeys       `json:"readpool"`
	Backup         tikvBackupKeys         `json:"backup"`
	Quota          tikvQuotaKeys          `json:"quota"`
	Gc             tikvGcKeys             `json:"gc"`
	Server         tikvServerKeys         `json:"server"`
	Storage        tikvStroageKeys        `json:"storage"`
	Split          tikvSplitKeys          `json:"split"`
	Cdc            tikvCdcKeys            `json:"cdc"`
	Db             tikvDbGlobalCfgKeys    `json:"defaultDB"`
}

type tikvQuotaKeys struct {
	// The soft limit on the CPU resources used by TiKV foreground to process read and write requests
	ForegroundCpuTime int `json:"foreground-cpu-time" default:"0"`
	// The soft limit on the bandwidth with which transactions write data
	ForegroundWriteBandwidth string `json:"foreground-write-bandwidth" default:"0KB"`
	// The soft limit on the bandwidth with which transactions and the Coprocessor read data
	ForegroundReadBandwidth string `json:"foreground-read-bandwidth" default:"0KB"`
	// The maximum time that a single read or write request is forced to wait before it is processed in the foreground
	MaxDelayDuration string `json:"max-delay-duration" default:"500ms"`
}

type tikvGcKeys struct {
	// The threshold at which Region GC is skipped (the number of GC versions/the number of keys)
	RatioThreshold int64 `json:"ratio-threshold" default:"0"`
	// The number of keys processed in one batch
	BatchKeys int64 `json:"batch-keys" default:"0"`
	// The maximum bytes that can be written into RocksDB per second
	MaxWriteBytesPerSec int64 `json:"max-write-bytes-per-sec" default:"1KB"`
	// Whether to enable compaction filter
	EnableCompactionFilter bool `json:"enable-compaction-filter" default:"true"`
	// Whether to skip the cluster version check of compaction filter (not released)
	CompactionFilterSkipVersionCheck bool `json:"compaction-filter-skip-version-check" default:"false"`
}

type tikvServerKeys struct {
	// Limits the memory size that can be used by gRPC
	GrpcMemoryPoolQuota string `json:"grpc-memory-pool-quota" default:"IGNOREONDESTROY#"`
	// Sets the maximum length of a gRPC message that can be sent
	MaxGrpcSendMsgLen int64 `json:"max-grpc-send-msg-len" default:"10485760"`
	// Sets the maximum number of Raft messages that are contained in a single gRPC message
	RaftMsgMaxBatchSize int64 `json:"raft-msg-max-batch-size" default:"IGNOREONDESTROY#"`
}
type tikvDbGlobalCfgKeys struct {
	// The maximum size of total WAL
	MaxTotalWalSize int64 `json:"max-total-wal-size" default:"IGNOREONDESTROY#"`
	// The number of background threads in RocksDB
	MaxBackgroundJobs int64 `json:"max-background-jobs" default:"IGNOREONDESTROY#[max(2, min({number_of_cores}-1,9))]"`
	// The maximum number of flush threads in RocksDB
	MaxBackgroundFlushes int64 `json:"max-background-flushes" default:"IGNOREONDESTROY#[(max-background-jobs + 3)/4]"`
	// The total number of files that RocksDB can open
	MaxOpenFiles int64 `json:"max-open-files" default:"40960"`
	// The size of readahead during compaction
	CompactionReadaheadSize string `json:"compaction-readahead-size" default:"0B"`
	// The rate at which OS incrementally synchronizes files to disk while these files are being written asynchronously
	BytesPerSync string `json:"bytes-per-sync" default:"1MB"`
	// The rate at which OS incrementally synchronizes WAL files to disk while the WAL files are being written
	WalBytesPerSync string `json:"wal-bytes-per-sync" default:"512KB"`
	// The maximum buffer size used in WritableFileWrite
	WritableFileMaxBufferSize string        `json:"writable-file-max-buffer-size" default:"1MB"`
	DefaultDbConfig           tikvDbCfgKeys `json:"default_db_config"`
}

type tikvDbCfgKeys struct {
	// The cache size of a block
	BlockCacheSize string `json:"block-cache-size" default:"IGNOREONDESTROY#[{'defaultcf': 'Total_machine_memory * 25%', 'writecf': 'Total_machine_memory * 15%','lockcf': 'Total_machine_memory * 2%'}]"`
	// The size of a memtable
	WriteBufferSize string `json:"write-buffer-size" default:"IGNOREONDESTROY#[{'defaultcf': '128MB', 'writecf': '128MB','lockcf': '32MB'}]"`
	// The maximum number of memtables
	MaxWriteBufferNumber int64 `json:"max-write-buffer-number" default:"5"`
	// The maximum number of bytes at base level (L1)
	MaxBytesForLevelBase string `json:"max-bytes-for-level-base" default:"IGNOREONDESTROY#[{'defaultcf': '512MB', 'writecf': '512MB','lockcf': '128MB'}]"`
	// The size of the target file at base level
	TargetFileSizeBase string `json:"target-file-size-base" default:"8MB"`
	// The maximum number of files at L0 that trigger compaction
	Level0FileNumCompactionTrigger int64 `json:"level0-file-num-compaction-trigger" default:"IGNOREONDESTROY#[{'defaultcf': 4, 'writecf': 4, 'lockcf': 1}]"`
	// The maximum number of files at L0 that trigger write stall
	Level0SlowdownWritesTrigger int64 `json:"level0-slowdown-writes-trigger" default:"20"`
	// The maximum number of files at L0 that completely block write
	Level0StopWritesTrigger int64 `json:"level0-stop-writes-trigger" default:"36"`
	// The maximum number of bytes written into disk per compaction
	MaxCompactionBytes string `json:"max-compaction-bytes" default:"2GB"`
	// The default amplification multiple for each layer
	MaxBytesForLevelMultiplier int64 `json:"max-bytes-for-level-multiplier" default:"10"`
	// Enables or disables automatic compaction
	DisableAutoCompactions bool `json:"disable-auto-compactions" default:"false"`
	// The soft limit on the pending compaction bytes
	SoftPendingCompactionBytesLimit string `json:"soft-pending-compaction-bytes-limit" default:"192GB"`
	// The hard limit on the pending compaction bytes
	HardPendingCompactionBytesLimit string `json:"hard-pending-compaction-bytes-limit" default:"1024GB"`
	// The mode of processing blob files
	Titan tikvDbCfgTitan `json:"titan"`
}
type tikvDbCfgTitan struct {
	BlobRunMode string `json:"blob-run-mode" default:"normal"`
}

type tikvStroageKeys struct {
	// The size of shared block cache (supported since v4.0.3)
	BlockCache tikvStorageBlockCache `json:"block-cache"`
	// The number of threads in the Scheduler thread pool
	SchedulerWorkerPoolSize int64 `json:"scheduler-worker-pool-size" default:"4"`
}

type tikvStorageBlockCache struct {
	Capacity string `json:"capacity" default:"IGNOREONDESTROY#[45% f the size of total system memory]"`
}

type tikvSplitKeys struct {
	// The threshold to execute load-base-split on a Region. If the QPS of read requests for a Region exceeds qps-threshold for a consecutive period of time, this Region should be
	QpsThreshold string `json:"qps-threshold" default:"IGNOREONDESTROY#"`
	// The threshold to execute load-base-split on a Region. If the traffic of read requests for a Region exceeds the byte-threshold for a consecutive period of time, this Region should be
	ByteThreshold string `json:"byte-threshold" default:"IGNOREONDESTROY#"`
	// The parameter of load-base-split, which ensures the load of the two split Regions is as balanced as possible. The smaller the value is, the more balanced the load is. But setting it too small might cause split failure.
	SplitBalanceScore string `json:"split-balance-score" default:"IGNOREONDESTROY#"`
	// The parameter of load-base- The smaller the value, the fewer cross-Region visits after Region
	SplitContainedScore string `json:"split-contained-score" default:"IGNOREONDESTROY#"`
}
type tikvCdcKeys struct {
	// The time interval at which Resolved TS is forwarded
	MinTsInterval string `json:"min-ts-interval" default:"1s"`
	// The upper limit of memory occupied by the TiCDC Old Value entries
	OldValueCacheMemoryQuota string `json:"old-value-cache-memory-quota" default:"512MB"`
	// The upper limit of memory occupied by TiCDC data change events
	SinkMemoryQuota string `json:"sink-memory-quota" default:"512MB"`
	// The upper limit on the speed of incremental scanning for historical data
	IncrementalScanSpeedLimit string `json:"incremental-scan-speed-limit" default:"128MB"`
	// The maximum number of concurrent incremental scanning tasks for historical data
	IncrementalScanConcurrency int64 `json:"incremental-scan-concurrency" default:"6"`
}
type tikvReadpoolKeys struct {
	Unifed tikvReadpoolUnified `json:"unifed"`
}

type tikvReadpoolUnified struct {
	//The maximum number of threads in the thread pool that uniformly processes read requests, which is the size of the UnifyReadPool thread pool
	MaxThreadCount int64 `json:"max-thread-count" default:"IGNOREONDESTROY#[MAX(4, cpu_count * 0.8)]"`
}

type tikvBackupKeys struct {
	// The number of backup threads (supported since v4.0.3)
	NumThreads int64 `json:"num-threads" default:"IGNOREONDESTROY#[MIN(CPU * 0.5, 8)]"`
}

type tikvRaftstoreKeys struct {
	// The number of Raft logs to be confirmed. If this number is exceeded, the Raft state machine slows down log sending.
	RaftMaxInflightMsgs int64 `json:"raft-max-inflight-msgs" default:"256"`
	// The time interval at which the polling task of deleting Raft logs is scheduled
	RaftLogGcTickInterval string `json:"raft-log-gc-tick-interval" default:"3s"`
	// The soft limit on the maximum allowable number of residual Raft logs
	RaftLogGcThreshold int64 `json:"raft-log-gc-threshold" default:"50"`
	// The hard limit on the allowable number of residual Raft logs
	RaftLogGcCountLimit int64 `json:"raft-log-gc-count-limit" default:"IGNOREONDESTROY#the log number that can be accommodated in the 3/4 Region size (calculated as 1MB for each log)"`
	// The hard limit on the allowable size of residual Raft logs
	RaftLogGcSizeLimit int64 `json:"raft-log-gc-size-limit" default:"IGNOREONDESTROY#[region_size*3/4]"`
	// The soft limit on the size of a single message packet that is allowed to be generated
	RaftMaxSizePerMsg string `json:"raft-max-size-per-msg" default:"1MB"`
	// The hard limit on the maximum size of a single Raft log
	RaftEntryMaxSize string `json:"raft-entry-max-size" default:"8MB"`
	// The maximum remaining time allowed for the log cache in memory
	RaftEntryCacheLifeTime string `json:"raft-entry-cache-life-time" default:"30s"`
	// The time interval at wh ich to check whether the Region split is needed
	SplitRegionCheckTickInterval string `json:"split-region-check-tick-interval" default:"10s"`
	// The maximum value by which the Region data is allowed to exceed before Region split
	RegionSplitCheckDiff string `json:"region-split-check-diff" default:"IGNOREONDESTROY#[regions_size*1/16]"`
	// The time interval at which to check whether it is necessary to manually trigger RocksDB compaction
	RegionCompactCheckInterval string `json:"region-compact-check-interval" default:"5m"`
	// The number of Regions checked at one time for each round of manual compaction
	RegionCompactCheckStep int64 `json:"region-compact-check-step" default:"100"`
	// The number of tombstones required to trigger RocksDB compaction
	RegionCompactMinTombstones int64 `json:"region-compact-min-tombstones" default:"10000"`
	// The proportion of tombstone required to trigger RocksDB compaction
	RegionCompactTombstonesPercent int64 `json:"region-compact-tombstones-percent" default:"30"`
	// The time interval at which a Region's heartbeat to PD is triggered
	PdHeartbeatTickInterval string `json:"pd-heartbeat-tick-interval" default:"1m"`
	// The time interval at which a store's heartbeat to PD is triggered
	PdStoreHeartbeatTickInterval string `json:"pd-store-heartbeat-tick-interval" default:"10s"`
	// The time interval at which the recycle of expired snapshot files is triggered
	SnapMgrGcTickInterval string `json:"snap-mgr-gc-tick-interval" default:"1m"`
	// The longest time for which a snapshot file is saved
	SnapGcTimeout string `json:"snap-gc-timeout"`
	// The time interval at which TiKV triggers a manual compaction for the Lock Column Family
	LockCfCompactInterval string `json:"lock-cf-compact-interval" default:"10m"`
	// The size at which TiKV triggers a manual compaction for the Lock Column Family
	LockCfCompactBytesThreshold string `json:"lock-cf-compact-bytes-threshold" default:"256MB"`
	// The maximum number of messages processed per batch
	MessagesPerTick int64 `json:"messages-per-tick" default:"4096"`
	// The longest inactive duration allowed for a peer
	MaxPeerDownDuration string `json:"max-peer-down-duration" default:"10m"`
	// The longest duration allowed for a peer to be without a leader. If this value is exceeded, the peer verifies with PD whether it has been deleted.
	MaxLeaderMissingDuration string `json:"max-leader-missing-duration" default:"2h"`
	// The normal duration allowed for a peer to be without a leader. If this value is exceeded, the peer is seen as abnormal and marked in metrics and logs.
	AbnormalLeaderMissingDuration string `json:"abnormal-leader-missing-duration" default:"10m"`
	// The time interval to check whether a peer is without a leader
	PeerStaleStateCheckInterval string `json:"peer-stale-state-check-interval" default:"5m"`
	// The time interval to check consistency (NOT recommended because it is not compatible with the garbage collection in TiDB)
	ConsistencyCheckInterval string `json:"consistency-check-interval" default:"0s"`
	// The longest trusted period of a Raft leader
	RaftStoreMaxLeaderLease string `json:"raft-store-max-leader-lease" default:"9s"`
	// The time interval for merge check
	MergeCheckTickInterval string `json:"merge-check-tick-interval" default:"2s"`
	// The time interval to check expired SST files
	CleanupImportSstInterval string `json:"cleanup-import-sst-interval" default:"10m"`
	// The maximum number of read requests processed in one batch
	LocalReadBatchSize string `json:"local-read-batch-size" default:"1024"`
	// The shortest wait duration before entering hibernation upon start. Within this duration, TiKV does not hibernate (not released).
	HibernateTimeout int64 `json:"hibernate-timeout" default:"0"`
	// The number of threads in the pool that flushes data to the disk, which is the size of the Apply thread pool
	ApplyPoolSize int64 `json:"apply-pool-size" default:"2"`
	// The number of threads in the pool that processes Raft, which is the size of the Raftstore thread pool
	StorePoolSize int64 `json:"store-pool-size" default:"2"`
	// Raft state machines process data write requests in batches by the BatchSystem. This configuration item specifies the maximum number of Raft state machines that can execute the requests in one batch.
	ApplyMaxBatchSize int64 `json:"apply-max-batch-size" default:"256"`
	// Raft state machines process requests for flushing logs into the disk in batches by the BatchSystem. This configuration item specifies the maximum number of Raft state machines that can process the requests in one batch.
	StoreMaxBatchSize int64 `json:"store-max-batch-size" default:"IGNOREONDESTROY#If hibernate-regions is enabled, the default value is 256. If hibernate-regions is disabled, the default value is 1024."`
}

type tikvCoprocessorKeys struct {
	// 	Enables to split Region by table
	SplitRegionOnTable bool `json:"split-region-on-table" default:"false"`
	// The threshold of Region split in batches
	BatchSplitLimit int64 `json:"batch-split-limit" default:"10"`
	// The maximum size of a Region
	RegionMaxSize string `json:"region-max-size" default:"IGNOREONDESTROY#[region-split-size/2*3]"`
	// The size of the newly split Region
	RegionSplitSize string `json:"region-split-size" default:"96MiB"`
	// The maximum number of keys allowed in a Region
	RegionMaxKeys string `json:"region-max-keys" default:"IGNOREONDESTROY#[region-split-keys/2*3]"`
	// The number of keys in the newly split Region
	RegionSplitKeys int64 `json:"region-split-keys" default:"960000"`
}

type tikvPessimisticTxnKeys struct {
	// The longest duration that a pessimistic transaction waits for the lock
	WaitForLockTimeout string `json:"wait-for-lock-timeout" default:"1s"`
	// The duration after which a pessimistic transaction is woken up
	WakeUpDelayDuration string `json:"wake-up-delay-duration" default:"20ms"`
	// Determines whether to enable the pipelined pessimistic locking process
	Pipelined bool `json:"pipelined" default:"true"`
	// Determines whether to enable the in-memory pessimistic lock
	InMemory bool `json:"in-memory" default:"true"`
}
