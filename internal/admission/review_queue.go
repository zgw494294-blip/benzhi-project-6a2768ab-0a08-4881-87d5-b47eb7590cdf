package admission

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"sort"
	"strings"
	"time"

	"seed-vault-admission/internal/assessment"
)

type ReviewQueueQuery struct {
	SpeciesName string
	Owner       string
	Severity    string
	IssueStatus string
	Sort        string
	PageSize    int
	Cursor      string
}

type ReviewQueueItem struct {
	Batch                   *AdmissionBatch `json:"batch"`
	PendingReviewCount      int             `json:"pendingReviewCount"`
	ReturnedCount           int             `json:"returnedCount"`
	UnclosedSeriousCount    int             `json:"unclosedSeriousCount"`
	QualifiedPacketCount    int             `json:"qualifiedPacketCount"`
	PacketCount             int             `json:"packetCount"`
	EarliestPendingReviewAt *time.Time      `json:"earliestPendingReviewAt,omitempty"`
	CanApprove              bool            `json:"canApprove"`
	Blockers                []string        `json:"blockers"`
	filteredPendingCount    int
}

type ReviewQueueStats struct {
	BatchCount              int `json:"batchCount"`
	PendingReviewIssueCount int `json:"pendingReviewIssueCount"`
	ApprovableBatchCount    int `json:"approvableBatchCount"`
	SeriousIssueBatchCount  int `json:"seriousIssueBatchCount"`
}

type ReviewQueueResult struct {
	Items      []ReviewQueueItem `json:"items"`
	Stats      ReviewQueueStats  `json:"stats"`
	NextCursor string            `json:"nextCursor,omitempty"`
}

func NormalizeReviewQueueQuery(q ReviewQueueQuery) (ReviewQueueQuery, error) {
	q.SpeciesName = strings.TrimSpace(q.SpeciesName)
	q.Owner = strings.TrimSpace(q.Owner)
	q.Severity = strings.TrimSpace(q.Severity)
	q.IssueStatus = strings.TrimSpace(q.IssueStatus)
	q.Sort = strings.TrimSpace(q.Sort)
	q.Cursor = strings.TrimSpace(q.Cursor)
	if q.PageSize == 0 {
		q.PageSize = 20
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		return q, invalid("pageSize 必须在 1 到 100 之间")
	}
	if q.Severity != "" && q.Severity != "serious" && q.Severity != "warning" && q.Severity != "minor" {
		return q, invalid("severity 是未知枚举值")
	}
	if q.IssueStatus != "" && q.IssueStatus != "open" && q.IssueStatus != "returned" && q.IssueStatus != "pending_review" && q.IssueStatus != "closed" {
		return q, invalid("issueStatus 是未知枚举值")
	}
	if q.Sort == "" {
		q.Sort = "earliestPendingAt"
	}
	if q.Sort != "earliestPendingAt" && q.Sort != "unclosedSeriousCount" && q.Sort != "updatedAt" {
		return q, invalid("sort 是未知枚举值")
	}
	return q, nil
}

func (s *Service) SearchReviewQueue(query ReviewQueueQuery) (*ReviewQueueResult, error) {
	query, err := NormalizeReviewQueueQuery(query)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]ReviewQueueItem, 0)
	for _, batch := range s.state.Batches {
		if batch.Status != StatusSubmitted && batch.Status != StatusRemediation {
			continue
		}
		if query.SpeciesName != "" && !strings.Contains(strings.ToLower(batch.SpeciesName), strings.ToLower(query.SpeciesName)) {
			continue
		}
		if query.Owner != "" && !strings.Contains(strings.ToLower(batch.Owner), strings.ToLower(query.Owner)) {
			continue
		}
		matchedIssueFilter := query.Severity == "" && query.IssueStatus == ""
		item := ReviewQueueItem{Batch: clone(batch)}
		for _, packet := range s.state.Packets {
			if packet.BatchID == batch.ID {
				item.PacketCount++
				if assessment.PacketQualified(s.latestDecisions(packet.ID)) {
					item.QualifiedPacketCount++
				}
			}
		}
		for _, issue := range s.state.Issues {
			if issue.BatchID != batch.ID {
				continue
			}
			if (query.Severity == "" || issue.Severity == query.Severity) && (query.IssueStatus == "" || issue.Status == query.IssueStatus) {
				matchedIssueFilter = true
			}
			if issue.Status == "pending_review" {
				item.PendingReviewCount++
				if (query.Severity == "" || issue.Severity == query.Severity) && (query.IssueStatus == "" || issue.Status == query.IssueStatus) {
					item.filteredPendingCount++
				}
				pendingAt := issue.PendingReviewAt
				if pendingAt == nil {
					fallback := batch.UpdatedAt
					pendingAt = &fallback
				}
				if item.EarliestPendingReviewAt == nil || pendingAt.Before(*item.EarliestPendingReviewAt) {
					value := *pendingAt
					item.EarliestPendingReviewAt = &value
				}
			}
			if issue.Status == "returned" {
				item.ReturnedCount++
			}
			if issue.Severity == "serious" && issue.Status != "closed" {
				item.UnclosedSeriousCount++
			}
		}
		if !matchedIssueFilter {
			continue
		}
		progress := s.progress(batch)
		item.Blockers = append([]string(nil), progress.Blockers...)
		item.CanApprove = progress.CanApproveReview
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return queueLess(items[i], items[j], query.Sort) })
	result := &ReviewQueueResult{Items: make([]ReviewQueueItem, 0)}
	result.Stats.BatchCount = len(items)
	for _, item := range items {
		result.Stats.PendingReviewIssueCount += item.filteredPendingCount
		if item.CanApprove {
			result.Stats.ApprovableBatchCount++
		}
		if item.UnclosedSeriousCount > 0 {
			result.Stats.SeriousIssueBatchCount++
		}
	}
	start := 0
	if query.Cursor != "" {
		offset, err := decodeQueueCursor(query.Cursor, query)
		if err != nil {
			return nil, invalid("cursor 无效")
		}
		if offset > len(items) {
			return nil, invalid("cursor 无效或已不属于当前筛选结果")
		}
		start = offset
	}
	end := start + query.PageSize
	if end > len(items) {
		end = len(items)
	}
	result.Items = append([]ReviewQueueItem(nil), items[start:end]...)
	if end < len(items) && end > start {
		result.NextCursor = encodeQueueCursor(end, query)
	}
	return result, nil
}

func queueLess(a, b ReviewQueueItem, sortBy string) bool {
	switch sortBy {
	case "unclosedSeriousCount":
		if a.UnclosedSeriousCount != b.UnclosedSeriousCount {
			return a.UnclosedSeriousCount > b.UnclosedSeriousCount
		}
	case "updatedAt":
		if !a.Batch.UpdatedAt.Equal(b.Batch.UpdatedAt) {
			return a.Batch.UpdatedAt.After(b.Batch.UpdatedAt)
		}
	default:
		if a.EarliestPendingReviewAt == nil && b.EarliestPendingReviewAt != nil {
			return false
		}
		if a.EarliestPendingReviewAt != nil && b.EarliestPendingReviewAt == nil {
			return true
		}
		if a.EarliestPendingReviewAt != nil && b.EarliestPendingReviewAt != nil && !a.EarliestPendingReviewAt.Equal(*b.EarliestPendingReviewAt) {
			return a.EarliestPendingReviewAt.Before(*b.EarliestPendingReviewAt)
		}
	}
	return a.Batch.ID < b.Batch.ID
}

type reviewQueueCursor struct {
	Version   int    `json:"version"`
	Offset    int    `json:"offset"`
	Signature string `json:"signature"`
}

func queueSignature(query ReviewQueueQuery) string {
	values := []string{query.SpeciesName, query.Owner, query.Severity, query.IssueStatus, query.Sort}
	return strings.Join(values, "\x00")
}

func encodeQueueCursor(offset int, query ReviewQueueQuery) string {
	raw, _ := json.Marshal(reviewQueueCursor{Version: 1, Offset: offset, Signature: queueSignature(query)})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeQueueCursor(cursor string, query ReviewQueueQuery) (int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, err
	}
	var value reviewQueueCursor
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return 0, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return 0, invalid("cursor 无效")
	}
	if value.Version != 1 || value.Offset < 1 || value.Signature != queueSignature(query) {
		return 0, invalid("cursor 无效")
	}
	return value.Offset, nil
}
