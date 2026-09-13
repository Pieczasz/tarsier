package kafka

type Writer struct{}

type Header struct {
	Key   string
	Value []byte
}

type Message struct {
	Topic   string
	Value   []byte
	Headers []Header
}

func (w *Writer) WriteMessages(ctx interface{}, msgs ...Message) error { return nil }
