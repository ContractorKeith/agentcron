package app

import (
	"github.com/ContractorKeith/agentcron/internal/store"
	"time"
)

func resetNextFire(st *store.JobState, j Job, now time.Time) {
	st.NextFire = nil
	if fires := jobFires(j, st.AnchorAt, now, 1); len(fires) > 0 {
		st.NextFire = &fires[0]
	}
	st.UpdatedAt = now
}
