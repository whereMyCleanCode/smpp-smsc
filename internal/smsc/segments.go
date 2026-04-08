package smsc

import (
	"fmt"
	"sync"
	"time"
)

const defaultSegmentsCount = 10

type segmentsMeta struct {
	CreatedAt         time.Time
	FirstPDUCreatedAt time.Time
}

type segmentGroup struct {
	meta     segmentsMeta
	list     [defaultSegmentsCount]*MessageSegment
	bitmap   uint16
	count    uint8
	expected uint8
}

func (g *segmentGroup) isComplete() bool {
	if g.count != g.expected {
		return false
	}
	expectedBitmap := uint16((1 << g.expected) - 1)
	return g.bitmap == expectedBitmap
}

type segmentShard struct {
	mu       sync.RWMutex
	segments map[string]*segmentGroup
}

type SegmentsManager struct {
	shards [256]*segmentShard

	logger Logger

	maxSegmentAge time.Duration
	cleanupTicker *time.Ticker
	idGenerator   IDGenerator
}

func NewSegmentsManager(lgr Logger, segmentTTL time.Duration, generator IDGenerator) *SegmentsManager {
	if segmentTTL <= 0 {
		segmentTTL = 3 * time.Minute
	}
	shards := [256]*segmentShard{}
	for i := range shards {
		shards[i] = &segmentShard{
			segments: make(map[string]*segmentGroup),
		}
	}
	return &SegmentsManager{
		logger:        lgr.WithStr("component", "segments_manager"),
		maxSegmentAge: segmentTTL,
		shards:        shards,
		cleanupTicker: time.NewTicker(time.Minute),
		idGenerator:   generator,
	}
}

func fnv32(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

func (m *SegmentsManager) getShard(key string) *segmentShard {
	hash := fnv32(key)
	return m.shards[hash%256]
}

func (m *SegmentsManager) generateMessageID() (uint64, error) {
	return m.idGenerator.GenerateID()
}

func (m *SegmentsManager) AddSegment(segment *MessageSegment) (uint64, uint32, string, bool, error) {
	messageID, err := m.generateMessageID()
	if err != nil {
		return 0, StatusSysErr, "", false, err
	}
	if segment.SegmentsCount > defaultSegmentsCount || segment.SegmentsCount <= 1 || segment.SegmentSeqNum > defaultSegmentsCount {
		return messageID, StatusInvMsgLen, "", false, fmt.Errorf("invalid segment count")
	}

	shard := m.getShard(segment.SegmentGroupID)
	shard.mu.Lock()
	group, exists := shard.segments[segment.SegmentGroupID]
	if !exists {
		group = &segmentGroup{
			meta: segmentsMeta{
				CreatedAt:         time.Now(),
				FirstPDUCreatedAt: segment.RegisteredAt,
			},
			expected: segment.SegmentsCount,
		}
		shard.segments[segment.SegmentGroupID] = group
	}

	idx := segment.SegmentSeqNum - 1
	if group.list[idx] == nil {
		group.count++
	}

	if group.count == 1 && group.bitmap == 0 {
		segment.MessageID = messageID
	} else {
		for _, seg := range group.list {
			if seg != nil {
				segment.MessageID = seg.MessageID
				messageID = seg.MessageID
				break
			}
		}
	}

	group.list[idx] = segment
	group.bitmap |= 1 << idx

	groupCopy := &segmentGroup{
		meta:     group.meta,
		bitmap:   group.bitmap,
		count:    group.count,
		expected: group.expected,
	}
	for i, seg := range group.list {
		if seg != nil {
			cp := *seg
			groupCopy.list[i] = &cp
		}
	}
	shard.mu.Unlock()

	msg, coding, complete := m.GetCompleteMessage(groupCopy)
	if !complete {
		return messageID, StatusOK, "", false, nil
	}

	deleteShard := m.getShard(segment.SegmentGroupID)
	deleteShard.mu.Lock()
	delete(deleteShard.segments, segment.SegmentGroupID)
	deleteShard.mu.Unlock()

	text, err := DecodeMessage(msg, coding)
	return messageID, StatusOK, text, true, err
}

func (m *SegmentsManager) GetCompleteMessage(group *segmentGroup) ([]byte, uint8, bool) {
	if !group.isComplete() {
		return nil, 0, false
	}

	encoding := uint8(DataCodingDefault)
	fullLen := 0
	for i := uint8(0); i < group.expected; i++ {
		seg := group.list[i]
		if seg == nil {
			return nil, 0, false
		}
		fullLen += len(seg.Text)
		if seg.Encoding == DataCodingUCS2 {
			encoding = DataCodingUCS2
		}
	}

	full := make([]byte, 0, fullLen)
	for seq := uint8(1); seq <= group.expected; seq++ {
		var seg *MessageSegment
		for i := uint8(0); i < defaultSegmentsCount; i++ {
			if group.list[i] != nil && group.list[i].SegmentSeqNum == seq {
				seg = group.list[i]
				break
			}
		}
		if seg == nil {
			return nil, 0, false
		}
		full = append(full, seg.Text...)
	}
	return full, encoding, true
}

func (m *SegmentsManager) startCleanupRoutine(ctxDone <-chan struct{}) {
	for {
		select {
		case <-ctxDone:
			m.cleanupTicker.Stop()
			return
		case <-m.cleanupTicker.C:
			m.cleanup()
		}
	}
}

func (m *SegmentsManager) cleanup() {
	now := time.Now()
	for i := range m.shards {
		shard := m.shards[i]
		toDelete := make([]string, 0)
		shard.mu.Lock()
		for key, group := range shard.segments {
			var oldest time.Time
			empty := true
			for _, seg := range group.list {
				if seg == nil {
					continue
				}
				oldest = seg.RegisteredAt
				empty = false
				break
			}
			if empty || now.Sub(oldest) > m.maxSegmentAge {
				toDelete = append(toDelete, key)
			}
		}
		for _, key := range toDelete {
			delete(shard.segments, key)
		}
		shard.mu.Unlock()
	}
}

func (m *SegmentsManager) Stop() {
	m.cleanupTicker.Stop()
	for i := range m.shards {
		shard := m.shards[i]
		shard.mu.Lock()
		shard.segments = make(map[string]*segmentGroup)
		shard.mu.Unlock()
	}
}
