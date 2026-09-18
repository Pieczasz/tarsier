package resolve

import "testing"

func FuzzAnalyzeGo(f *testing.F) {
	f.Add([]byte("package p\nconst a = \"user_id\"\n"))
	f.Add([]byte("package p\nconst (\n\ta = \"email\"\n\tb\n)\n"))
	f.Add([]byte("package p\nimport \"fmt\"\n"))
	f.Add([]byte("not go at all"))
	f.Fuzz(func(t *testing.T, src []byte) {
		file := Analyze("x.go", src)
		if file == nil {
			t.Fatal("Analyze must never return nil")
		}
		_ = file.PackageOf("fmt")
		_ = file.HasImport("fmt")
		_ = UnboundedLabel("user_id")
	})
}

func BenchmarkAnalyzeGo(b *testing.B) {
	src := []byte(`package p
import (
	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
)
const (
	label = "user_id"
	also
)
var LABELS = []string{"session_id", "status"}
`)
	b.ReportAllocs()
	for b.Loop() {
		_ = Analyze("x.go", src)
	}
}
