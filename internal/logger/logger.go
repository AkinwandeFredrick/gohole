package logger

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Logger struct {
	db   *sql.DB
	logCh chan QueryRecord
}

type QueryRecord struct {
	Timestamp    time.Time
	Domain       string
	QueryType    string
	SourceIP     string
	ResponseType string // blocked, allowed, cached, error
	ListName     string
	LatencyMs    int64
}

type Stats struct {
	TotalQueries   int64
	BlockedQueries int64
	AllowedQueries int64
	CachedQueries  int64
	PercentBlocked float64
}

type DomainCount struct {
	Domain string
	Count  int64
}

type ClientCount struct {
	IP    string
	Count int64
}

func New(dbPath string) (*Logger, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	l := &Logger{
		db:    db,
		logCh: make(chan QueryRecord, 10000),
	}

	go l.worker()

	return l, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS queries (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp    DATETIME NOT NULL,
			domain       TEXT NOT NULL,
			query_type   TEXT NOT NULL,
			source_ip    TEXT NOT NULL,
			response_type TEXT NOT NULL,
			list_name    TEXT,
			latency_ms   INTEGER
		);

		CREATE INDEX IF NOT EXISTS idx_queries_timestamp ON queries(timestamp);
		CREATE INDEX IF NOT EXISTS idx_queries_domain ON queries(domain);
		CREATE INDEX IF NOT EXISTS idx_queries_response_type ON queries(response_type);
		CREATE INDEX IF NOT EXISTS idx_queries_source_ip ON queries(source_ip);
	`)
	return err
}

func (l *Logger) worker() {
	batch := make([]QueryRecord, 0, 100)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case rec, ok := <-l.logCh:
			if !ok {
				l.flushBatch(batch)
				return
			}
			batch = append(batch, rec)
			if len(batch) >= 100 {
				l.flushBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				l.flushBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (l *Logger) flushBatch(batch []QueryRecord) {
	if len(batch) == 0 {
		return
	}

	tx, err := l.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO queries (timestamp, domain, query_type, source_ip, response_type, list_name, latency_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return
	}
	defer stmt.Close()

	for _, r := range batch {
		stmt.Exec(r.Timestamp, r.Domain, r.QueryType, r.SourceIP, r.ResponseType, r.ListName, r.LatencyMs)
	}

	tx.Commit()
}

func (l *Logger) Log(r QueryRecord) {
	select {
	case l.logCh <- r:
	default:
		// Drop if buffer full
	}
}

func (l *Logger) Close() error {
	close(l.logCh)
	time.Sleep(100 * time.Millisecond)
	return l.db.Close()
}

func (l *Logger) GetStats(since time.Duration) (*Stats, error) {
	cutoff := time.Now().Add(-since)

	rows, err := l.db.Query(`
		SELECT response_type, COUNT(*) 
		FROM queries 
		WHERE timestamp > ?
		GROUP BY response_type
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := &Stats{}
	for rows.Next() {
		var rt string
		var count int64
		rows.Scan(&rt, &count)
		stats.TotalQueries += count
		switch rt {
		case "blocked":
			stats.BlockedQueries = count
		case "allowed":
			stats.AllowedQueries = count
		case "cached":
			stats.CachedQueries = count
		}
	}

	if stats.TotalQueries > 0 {
		stats.PercentBlocked = float64(stats.BlockedQueries) / float64(stats.TotalQueries) * 100
	}

	return stats, nil
}

func (l *Logger) GetTopDomains(responseType string, limit int, since time.Duration) ([]DomainCount, error) {
	cutoff := time.Now().Add(-since)

	var query string
	var args []interface{}

	if responseType == "" {
		query = `SELECT domain, COUNT(*) as cnt FROM queries WHERE timestamp > ? GROUP BY domain ORDER BY cnt DESC LIMIT ?`
		args = []interface{}{cutoff, limit}
	} else {
		query = `SELECT domain, COUNT(*) as cnt FROM queries WHERE timestamp > ? AND response_type = ? GROUP BY domain ORDER BY cnt DESC LIMIT ?`
		args = []interface{}{cutoff, responseType, limit}
	}

	rows, err := l.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DomainCount
	for rows.Next() {
		var dc DomainCount
		rows.Scan(&dc.Domain, &dc.Count)
		results = append(results, dc)
	}
	return results, nil
}

func (l *Logger) GetTopClients(limit int, since time.Duration) ([]ClientCount, error) {
	cutoff := time.Now().Add(-since)

	rows, err := l.db.Query(`
		SELECT source_ip, COUNT(*) as cnt 
		FROM queries 
		WHERE timestamp > ?
		GROUP BY source_ip 
		ORDER BY cnt DESC 
		LIMIT ?
	`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ClientCount
	for rows.Next() {
		var cc ClientCount
		rows.Scan(&cc.IP, &cc.Count)
		results = append(results, cc)
	}
	return results, nil
}

func (l *Logger) GetRecentQueries(limit int) ([]QueryRecord, error) {
	rows, err := l.db.Query(`
		SELECT timestamp, domain, query_type, source_ip, response_type, list_name, latency_ms
		FROM queries 
		ORDER BY timestamp DESC 
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []QueryRecord
	for rows.Next() {
		var r QueryRecord
		var listName sql.NullString
		rows.Scan(&r.Timestamp, &r.Domain, &r.QueryType, &r.SourceIP, &r.ResponseType, &listName, &r.LatencyMs)
		r.ListName = listName.String
		results = append(results, r)
	}
	return results, nil
}

func (l *Logger) GetQueriesOverTime(buckets int, since time.Duration) ([]map[string]interface{}, error) {
	cutoff := time.Now().Add(-since)
	bucketDuration := since / time.Duration(buckets)

	rows, err := l.db.Query(`
		SELECT 
			CAST((strftime('%s', timestamp) - strftime('%s', ?)) / ? AS INTEGER) as bucket,
			response_type,
			COUNT(*) as cnt
		FROM queries
		WHERE timestamp > ?
		GROUP BY bucket, response_type
		ORDER BY bucket
	`, cutoff, int(bucketDuration.Seconds()), cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bucketMap := make(map[int]map[string]int64)
	for rows.Next() {
		var bucket int
		var rt string
		var cnt int64
		rows.Scan(&bucket, &rt, &cnt)
		if _, ok := bucketMap[bucket]; !ok {
			bucketMap[bucket] = make(map[string]int64)
		}
		bucketMap[bucket][rt] = cnt
	}

	result := make([]map[string]interface{}, buckets)
	for i := 0; i < buckets; i++ {
		t := cutoff.Add(time.Duration(i) * bucketDuration)
		m := map[string]interface{}{
			"time":    t.Format(time.RFC3339),
			"blocked": int64(0),
			"allowed": int64(0),
		}
		if bm, ok := bucketMap[i]; ok {
			for rt, cnt := range bm {
				m[rt] = cnt
			}
		}
		result[i] = m
	}

	return result, nil
}
