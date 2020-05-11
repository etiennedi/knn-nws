package main

import (
	"fmt"
	"io"
	"sync"
	"time"
)

type monitoring struct {
	sync.Mutex
	startTime        time.Time
	spentInserting   time.Duration
	spentContains    time.Duration
	spentFlattening  time.Duration
	spentDeleting    time.Duration
	spentDistancing  time.Duration
	spentReadingDisk time.Duration
	spentWritingDisk time.Duration
	spentMinMax      time.Duration
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
total: %s
`, m.spentInserting, m.spentContains, m.spentFlattening, m.spentDeleting,
		m.spentDistancing, m.spentMinMax, m.spentReadingDisk, m.spentWritingDisk, time.Since(m.startTime))
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
