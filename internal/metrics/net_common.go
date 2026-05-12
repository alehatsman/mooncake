package metrics

import "time"

// Sample interval for delta-based counters (CPU on Linux uses cpuSampleSleep;
// net on both platforms uses this).
const netSampleSleep = 1 * time.Second

// netDevSample is { ifaceName → (rxBytes, txBytes) } for non-loopback ifaces.
// Shared by Linux (/proc/net/dev) and macOS (netstat -ibn) collectors.
type netDevSample map[string]netCounters

type netCounters struct {
	rx uint64
	tx uint64
}

// computeNetBps rolls up rx/tx counter deltas across interfaces present in
// both samples and converts to bytes/sec. Ignores negative deltas (counter
// wrap, interface reset).
func computeNetBps(first, second netDevSample, elapsed time.Duration) (rxBps, txBps int64) {
	secs := elapsed.Seconds()
	if secs <= 0 {
		return 0, 0
	}
	var rx, tx int64
	for iface, b := range second {
		a, ok := first[iface]
		if !ok {
			continue
		}
		if b.rx >= a.rx {
			rx += int64(b.rx - a.rx)
		}
		if b.tx >= a.tx {
			tx += int64(b.tx - a.tx)
		}
	}
	return int64(float64(rx) / secs), int64(float64(tx) / secs)
}
