package main

import (
	"fmt"
	"io"
	"sync"
	"time"
)

type monitoring struct {
	sync.Mutex
	startTime                         time.Time
	spentInserting                    time.Duration
	spentContains                     time.Duration
	spentFlattening                   time.Duration
	spentDeleting                     time.Duration
	spentDistancing                   time.Duration
	spentReadingDisk                  time.Duration
	spentWritingDisk                  time.Duration
	spentMinMax                       time.Duration
	spentCachePurging                 time.Duration
	spentCacheReadLocking             time.Duration
	spentCacheReadLockingBeginning    time.Duration
	spentCacheLocking                 time.Duration
	spentCacheItemLocking             time.Duration
	spentBuildingReadLocking          time.Duration
	spentBuildingReadLockingBeginning time.Duration
	spentBuildingLocking              time.Duration
	spentBuildingItemLocking          time.Duration
}

func newMonitoring() *monitoring {
	return &monitoring{startTime: time.Now()}
}

func (m *monitoring) reset() {
	m.startTime = time.Now()
	m.spentInserting = 0
	m.spentContains = 0
	m.spentFlattening = 0
	m.spentDeleting = 0
	m.spentDistancing = 0
	m.spentReadingDisk = 0
	m.spentWritingDisk = 0
	m.spentMinMax = 0
	m.spentCachePurging = 0
	m.spentCacheReadLocking = 0
	m.spentCacheLocking = 0
	m.spentCacheItemLocking = 0
	m.spentBuildingReadLocking = 0
	m.spentBuildingReadLockingBeginning = 0
	m.spentBuildingLocking = 0
	m.spentBuildingItemLocking = 0
}

func (m *monitoring) writeTimes(w io.Writer) {
	m.Lock()
	defer m.Unlock()

	fmt.Fprintf(w, `
inserting: %s
contains: %s
flattening: %s
deleting: %s
distancing: %s
minMaxing: %s
reading disk: %s
writing disk: %s

cache purging: %s
cache read locking: %s
cache item locking: %s
cache locking: %s

building read locking: %s
building node locking: %s
building locking: %s

total: %s
`, m.spentInserting, m.spentContains, m.spentFlattening, m.spentDeleting,
		m.spentDistancing, m.spentMinMax, m.spentReadingDisk, m.spentWritingDisk,
		m.spentCachePurging, m.spentCacheReadLocking, m.spentCacheItemLocking, m.spentCacheLocking,
		m.spentBuildingReadLocking, m.spentBuildingItemLocking, m.spentBuildingLocking,
		time.Since(m.startTime))
}

func (m *monitoring) addInserting(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentInserting += time.Since(t)
}

func (m *monitoring) addContains(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentContains += time.Since(t)
}

func (m *monitoring) addFlattening(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentFlattening += time.Since(t)
}

func (m *monitoring) addDeleting(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentDeleting += time.Since(t)
}

func (m *monitoring) addMinMax(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentMinMax += time.Since(t)
}

func (m *monitoring) addDistancing(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentDistancing += time.Since(t)
}

func (m *monitoring) addReadingDisk(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentReadingDisk += time.Since(t)
}

func (m *monitoring) addWritingDisk(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentWritingDisk += time.Since(t)
}

func (m *monitoring) addCachePurging(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentCachePurging += time.Since(t)
}

func (m *monitoring) addCacheLocking(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentCacheLocking += time.Since(t)
}

func (m *monitoring) addCacheReadLocking(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentCacheReadLocking += time.Since(t)
}

func (m *monitoring) addCacheItemLocking(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentCacheItemLocking += time.Since(t)
}

func (m *monitoring) addBuildingLocking(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentBuildingLocking += time.Since(t)
}

func (m *monitoring) addBuildingReadLocking(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentBuildingReadLocking += time.Since(t)
}

func (m *monitoring) addBuildingReadLockingBeginning(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentBuildingReadLockingBeginning += time.Since(t)
}

func (m *monitoring) addBuildingItemLocking(t time.Time) {
	m.Lock()
	defer m.Unlock()
	m.spentBuildingItemLocking += time.Since(t)
}
