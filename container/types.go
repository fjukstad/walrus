package container

type ContainerStats struct {
	PidsStats  `json:"pids_stats"`
	BlkIoStats `json:"blkio_stats"`
	NumProcs   int `json:"num_procs"`

	CPUStats    `json:"cpu_stats"`
	PrecpuStats struct{} `json:"precpu_stats"`
	MemoryStats `json:"memory_stats"`
	Name        string `json:"name"`
	ID          string `json:"id"`
	Networks    `json:"networks"`

	Preread      string   `json:"preread"`
	Read         string   `json:"read"`
	StorageStats struct{} `json:"storage_stats"`
}

type PidsStats struct {
	Current int `json:"current"`
}

type Networks struct {
	Eth0 `json:"eth0"`
}

type Eth0 struct {
	RxBytes   int `json:"rx_bytes"`
	RxDropped int `json:"rx_dropped"`
	RxErrors  int `json:"rx_errors"`
	RxPackets int `json:"rx_packets"`
	TxBytes   int `json:"tx_bytes"`
	TxDropped int `json:"tx_dropped"`
	TxErrors  int `json:"tx_errors"`
	TxPackets int `json:"tx_packets"`
}

type BlkIoStats struct {
	IoServiceBytesRecursive interface{} `json:"io_service_bytes_recursive"`
	IoServicedRecursive     interface{} `json:"io_serviced_recursive"`
	IoQueueRecursive        interface{} `json:"io_queue_recursive"`
	IoServiceTimeRecursive  interface{} `json:"io_service_time_recursive"`
	IoWaitTimeRecursive     interface{} `json:"io_wait_time_recursive"`
	IoMergedRecursive       interface{} `json:"io_merged_recursive"`
	IoTimeRecursive         interface{} `json:"io_time_recursive"`
	SectorsRecursive        interface{} `json:"sectors_recursive"`
}

type CPUStats struct {
	CPUUsage       `json:"cpu_usage"`
	OnlineCpus     int `json:"online_cpus"`
	SystemCPUUsage int `json:"system_cpu_usage"`
	ThrottlingData `json:"throttling_data"`
}

type PrecpuStats struct {
	CPUUsage       `json:"cpu_usage"`
	ThrottlingData `json:"throttling_data"`

	OnlineCpus     int `json:"online_cpus"`
	SystemCPUUsage int `json:"system_cpu_usage"`
}

type CPUUsage struct {
	PercpuUsage       []int `json:"percpu_usage"`
	TotalUsage        int   `json:"total_usage"`
	UsageInKernelmode int   `json:"usage_in_kernelmode"`
	UsageInUsermode   int   `json:"usage_in_usermode"`
}

type ThrottlingData struct {
	Periods          int `json:"periods"`
	ThrottledPeriods int `json:"throttled_periods"`
	ThrottledTime    int `json:"throttled_time"`
}

type MemoryStats struct {
	Limit    int `json:"limit"`
	MaxUsage int `json:"max_usage"`
	Stats    `json:"stats"`
	Usage    int `json:"usage"`
}

type Stats struct {
	ActiveAnon              int `json:"active_anon"`
	ActiveFile              int `json:"active_file"`
	Cache                   int `json:"cache"`
	Dirty                   int `json:"dirty"`
	HierarchicalMemoryLimit int `json:"hierarchical_memory_limit"`
	HierarchicalMemswLimit  int `json:"hierarchical_memsw_limit"`
	InactiveAnon            int `json:"inactive_anon"`
	InactiveFile            int `json:"inactive_file"`
	MappedFile              int `json:"mapped_file"`
	Pgfault                 int `json:"pgfault"`
	Pgmajfault              int `json:"pgmajfault"`
	Pgpgin                  int `json:"pgpgin"`
	Pgpgout                 int `json:"pgpgout"`
	Rss                     int `json:"rss"`
	RssHuge                 int `json:"rss_huge"`
	Swap                    int `json:"swap"`
	TotalActiveAnon         int `json:"total_active_anon"`
	TotalActiveFile         int `json:"total_active_file"`
	TotalCache              int `json:"total_cache"`
	TotalDirty              int `json:"total_dirty"`
	TotalInactiveAnon       int `json:"total_inactive_anon"`
	TotalInactiveFile       int `json:"total_inactive_file"`
	TotalMappedFile         int `json:"total_mapped_file"`
	TotalPgfault            int `json:"total_pgfault"`
	TotalPgmajfault         int `json:"total_pgmajfault"`
	TotalPgpgin             int `json:"total_pgpgin"`
	TotalPgpgout            int `json:"total_pgpgout"`
	TotalRss                int `json:"total_rss"`
	TotalRssHuge            int `json:"total_rss_huge"`
	TotalSwap               int `json:"total_swap"`
	TotalUnevictable        int `json:"total_unevictable"`
	TotalWriteback          int `json:"total_writeback"`
	Unevictable             int `json:"unevictable"`
	Writeback               int `json:"writeback"`
}
