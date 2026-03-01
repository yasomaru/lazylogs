package entry

import "testing"

var jsonLine = `{"time":"2025-03-01T10:00:00Z","level":"info","msg":"request completed","method":"GET","path":"/api/users","status":200,"latency":"45ms","request_id":"abc-123"}`
var logfmtLine = `ts=2025-03-01T10:00:00Z level=info msg="request completed" method=GET path=/api/users status=200 latency=45ms request_id=abc-123`
var plainLine = `2025-03-01 10:00:00 INFO request completed method=GET path=/api/users status=200 latency=45ms`

func BenchmarkParseJSON(b *testing.B) {
	for b.Loop() {
		ParseLine(jsonLine, 1)
	}
}

func BenchmarkParseLogfmt(b *testing.B) {
	for b.Loop() {
		ParseLine(logfmtLine, 1)
	}
}

func BenchmarkParsePlain(b *testing.B) {
	for b.Loop() {
		ParseLine(plainLine, 1)
	}
}
