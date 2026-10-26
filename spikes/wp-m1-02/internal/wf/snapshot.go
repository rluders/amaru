package wf

import "context"

// Snapshot is what the matrix checks after a scenario.
type Snapshot struct {
	Execs    map[string]string  `json:"executions"`
	Steps    map[string]StepRow `json:"steps"`
	Events   []string           `json:"events"`
	Effects  map[string]int     `json:"effects"` // external side effects per Step
	Unsent   int                `json:"outbox_unsent"`
	Timers   int                `json:"timers_pending"`
	SeqOK    bool               `json:"event_seq_contiguous"`
	Messages int                `json:"outbox_total"`
}

type StepRow struct {
	Ord      int    `json:"ord"`
	State    string `json:"state"`
	Version  int    `json:"version"`
	Attempts int    `json:"attempts"`
	Job      string `json:"remote_job,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

func (c *Coord) Snapshot(ctx context.Context) (Snapshot, error) {
	s := Snapshot{Execs: map[string]string{}, Steps: map[string]StepRow{}, Effects: map[string]int{}}
	q := func(sql string, f func(r interface{ Scan(...any) error }) error) error {
		rows, err := c.Pool.Query(ctx, sql)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			if err := f(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	}
	if err := q(`select id,state from executions`, func(r interface{ Scan(...any) error }) error {
		var id, st string
		err := r.Scan(&id, &st)
		s.Execs[id] = st
		return err
	}); err != nil {
		return s, err
	}
	if err := q(`select id,ord,state,version,attempts,coalesce(remote_job,''),coalesce(reason,'') from steps`, func(r interface{ Scan(...any) error }) error {
		var id string
		var x StepRow
		err := r.Scan(&id, &x.Ord, &x.State, &x.Version, &x.Attempts, &x.Job, &x.Reason)
		s.Steps[id] = x
		return err
	}); err != nil {
		return s, err
	}
	if err := q(`select kind||coalesce(' '||step_id,'')||coalesce(' ('||data||')','') from events order by exec_id, seq`, func(r interface{ Scan(...any) error }) error {
		var e string
		err := r.Scan(&e)
		s.Events = append(s.Events, e)
		return err
	}); err != nil {
		return s, err
	}
	if err := q(`select step_id, count(*) from world group by step_id`, func(r interface{ Scan(...any) error }) error {
		var id string
		var n int
		err := r.Scan(&id, &n)
		s.Effects[id] = n
		return err
	}); err != nil {
		return s, err
	}
	c.Pool.QueryRow(ctx, `select count(*) from outbox where sent_at is null`).Scan(&s.Unsent)
	c.Pool.QueryRow(ctx, `select count(*) from outbox`).Scan(&s.Messages)
	c.Pool.QueryRow(ctx, `select count(*) from timers where not fired`).Scan(&s.Timers)
	c.Pool.QueryRow(ctx, `select coalesce(bool_and(seq = rn),true) from (select seq, row_number() over (partition by exec_id order by seq) rn from events) x`).Scan(&s.SeqOK)
	return s, nil
}
