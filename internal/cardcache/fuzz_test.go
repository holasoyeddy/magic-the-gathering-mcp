package cardcache

import (
	"encoding/json"
	"testing"
)

func FuzzCacheRecordDecodeNeverPanics(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"schema_version":1,"operation":"oracle-card","payload":{}}`))
	f.Add([]byte("not-json"))
	f.Fuzz(func(t *testing.T, data []byte) {
		var record entry
		_ = json.Unmarshal(data, &record)
		if json.Valid(data) && record.Payload != nil {
			_ = digest(record.Payload)
		}
	})
}
