package ledger

import (
	"errors"
	"fmt"
	"strings"
)

type Chain struct {
	events       []Event
	lastDigest   string
	lastSequence uint64
}

func NewChain(events []Event) (*Chain, error) {
	c := &Chain{}
	for _, event := range events {
		if err := c.verifyNext(event); err != nil {
			return nil, err
		}
		c.events = append(c.events, event)
		c.lastSequence = event.Sequence
		c.lastDigest = event.Digest
	}
	if err := validateFrozenConstraint(c.events); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Chain) verifyNext(e Event) error {
	if e.Sequence != c.lastSequence+1 {
		return fmt.Errorf("%s 序号不连续", describe(e))
	}
	if e.PreviousDigest != c.lastDigest {
		return fmt.Errorf("%s 前序摘要不匹配", describe(e))
	}
	got, err := digestEvent(e)
	if err != nil {
		return err
	}
	if got != e.Digest {
		return fmt.Errorf("%s 摘要不匹配", describe(e))
	}
	if strings.TrimSpace(e.Actor) == "" || strings.TrimSpace(e.Kind) == "" || strings.TrimSpace(e.BatchID) == "" {
		return errors.New("事件审计元数据不完整")
	}
	return nil
}

func validateFrozenConstraint(events []Event) error {
	frozen := make(map[string]bool)
	for _, e := range events {
		if frozen[e.BatchID] && e.Kind != "certificate.issued" {
			return fmt.Errorf("批次 %s 冻结后出现非法事件 %s", e.BatchID, e.Kind)
		}
		if e.Kind == "batch.frozen" {
			frozen[e.BatchID] = true
		}
	}
	return nil
}

func (c *Chain) Append(e Event) error {
	if err := c.verifyNext(e); err != nil {
		return err
	}
	c.events = append(c.events, e)
	c.lastSequence = e.Sequence
	c.lastDigest = e.Digest
	return nil
}

func (c *Chain) NextSequence() uint64 { return c.lastSequence + 1 }
func (c *Chain) LastDigest() string   { return c.lastDigest }
func (c *Chain) Events() []Event      { return append([]Event(nil), c.events...) }
