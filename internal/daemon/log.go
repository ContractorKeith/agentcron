package daemon

import (
	"encoding/json"
	"github.com/ContractorKeith/agentcron/internal/store"
	"time"
)

func utcPointer(at *time.Time) *time.Time {
	if at == nil {
		return nil
	}
	u := at.UTC()
	return &u
}
func (e *Engine) log(v any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if r, ok := v.(store.Run); ok {
		r.ScheduledFor = r.ScheduledFor.UTC()
		r.StartedAt = utcPointer(r.StartedAt)
		r.EndedAt = utcPointer(r.EndedAt)
		r.OwnerStart = utcPointer(r.OwnerStart)
		r.ProcStart = utcPointer(r.ProcStart)
		v = r
	}
	if e.Log != nil {
		_ = json.NewEncoder(e.Log).Encode(v)
	}
}
