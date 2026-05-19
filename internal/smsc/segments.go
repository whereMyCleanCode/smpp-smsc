package smsc

import (
	"fmt"
	"sync"
	"time"
)

const (
	minSubmitSMSegmentsCap = 2
	maxSubmitSMSegmentsCap = 25
)

func normalizeMaxSubmitSMSegments(lgr Logger, n int) int {
	if n < minSubmitSMSegmentsCap {
		lgr.Warn().
			Int("requested", n).
			Int("using", minSubmitSMSegmentsCap).
			Msg("max submit_sm segments below minimum; clamped")
		return minSubmitSMSegmentsCap
	}
	if n > maxSubmitSMSegmentsCap {
		lgr.Warn().
			Int("requested", n).
			Int("using", maxSubmitSMSegmentsCap).
			Msg("max submit_sm segments above maximum; clamped")
		return maxSubmitSMSegmentsCap
	}
	return n
}

type segmentsMeta struct {
	CreatedAt         time.Time
	FirstPDUCreatedAt time.Time
}

type segmentGroup struct {
	meta     segmentsMeta
	list     []*MessageSegment
	count    uint8
	expected uint8
	// OR-accumulated delivery receipt request across all received segments in group.
	deliveryReceiptRequested bool
}

func (g *segmentGroup) isComplete() bool {
	if g.count != g.expected {
		return false
	}
	for i := uint8(0); i < g.expected; i++ {
		if int(i) >= len(g.list) || g.list[i] == nil {
			return false
		}
	}
	return true
}

type segmentShard struct {
	mu       sync.RWMutex
	segments map[string]*segmentGroup
}

type SegmentsManager struct {
	shards [256]*segmentShard

	logger Logger

	maxSegments   int
	maxSegmentAge time.Duration
	cleanupTicker *time.Ticker
	idGenerator   IDGenerator
}

func NewSegmentsManager(lgr Logger, segmentTTL time.Duration, generator IDGenerator, maxSegments int) *SegmentsManager {
	if segmentTTL <= 0 {
		segmentTTL = 3 * time.Minute
	}
	maxSegments = normalizeMaxSubmitSMSegments(lgr, maxSegments)
	shards := [256]*segmentShard{}
	for i := range shards {
		shards[i] = &segmentShard{
			segments: make(map[string]*segmentGroup),
		}
	}
	return &SegmentsManager{
		logger:        lgr.WithStr("component", "segments_manager"),
		maxSegments:   maxSegments,
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

func (m *SegmentsManager) AddSegment(segment *MessageSegment) (uint64, uint32, string, bool, bool, error) {
	messageID, err := m.generateMessageID()
	if err != nil {
		return 0, StatusSysErr, "", false, false, err
	}
	if segment.SegmentsCount <= 1 ||
		segment.SegmentSeqNum == 0 ||
		segment.SegmentSeqNum > segment.SegmentsCount ||
		segment.SegmentsCount > uint8(m.maxSegments) ||
		segment.SegmentSeqNum > uint8(m.maxSegments) {
		return messageID, StatusInvMsgLen, "", false, false, fmt.Errorf("invalid segment count")
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
			list:     make([]*MessageSegment, segment.SegmentsCount),
			expected: segment.SegmentsCount,
		}
		shard.segments[segment.SegmentGroupID] = group
	} else if segment.SegmentsCount != group.expected {
		shard.mu.Unlock()
		return messageID, StatusInvMsgLen, "", false, false, fmt.Errorf("segment count mismatch")
	}

	group.deliveryReceiptRequested = group.deliveryReceiptRequested || segment.DeliveryReceiptRequested

	idx := int(segment.SegmentSeqNum - 1)
	if idx < 0 || idx >= len(group.list) {
		shard.mu.Unlock()
		return messageID, StatusInvMsgLen, "", false, false, fmt.Errorf("invalid segment index")
	}

	oldCount := group.count
	if group.list[idx] == nil {
		group.count++
	}

	if oldCount == 0 {
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

	groupCopy := &segmentGroup{
		meta:                     group.meta,
		count:                    group.count,
		expected:                 group.expected,
		deliveryReceiptRequested: group.deliveryReceiptRequested,
		list:                     make([]*MessageSegment, len(group.list)),
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
		return messageID, StatusOK, "", false, false, nil
	}

	deleteShard := m.getShard(segment.SegmentGroupID)
	deleteShard.mu.Lock()
	delete(deleteShard.segments, segment.SegmentGroupID)
	deleteShard.mu.Unlock()

	text, err := DecodeMessage(msg, coding)
	return messageID, StatusOK, text, true, groupCopy.deliveryReceiptRequested, err
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
		seg := group.list[seq-1]
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
